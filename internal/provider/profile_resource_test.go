// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// A profile needs a Bundle ID and at least one certificate, so every test here
// creates all three. IOS_APP_STORE is deliberate: development and ad hoc
// profiles require devices, and Apple cannot delete a device once registered,
// so those variants would leave permanent residue in the team.
//
// The certificate is IOS_DISTRIBUTION because that is what an App Store profile
// accepts. It is revoked when the test destroys, but a run interrupted between
// create and destroy holds one distribution slot until it is revoked by hand.
const testAccProfileBundleIdentifier = "com.test.terraform-profile"

func TestAccProfileResource_basic(t *testing.T) {
	csr := testAccCertificateCSR(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckProfileDestroy,
			testAccCheckCertificateDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccProfileResourceConfig(csr, "Terraform Test Profile", "IOS_APP_STORE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProfileExists("apple_profile.test"),
					resource.TestCheckResourceAttr("apple_profile.test", "name", "Terraform Test Profile"),
					resource.TestCheckResourceAttr("apple_profile.test", "profile_type", "IOS_APP_STORE"),
					// platform is derived by Apple from profile_type rather
					// than configured.
					resource.TestCheckResourceAttr("apple_profile.test", "platform", "IOS"),
					resource.TestCheckResourceAttr("apple_profile.test", "profile_state", "ACTIVE"),
					resource.TestCheckResourceAttrSet("apple_profile.test", "uuid"),
					resource.TestCheckResourceAttrSet("apple_profile.test", "profile_content"),
					resource.TestCheckResourceAttrSet("apple_profile.test", "created_date"),
					resource.TestCheckResourceAttrSet("apple_profile.test", "expiration_date"),
					resource.TestCheckResourceAttr("apple_profile.test", "certificates.#", "1"),
					resource.TestCheckResourceAttrPair(
						"apple_profile.test", "certificates.0",
						"apple_certificate.test", "id",
					),
					resource.TestCheckResourceAttrPair(
						"apple_profile.test", "bundle_id",
						"apple_bundle_id.test", "id",
					),
				),
			},
			// Import by Apple's profile ID. bundle_id, certificates and devices
			// are ignored because Apple's profile response does not carry the
			// relationships, so ImportState writes them empty -- a wart worth
			// knowing about: the first plan after an import wants to replace
			// the profile.
			{
				ResourceName:            "apple_profile.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"bundle_id", "certificates", "devices"},
			},
			// Import by name, the other form ImportState accepts.
			{
				ResourceName:            "apple_profile.test",
				ImportState:             true,
				ImportStateId:           "Terraform Test Profile",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"bundle_id", "certificates", "devices"},
			},
		},
	})
}

// TestAccProfileResource_rename covers renaming a profile, which Apple can only
// do by reissuing it.
//
// This test is what settled the question the resource used to hedge on: Apple
// answers PATCH /v1/profiles/{id} with 403 "The resource 'profiles' does not
// allow 'UPDATE'. Allowed operations are: CREATE, DELETE, GET_COLLECTION,
// GET_INSTANCE". name therefore carries RequiresReplace like every other
// attribute here, and a rename is a revoke and reissue -- which is exactly why
// fastlane recreates profiles to rename them.
func TestAccProfileResource_rename(t *testing.T) {
	csr := testAccCertificateCSR(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckProfileDestroy,
			testAccCheckCertificateDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccProfileResourceConfig(csr, "Terraform Rename Before", "IOS_APP_STORE"),
				Check:  testAccCheckProfileExists("apple_profile.test"),
			},
			{
				Config: testAccProfileResourceConfig(csr, "Terraform Rename After", "IOS_APP_STORE"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("apple_profile.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProfileExists("apple_profile.test"),
					resource.TestCheckResourceAttr("apple_profile.test", "name", "Terraform Rename After"),
				),
			},
		},
	})
}

