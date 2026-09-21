// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestFlight hangs off an app record Apple's API cannot create, so these tests
// need APPLE_TEST_APP_ID the same way the subscription and app listing tests do
// -- testAccPreCheckSubscription (subscription_resource_test.go) is that check.
//
// What each leaves behind differs by resource:
//
//   - apple_beta_group creates a real group and deletes it. A group with no
//     testers and no public link invites nobody, so a completed run is neutral
//     and an interrupted one leaves a group to delete in App Store Connect.
//     Names are randomised because Apple enforces them unique within an app.
//   - apple_beta_app_localization writes the app's TestFlight page text for one
//     language and deletes it afterwards. It uses a locale the app is unlikely
//     to be using already (see testAccBetaLocale) so the test never touches the
//     primary locale's record, which Apple refuses to delete.
//   - apple_beta_build_localization needs a build that has already been
//     uploaded, so it skips behind testAccPreCheckBuild like the build
//     attachment test does.
//   - apple_beta_app_review_detail adopts a record Apple created with the app.
//     There is nothing to delete, so testAccCheckAppStillExists is the only
//     honest CheckDestroy and the contact details stay on the app afterwards.
//
// None of these enables a public TestFlight link. Doing so issues a URL anyone
// can join the beta through, which is not something a test should put into the
// world; the configuration that would be rejected for it is covered by a
// plan-only error check instead.

// testAccBetaLocale is the language these tests write TestFlight text in.
//
// Deliberately not en-US: Apple creates a localization for the app's primary
// locale itself and refuses to delete the last one an app has, so a test that
// used the primary locale would adopt a record it then could not remove.
const testAccBetaLocale = "de-DE"

// testAccBetaGroupName returns a group name unique to this run.
//
// Apple enforces the name unique within an app, so a fixed one would collide
// with whatever an interrupted run left behind -- the same reason the
// subscription tests randomise product names.
func testAccBetaGroupName(base string) string {
	return fmt.Sprintf("%s %s", base, acctest.RandString(6))
}

// --- apple_beta_group ---------------------------------------------------------

func TestAccBetaGroupResource_basic(t *testing.T) {
	name := testAccBetaGroupName("Terraform External")
	renamed := testAccBetaGroupName("Terraform Renamed")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBetaGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBetaGroupConfig(name, false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBetaGroupExists("apple_beta_group.test"),
					resource.TestCheckResourceAttr("apple_beta_group.test", "name", name),
					resource.TestCheckResourceAttr("apple_beta_group.test", "app_id", testAccAppID()),
					resource.TestCheckResourceAttr("apple_beta_group.test", "is_internal_group", "false"),
					resource.TestCheckResourceAttr("apple_beta_group.test", "feedback_enabled", "true"),
					resource.TestCheckResourceAttrSet("apple_beta_group.test", "id"),
					resource.TestCheckResourceAttrSet("apple_beta_group.test", "created_date"),
				),
			},
			// name and feedback_enabled are both in Apple's update request, so
			// this exercises the whole in-place path without a replacement.
			{
				Config: testAccBetaGroupConfig(renamed, false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBetaGroupExists("apple_beta_group.test"),
					resource.TestCheckResourceAttr("apple_beta_group.test", "name", renamed),
					resource.TestCheckResourceAttr("apple_beta_group.test", "feedback_enabled", "false"),
				),
			},
			// A bare Apple group ID is enough: GET /v1/betaGroups/{id} accepts
			// include=app, so app_id comes back with the record.
			{
				ResourceName:      "apple_beta_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccBetaGroupResource_internal covers the other kind of group.
//
// An internal group draws its members from the App Store Connect team and skips
// beta review entirely, and neither is_internal_group nor
// has_access_to_all_builds is in Apple's update request -- so this also pins
// that both round-trip through a read.
func TestAccBetaGroupResource_internal(t *testing.T) {
	name := testAccBetaGroupName("Terraform Internal")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBetaGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBetaGroupInternalConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBetaGroupExists("apple_beta_group.internal"),
					resource.TestCheckResourceAttr("apple_beta_group.internal", "name", name),
					resource.TestCheckResourceAttr("apple_beta_group.internal", "is_internal_group", "true"),
					resource.TestCheckResourceAttr("apple_beta_group.internal", "has_access_to_all_builds", "true"),
				),
			},
			{
				ResourceName:      "apple_beta_group.internal",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccBetaGroupResource_publicLinkOnInternal checks the config-time refusal.
//
// Apple rejects the public-link attributes on an internal group, and its
// refusal names the attribute rather than the reason. This never applies
// anything, so it needs no test app -- only a provider.
func TestAccBetaGroupResource_publicLinkOnInternal(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBetaGroupPublicLinkOnInternalConfig(testAccBetaGroupName("Terraform Invalid")),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("Public Link On An Internal Group"),
			},
		},
	})
}

