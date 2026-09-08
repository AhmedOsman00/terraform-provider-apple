// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Capabilities hang off a Bundle ID, so every test here creates one. Both are
// deletable, so a completed run leaves nothing behind.
const testAccCapabilityBundleIdentifier = "com.test.terraform-capability"

func TestAccBundleIDCapabilityResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBundleIDCapabilityDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDCapabilityResourceConfig("PUSH_NOTIFICATIONS", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDCapabilityExists("apple_bundle_id_capability.test"),
					resource.TestCheckResourceAttr("apple_bundle_id_capability.test", "capability_type", "PUSH_NOTIFICATIONS"),
					resource.TestCheckResourceAttrSet("apple_bundle_id_capability.test", "id"),
					resource.TestCheckResourceAttrPair(
						"apple_bundle_id_capability.test", "bundle_id",
						"apple_bundle_id.test", "id",
					),
					// Apple returns no settings for push notifications, and
					// SettingsFromAPI maps that to null rather than an empty
					// list -- an empty list would not match the null in
					// configuration and would fail the apply.
					resource.TestCheckNoResourceAttr("apple_bundle_id_capability.test", "settings.#"),
				),
			},
			// Import the composite form. Apple's capability response does not
			// carry the parent Bundle ID, so this is the only form that yields
			// complete state.
			{
				ResourceName:      "apple_bundle_id_capability.test",
				ImportState:       true,
				ImportStateIdFunc: testAccBundleIDCapabilityImportIDFunc("apple_bundle_id_capability.test"),
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccBundleIDCapabilityResource_importBareID covers importing without the
// composite "<bundle_id>/<capability_id>" form. Apple embeds the parent in the
// capability ID, so bundle_id is recovered from there and confirmed against the
// parent's collection -- a bare import yields complete state, not partial.
func TestAccBundleIDCapabilityResource_importBareID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBundleIDCapabilityDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDCapabilityResourceConfig("PUSH_NOTIFICATIONS", ""),
				Check:  testAccCheckBundleIDCapabilityExists("apple_bundle_id_capability.test"),
			},
			{
				ResourceName:      "apple_bundle_id_capability.test",
				ImportState:       true,
				ImportStateVerify: false,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected one imported instance, got %d", len(states))
					}

					is := states[0]
					if got := is.Attributes["capability_type"]; got != "PUSH_NOTIFICATIONS" {
						return fmt.Errorf("imported capability_type is %q, expected PUSH_NOTIFICATIONS", got)
					}

					bundleID := is.Attributes["bundle_id"]
					if bundleID == "" {
						return fmt.Errorf("importing a bare capability ID left bundle_id empty, expected it recovered from %q", is.ID)
					}

					if prefix := apple.BundleIDFromCapabilityID(is.ID); bundleID != prefix {
						return fmt.Errorf("imported bundle_id is %q, expected %q from capability ID %q", bundleID, prefix, is.ID)
					}

					return nil
				},
			},
		},
	})
}

// TestAccBundleIDCapabilityResource_settings is the only path that reaches
// Update: capability_type and bundle_id both force replacement, so settings are
// the one thing that can change in place.
//
// The keys come from examples/resources/apple_bundle_id_capability. If Apple
// renames them the create fails here rather than silently doing nothing.
func TestAccBundleIDCapabilityResource_settings(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBundleIDCapabilityDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDCapabilityResourceConfig("DATA_PROTECTION", `
  settings = [
    {
      key   = "DATA_PROTECTION_PERMISSION_LEVEL"
      value = "COMPLETE_PROTECTION"
    },
  ]
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDCapabilityExists("apple_bundle_id_capability.test"),
					resource.TestCheckResourceAttr("apple_bundle_id_capability.test", "settings.#", "1"),
					resource.TestCheckResourceAttr("apple_bundle_id_capability.test", "settings.0.key", "DATA_PROTECTION_PERMISSION_LEVEL"),
					resource.TestCheckResourceAttr("apple_bundle_id_capability.test", "settings.0.value", "COMPLETE_PROTECTION"),
				),
			},
			{
				Config: testAccBundleIDCapabilityResourceConfig("DATA_PROTECTION", `
  settings = [
    {
      key   = "DATA_PROTECTION_PERMISSION_LEVEL"
      value = "PROTECTED_UNTIL_FIRST_USER_AUTH"
    },
  ]
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("apple_bundle_id_capability.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDCapabilityExists("apple_bundle_id_capability.test"),
					resource.TestCheckResourceAttr("apple_bundle_id_capability.test", "settings.0.value", "PROTECTED_UNTIL_FIRST_USER_AUTH"),
				),
			},
		},
	})
}

