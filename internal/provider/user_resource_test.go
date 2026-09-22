// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Team membership is the one area of this provider where an acceptance test can
// take somebody's access away, so both resources here are guarded past
// credentials the way the app pricing and TestFlight tester tests are.
//
// What each does to a real team:
//
//   - apple_user_invitation sends a real invitation email and cancels it on
//     destroy. Nothing persists if the run completes; an interrupted one leaves
//     a pending invitation to cancel under Users and Access, and it lapses by
//     itself after 72 hours either way. It skips unless
//     APPLE_TEST_USER_INVITE_EMAIL names an address the runner is entitled to
//     invite.
//   - apple_user adopts somebody who is already on the team and **removes them
//     from it on destroy**, revoking their access to every app. That cannot be
//     put back by the API: they have to be invited again and accept again. It
//     needs APPLE_TEST_USER_EMAIL to name the member and
//     APPLE_TEST_ALLOW_USER_REMOVAL to be set as well, because naming the
//     member is not on its own a statement that losing them is acceptable.
//
// Both also need an App Store Connect API key with the Admin role: a Developer
// or App Manager key can read /v1/users and not write to it.

const (
	// testAccUserInviteEmailEnvVar names the address the invitation test writes
	// to. There is no safe default: applying it makes Apple email that address
	// an invitation to join somebody's App Store Connect team.
	testAccUserInviteEmailEnvVar = "APPLE_TEST_USER_INVITE_EMAIL"

	// testAccUserEmailEnvVar names an existing member for the apple_user test.
	testAccUserEmailEnvVar = "APPLE_TEST_USER_EMAIL"

	// testAccAllowUserRemovalEnvVar is the second half of the apple_user guard.
	// The test's destroy step removes the member from the team for good.
	testAccAllowUserRemovalEnvVar = "APPLE_TEST_ALLOW_USER_REMOVAL"
)

func testAccUserInviteEmail() string {
	return os.Getenv(testAccUserInviteEmailEnvVar)
}

func testAccUserEmail() string {
	return os.Getenv(testAccUserEmailEnvVar)
}

func testAccPreCheckUserInvitation(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	if testAccUserInviteEmail() == "" {
		t.Skipf("skipping user invitation acceptance test: set %s to an email address you are entitled "+
			"to invite. Applying this test sends that address a real invitation to join your App Store "+
			"Connect team, and the credentials need the Admin role.", testAccUserInviteEmailEnvVar)
	}
}

func testAccPreCheckUser(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	if testAccUserEmail() == "" || os.Getenv(testAccAllowUserRemovalEnvVar) == "" {
		t.Skipf("skipping user acceptance test: set %s to a team member this test may modify and set "+
			"%s to confirm it may remove them. Destroy takes that member off the team and revokes their "+
			"access to every app, which the API cannot undo — they have to be invited again and accept "+
			"again. Never point it at a colleague who needs their access.",
			testAccUserEmailEnvVar, testAccAllowUserRemovalEnvVar)
	}
}

// --- apple_user_invitation ----------------------------------------------------

// TestAccUserInvitationResource_basic covers the invitation lifecycle: send,
// read back, import, cancel.
//
// There is no update step. Apple publishes no PATCH for an invitation, so every
// attribute forces replacement, and a replacement here would mean a second
// email to the same address.
func TestAccUserInvitationResource_basic(t *testing.T) {
	email := testAccUserInviteEmail()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckUserInvitation(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserInvitationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserInvitationConfig(email),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserInvitationExists("apple_user_invitation.test"),
					resource.TestCheckResourceAttr("apple_user_invitation.test", "email", email),
					resource.TestCheckResourceAttr("apple_user_invitation.test", "first_name", "Terraform"),
					resource.TestCheckResourceAttr("apple_user_invitation.test", "roles.#", "1"),
					resource.TestCheckTypeSetElemAttr("apple_user_invitation.test", "roles.*", "DEVELOPER"),
					resource.TestCheckResourceAttr("apple_user_invitation.test", "accepted", "false"),
					resource.TestCheckNoResourceAttr("apple_user_invitation.test", "user_id"),
					// Apple's own answer: invitations are good for 72 hours.
					resource.TestCheckResourceAttrSet("apple_user_invitation.test", "expiration_date"),
				),
			},
			// The address is an accepted import ID alongside Apple's own, which
			// is what makes an invitation recoverable by somebody who only has
			// the email in front of them.
			{
				ResourceName:      "apple_user_invitation.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     email,
			},
		},
	})
}

func testAccUserInvitationConfig(email string) string {
	return fmt.Sprintf(`
resource "apple_user_invitation" "test" {
  email            = %[1]q
  first_name       = "Terraform"
  last_name        = "Acceptance"
  roles            = ["DEVELOPER"]
  all_apps_visible = true
}
`, email)
}

// TestAccUserInvitationResource_accountHolderRole checks the plan-time refusal.
//
// Apple rejects a request granting ACCOUNT_HOLDER, and its refusal arrives on
// apply after the email address has already been read out of a configuration
// somebody expected to work. This never applies anything, so it needs only
// credentials.
func TestAccUserInvitationResource_accountHolderRole(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "apple_user_invitation" "invalid" {
  email      = "terraform-acceptance@example.invalid"
  first_name = "Terraform"
  last_name  = "Acceptance"
  roles      = ["ACCOUNT_HOLDER"]
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("Account Holder Role Cannot Be Assigned"),
			},
		},
	})
}