func testAccBetaGroupConfig(name string, internal, feedback bool) string {
	return fmt.Sprintf(`
resource "apple_beta_group" "test" {
  app_id            = %[1]q
  name              = %[2]q
  is_internal_group = %[3]t
  feedback_enabled  = %[4]t
}
`, testAccAppID(), name, internal, feedback)
}

func testAccBetaGroupInternalConfig(name string) string {
	return fmt.Sprintf(`
resource "apple_beta_group" "internal" {
  app_id                   = %[1]q
  name                     = %[2]q
  is_internal_group        = true
  has_access_to_all_builds = true
}
`, testAccAppID(), name)
}

func testAccBetaGroupPublicLinkOnInternalConfig(name string) string {
	return fmt.Sprintf(`
resource "apple_beta_group" "invalid" {
  app_id              = %[1]q
  name                = %[2]q
  is_internal_group   = true
  public_link_enabled = true
}
`, testAccAppID(), name)
}

// --- apple_beta_app_localization ----------------------------------------------

func TestAccBetaAppLocalizationResource_basic(t *testing.T) {
	description := "Terraform acceptance test build. " + acctest.RandString(8)
	updated := "Terraform acceptance test build, updated. " + acctest.RandString(8)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBetaAppLocalizationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBetaAppLocalizationConfig(description),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBetaAppLocalizationExists("apple_beta_app_localization.test"),
					resource.TestCheckResourceAttr("apple_beta_app_localization.test", "app_id", testAccAppID()),
					resource.TestCheckResourceAttr("apple_beta_app_localization.test", "locale", testAccBetaLocale),
					resource.TestCheckResourceAttr("apple_beta_app_localization.test", "description", description),
					resource.TestCheckResourceAttr("apple_beta_app_localization.test",
						"feedback_email", "testflight@example.com"),
					resource.TestCheckResourceAttrSet("apple_beta_app_localization.test", "id"),
				),
			},
			{
				Config: testAccBetaAppLocalizationConfig(updated),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBetaAppLocalizationExists("apple_beta_app_localization.test"),
					resource.TestCheckResourceAttr("apple_beta_app_localization.test", "description", updated),
				),
			},
			{
				ResourceName:      "apple_beta_app_localization.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccBetaAppLocalizationConfig(description string) string {
	return fmt.Sprintf(`
resource "apple_beta_app_localization" "test" {
  app_id         = %[1]q
  locale         = %[2]q
  description    = %[3]q
  feedback_email = "testflight@example.com"
  marketing_url  = "https://example.com/testflight"
}
`, testAccAppID(), testAccBetaLocale, description)
}

// --- apple_beta_build_localization ---------------------------------------------

// TestAccBetaBuildLocalizationResource_basic needs a build that is already in
// App Store Connect, for the reason the build attachment test does: this
// provider does not upload builds, and Apple leaves one PROCESSING for five to
// thirty minutes afterwards.
func TestAccBetaBuildLocalizationResource_basic(t *testing.T) {
	whatsNew := "Terraform acceptance test note. " + acctest.RandString(8)
	updated := "Terraform acceptance test note, updated. " + acctest.RandString(8)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckBuild(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBetaBuildLocalizationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBetaBuildLocalizationConfig(whatsNew),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_beta_build_localization.test", "app_id", testAccAppID()),
					resource.TestCheckResourceAttr("apple_beta_build_localization.test", "locale", testAccBetaLocale),
					resource.TestCheckResourceAttr("apple_beta_build_localization.test", "whats_new", whatsNew),
					resource.TestCheckResourceAttrSet("apple_beta_build_localization.test", "id"),
					resource.TestCheckResourceAttrSet("apple_beta_build_localization.test", "build_id"),
				),
			},
			{
				Config: testAccBetaBuildLocalizationConfig(updated),
				Check: resource.TestCheckResourceAttr(
					"apple_beta_build_localization.test", "whats_new", updated),
			},
			// There is no bare-ID import form: the record names its build by
			// Apple's opaque ID, from which the build number and its train
			// cannot be recovered without further requests.
			{
				ResourceName:      "apple_beta_build_localization.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) {
					return strings.Join([]string{
						testAccAppID(),
						os.Getenv(testAccBuildTrainEnvVar),
						os.Getenv(testAccBuildNumberEnvVar),
						testAccBetaLocale,
					}, "/"), nil
				},
			},
		},
	})
}

func testAccBetaBuildLocalizationConfig(whatsNew string) string {
	return fmt.Sprintf(`
resource "apple_beta_build_localization" "test" {
  app_id              = %[1]q
  build_number        = %[2]q
  pre_release_version = %[3]q
  locale              = %[4]q
  whats_new           = %[5]q
}
`, testAccAppID(), os.Getenv(testAccBuildNumberEnvVar), os.Getenv(testAccBuildTrainEnvVar), testAccBetaLocale, whatsNew)
}