// TestAccProfileResource_requiresReplace covers bundle_id, one of the four
// attributes Apple will not let a profile change in place.
//
// It moves the profile between two Bundle IDs rather than changing
// profile_type, because every other iOS profile type either requires devices
// (which Apple cannot delete once registered) or an Enterprise account, and a
// macOS type would need a Bundle ID that supports macOS.
func TestAccProfileResource_requiresReplace(t *testing.T) {
	csr := testAccCertificateCSR(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckProfileDestroy,
			testAccCheckCertificateDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccProfileResourceConfigTwoBundles(csr, "Terraform Replace Profile", "first"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProfileExists("apple_profile.test"),
					resource.TestCheckResourceAttrPair(
						"apple_profile.test", "bundle_id",
						"apple_bundle_id.first", "id",
					),
				),
			},
			{
				Config: testAccProfileResourceConfigTwoBundles(csr, "Terraform Replace Profile", "second"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("apple_profile.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProfileExists("apple_profile.test"),
					resource.TestCheckResourceAttrPair(
						"apple_profile.test", "bundle_id",
						"apple_bundle_id.second", "id",
					),
				),
			},
		},
	})
}

func TestAccProfileResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccProfileResourceConfigValidation("Terraform Invalid Profile", "NOT_A_PROFILE_TYPE"),
				ExpectError: regexp.MustCompile(`Attribute profile_type value must be one of`),
			},
			{
				Config:      testAccProfileResourceConfigValidation(strings.Repeat("x", 65), "IOS_APP_STORE"),
				ExpectError: regexp.MustCompile(`string length must be between 1 and 64`),
			},
		},
	})
}

// testAccProfileResourceConfig renders the Bundle ID, certificate and profile a
// profile test needs. The certificate is created here rather than shared across
// tests so each test cleans up everything it made.
func testAccProfileResourceConfig(csr, name, profileType string) string {
	return fmt.Sprintf(`
resource "apple_bundle_id" "test" {
  identifier = %[1]q
  name       = "Terraform Profile Test"
  platform   = "IOS"
}

resource "apple_certificate" "test" {
  certificate_type = "IOS_DISTRIBUTION"
  csr_content      = %[2]q
}

resource "apple_profile" "test" {
  name         = %[3]q
  profile_type = %[4]q
  bundle_id    = apple_bundle_id.test.id
  certificates = [apple_certificate.test.id]
}
`, testAccProfileBundleIdentifier, csr, name, profileType)
}

// testAccProfileResourceConfigTwoBundles points the profile at one of two
// Bundle IDs, so a step can move it and observe the replacement.
func testAccProfileResourceConfigTwoBundles(csr, name, bundle string) string {
	return fmt.Sprintf(`
resource "apple_bundle_id" "first" {
  identifier = "%[1]s-first"
  name       = "Terraform Profile Test One"
  platform   = "IOS"
}

resource "apple_bundle_id" "second" {
  identifier = "%[1]s-second"
  name       = "Terraform Profile Test Two"
  platform   = "IOS"
}

resource "apple_certificate" "test" {
  certificate_type = "IOS_DISTRIBUTION"
  csr_content      = %[2]q
}

resource "apple_profile" "test" {
  name         = %[3]q
  profile_type = "IOS_APP_STORE"
  bundle_id    = apple_bundle_id.%[4]s.id
  certificates = [apple_certificate.test.id]
}
`, testAccProfileBundleIdentifier, csr, name, bundle)
}

// testAccProfileResourceConfigValidation is rejected before the plan reaches
// Apple, so it references nothing and creates nothing.
func testAccProfileResourceConfigValidation(name, profileType string) string {
	return fmt.Sprintf(`
resource "apple_profile" "test" {
  name         = %[1]q
  profile_type = %[2]q
  bundle_id    = "ABCD123456"
  certificates = ["ABCD123456"]
}
`, name, profileType)
}

func testAccCheckProfileExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		is, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("building API client: %w", err)
		}

		profile, err := client.GetProfile(is.ID)
		if err != nil {
			return fmt.Errorf("reading Profile %s from Apple: %w", is.ID, err)
		}

		if got := profile.Attributes.Name; got != is.Attributes["name"] {
			return fmt.Errorf("Profile %s is named %q at Apple, state says %q", is.ID, got, is.Attributes["name"])
		}

		return nil
	}
}

func testAccCheckProfileDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("building API client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "apple_profile" || rs.Primary == nil {
			continue
		}

		if _, err := client.GetProfile(rs.Primary.ID); err == nil {
			return fmt.Errorf("Profile %s still exists at Apple after destroy", rs.Primary.ID)
		}
	}

	return nil
}