// TestAccBundleIDCapabilityResource_requiresReplace covers capability_type,
// which Apple cannot change on an existing capability.
func TestAccBundleIDCapabilityResource_requiresReplace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBundleIDCapabilityDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDCapabilityResourceConfig("PUSH_NOTIFICATIONS", ""),
				Check:  testAccCheckBundleIDCapabilityExists("apple_bundle_id_capability.test"),
			},
			{
				Config: testAccBundleIDCapabilityResourceConfig("HEALTHKIT", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("apple_bundle_id_capability.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDCapabilityExists("apple_bundle_id_capability.test"),
					resource.TestCheckResourceAttr("apple_bundle_id_capability.test", "capability_type", "HEALTHKIT"),
				),
			},
		},
	})
}

func TestAccBundleIDCapabilityResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "apple_bundle_id_capability" "test" {
  bundle_id       = "ABCD123456"
  capability_type = "NOT_A_CAPABILITY"
}
`,
				ExpectError: regexp.MustCompile(`Attribute capability_type value must be one of`),
			},
		},
	})
}

// testAccBundleIDCapabilityResourceConfig renders a Bundle ID and one
// capability. settings is injected verbatim so a step can supply a settings
// block without a second config function.
func testAccBundleIDCapabilityResourceConfig(capabilityType, settings string) string {
	return fmt.Sprintf(`
resource "apple_bundle_id" "test" {
  identifier = %[1]q
  name       = "Terraform Capability Test"
  platform   = "IOS"
}

resource "apple_bundle_id_capability" "test" {
  bundle_id       = apple_bundle_id.test.id
  capability_type = %[2]q
%[3]s
}
`, testAccCapabilityBundleIdentifier, capabilityType, settings)
}

// testAccBundleIDCapabilityImportIDFunc builds the "<bundle_id>/<capability_id>"
// form that import verifies before writing state.
func testAccBundleIDCapabilityImportIDFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		is, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}

		bundleID := is.Attributes["bundle_id"]
		if bundleID == "" {
			return "", fmt.Errorf("%s has no bundle_id in state to import with", resourceName)
		}

		return fmt.Sprintf("%s/%s", bundleID, is.ID), nil
	}
}

func testAccCheckBundleIDCapabilityExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		is, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("building API client: %w", err)
		}

		// Apple allows no GET on a single capability, so the check reads the
		// parent's collection. bundle_id is in state for anything the tests
		// create; the ID's own prefix covers a bare-ID import.
		bundleID := is.Attributes["bundle_id"]
		if bundleID == "" {
			bundleID = apple.BundleIDFromCapabilityID(is.ID)
		}

		capability, err := client.GetBundleIDCapability(bundleID, is.ID)
		if err != nil {
			return fmt.Errorf("reading Bundle ID Capability %s from Apple: %w", is.ID, err)
		}

		if got := string(capability.Attributes.CapabilityType); got != is.Attributes["capability_type"] {
			return fmt.Errorf("Bundle ID Capability %s is of type %q at Apple, state says %q", is.ID, got, is.Attributes["capability_type"])
		}

		return nil
	}
}

func testAccCheckBundleIDCapabilityDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("building API client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "apple_bundle_id_capability" || rs.Primary == nil {
			continue
		}

		bundleID := rs.Primary.Attributes["bundle_id"]
		if bundleID == "" {
			bundleID = apple.BundleIDFromCapabilityID(rs.Primary.ID)
		}

		// The parent Bundle ID is normally destroyed in the same run, which
		// makes the list call fail rather than return an empty collection.
		// Either way the capability is gone; only a successful lookup is a
		// failure here.
		if _, err := client.GetBundleIDCapability(bundleID, rs.Primary.ID); err == nil {
			return fmt.Errorf("Bundle ID Capability %s still exists at Apple after destroy", rs.Primary.ID)
		}
	}

	return nil
}