// --- apple_beta_app_review_detail ----------------------------------------------

// TestAccBetaAppReviewDetailResource_basic adopts the record Apple created with
// the app.
//
// Apple publishes no POST and no DELETE for it, so the values written here stay
// on the app afterwards -- which is why this runs against a scratch app and why
// testAccCheckAppStillExists is the CheckDestroy.
func TestAccBetaAppReviewDetailResource_basic(t *testing.T) {
	notes := "Terraform acceptance test. " + acctest.RandString(8)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStillExists,
		Steps: []resource.TestStep{
			{
				Config: testAccBetaAppReviewDetailConfig(notes),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_beta_app_review_detail.test", "app_id", testAccAppID()),
					resource.TestCheckResourceAttr("apple_beta_app_review_detail.test", "notes", notes),
					resource.TestCheckResourceAttr("apple_beta_app_review_detail.test",
						"demo_account_required", "false"),
					resource.TestCheckResourceAttrSet("apple_beta_app_review_detail.test", "id"),
				),
			},
			// The import ID is the app ID: an app has exactly one beta review
			// detail and Apple publishes no collection of them.
			{
				ResourceName:            "apple_beta_app_review_detail.test",
				ImportState:             true,
				ImportStateId:           testAccAppID(),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"demo_account_password"},
			},
		},
	})
}

func testAccBetaAppReviewDetailConfig(notes string) string {
	return fmt.Sprintf(`
resource "apple_beta_app_review_detail" "test" {
  app_id                = %[1]q
  contact_first_name    = "Terraform"
  contact_last_name     = "Acceptance"
  contact_email         = "testflight@example.com"
  contact_phone         = "+20 109 255 8423"
  demo_account_required = false
  notes                 = %[2]q
}
`, testAccAppID(), notes)
}

// --- Checks --------------------------------------------------------------------

// testAccCheckBetaGroupExists asks Apple, not Terraform state.
func testAccCheckBetaGroupExists(name string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		client, err := testAccAPIClient()
		if err != nil {
			return err
		}

		group, err := client.GetBetaGroup(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("beta group %s not readable at Apple: %w", rs.Primary.ID, err)
		}

		if group.Attributes.Name == nil || *group.Attributes.Name != rs.Primary.Attributes["name"] {
			return fmt.Errorf("beta group %s is named differently at Apple than in state", rs.Primary.ID)
		}

		return nil
	}
}

// testAccCheckBetaGroupDestroy asserts the group is gone from Apple, not merely
// from state.
func testAccCheckBetaGroupDestroy(state *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return err
	}

	for _, rs := range state.RootModule().Resources {
		if rs.Type != "apple_beta_group" {
			continue
		}

		if _, err := client.GetBetaGroup(rs.Primary.ID); err == nil {
			return fmt.Errorf("beta group %s still exists at Apple after destroy", rs.Primary.ID)
		}
	}

	return nil
}

// testAccCheckBetaAppLocalizationExists asks Apple, not Terraform state.
func testAccCheckBetaAppLocalizationExists(name string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		client, err := testAccAPIClient()
		if err != nil {
			return err
		}

		if _, err := client.GetBetaAppLocalization(rs.Primary.ID); err != nil {
			return fmt.Errorf("beta app localization %s not readable at Apple: %w", rs.Primary.ID, err)
		}

		return nil
	}
}

// testAccCheckBetaAppLocalizationDestroy asserts the app no longer has
// TestFlight text in the test locale.
//
// The record is looked up through the app rather than by ID: a destroy that
// left it behind is the failure worth catching, and the app's collection is
// where it would show up.
func testAccCheckBetaAppLocalizationDestroy(_ *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return err
	}

	if _, err := client.GetBetaAppLocalizationByLocale(testAccAppID(), testAccBetaLocale); err == nil {
		return fmt.Errorf("beta app localization for %s still exists on app %s after destroy",
			testAccBetaLocale, testAccAppID())
	}

	return nil
}

// testAccCheckBetaBuildLocalizationDestroy asserts the note is gone from the
// build it was written on.
func testAccCheckBetaBuildLocalizationDestroy(state *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return err
	}

	for _, rs := range state.RootModule().Resources {
		if rs.Type != "apple_beta_build_localization" {
			continue
		}

		buildID := rs.Primary.Attributes["build_id"]
		if buildID == "" {
			continue
		}

		if _, err := client.GetBetaBuildLocalizationByLocale(buildID, testAccBetaLocale); err == nil {
			return fmt.Errorf("beta build localization for %s still exists on build %s after destroy",
				testAccBetaLocale, buildID)
		}
	}

	return nil
}