// TestAccUserInvitationResource_conflictingVisibility checks the other
// plan-time refusal: Apple's own message for this names the relationship rather
// than the contradiction.
func TestAccUserInvitationResource_conflictingVisibility(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "apple_user_invitation" "invalid" {
  email            = "terraform-acceptance@example.invalid"
  first_name       = "Terraform"
  last_name        = "Acceptance"
  roles            = ["DEVELOPER"]
  all_apps_visible = true
  visible_apps     = ["1234567890"]
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("Conflicting App Visibility"),
			},
		},
	})
}

// --- apple_user ---------------------------------------------------------------

// TestAccUserResource_basic covers adopting a member, changing what they can do
// and removing them.
//
// The roles are deliberately modest and end at DEVELOPER: a test that granted
// ADMIN would leave a window in which the member could do anything to the team,
// and an interrupted run would leave it open.
func TestAccUserResource_basic(t *testing.T) {
	email := testAccUserEmail()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckUser(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig(email, `["DEVELOPER"]`, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists("apple_user.test"),
					resource.TestCheckResourceAttr("apple_user.test", "username", email),
					resource.TestCheckResourceAttr("apple_user.test", "roles.#", "1"),
					resource.TestCheckTypeSetElemAttr("apple_user.test", "roles.*", "DEVELOPER"),
					resource.TestCheckResourceAttr("apple_user.test", "provisioning_allowed", "false"),
					resource.TestCheckResourceAttrSet("apple_user.test", "id"),
				),
			},
			// Roles and provisioning access are the two things Apple's update
			// request actually carries, so this is the whole of the in-place
			// update path.
			{
				Config: testAccUserConfig(email, `["DEVELOPER", "APP_MANAGER"]`, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists("apple_user.test"),
					resource.TestCheckResourceAttr("apple_user.test", "roles.#", "2"),
					resource.TestCheckTypeSetElemAttr("apple_user.test", "roles.*", "APP_MANAGER"),
					resource.TestCheckResourceAttr("apple_user.test", "provisioning_allowed", "true"),
				),
			},
			// The address is an accepted import ID alongside Apple's opaque
			// one, which nobody has to hand.
			{
				ResourceName:      "apple_user.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     email,
			},
		},
	})
}

// TestAccUserResource_notOnTheTeam covers the adoption failure.
//
// Apple publishes no way to create a user, so an address that is not already a
// member has to be reported as such and has to name the resource that can do
// something about it. Nothing is written, so this needs only credentials.
func TestAccUserResource_notOnTheTeam(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "apple_user" "missing" {
  username = "terraform-acceptance-no-such-member@example.invalid"
  roles    = ["DEVELOPER"]
}
`,
				ExpectError: regexp.MustCompile("User Not On The Team"),
			},
		},
	})
}

func testAccUserConfig(email, roles string, provisioning bool) string {
	return fmt.Sprintf(`
resource "apple_user" "test" {
  username             = %[1]q
  roles                = %[2]s
  all_apps_visible     = true
  provisioning_allowed = %[3]t
}
`, email, roles, provisioning)
}

// --- checks -------------------------------------------------------------------

// testAccCheckUserInvitationExists asks Apple rather than Terraform state: an
// invitation that was never sent is still in state after a successful apply.
func testAccCheckUserInvitationExists(name string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, err := testAccStateResource(state, name)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build an API client: %w", err)
		}

		invitation, err := client.GetUserInvitation(rs.ID)
		if err != nil {
			return fmt.Errorf("user invitation %s not found at Apple: %w", rs.ID, err)
		}

		if invitation.Attributes.Email == nil ||
			!strings.EqualFold(*invitation.Attributes.Email, rs.Attributes["email"]) {
			return fmt.Errorf("user invitation %s is for a different address than state records", rs.ID)
		}

		return nil
	}
}

// testAccCheckUserInvitationDestroy verifies the invitation was cancelled.
func testAccCheckUserInvitationDestroy(state *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("could not build an API client: %w", err)
	}

	for _, rs := range state.RootModule().Resources {
		if rs.Type != "apple_user_invitation" {
			continue
		}

		if _, err := client.GetUserInvitation(rs.Primary.ID); err == nil {
			return fmt.Errorf("user invitation %s is still pending at Apple after destroy", rs.Primary.ID)
		}
	}

	return nil
}

// testAccCheckUserExists asks Apple for the member behind the resource.
func testAccCheckUserExists(name string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, err := testAccStateResource(state, name)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build an API client: %w", err)
		}

		if _, err := client.GetUser(rs.ID); err != nil {
			return fmt.Errorf("user %s not found at Apple: %w", rs.ID, err)
		}

		return nil
	}
}

// testAccCheckUserDestroy verifies the member really did come off the team.
//
// This is the check that matters most in this file: a destroy that silently did
// nothing would leave somebody with access that a configuration says they no
// longer have.
func testAccCheckUserDestroy(state *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("could not build an API client: %w", err)
	}

	for _, rs := range state.RootModule().Resources {
		if rs.Type != "apple_user" {
			continue
		}

		if _, err := client.GetUser(rs.Primary.ID); err == nil {
			return fmt.Errorf("user %s is still on the team at Apple after destroy", rs.Primary.ID)
		}
	}

	return nil
}
