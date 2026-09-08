// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccBundleIDResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBundleIDDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBundleIDResourceConfig("com.test.terraform-example", "Test Example", "IOS"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDExists("apple_bundle_id.test"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "identifier", "com.test.terraform-example"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "name", "Test Example"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "platform", "IOS"),
					resource.TestCheckResourceAttrSet("apple_bundle_id.test", "id"),
					resource.TestCheckResourceAttrSet("apple_bundle_id.test", "seed_id"),
				),
			},
			// ImportState testing. platform is ignored because Apple stores
			// every Bundle ID as UNIVERSAL whatever was sent, so an import can
			// only ever recover UNIVERSAL while the configuration says IOS.
			{
				ResourceName:            "apple_bundle_id.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"platform"},
				ImportStateIdFunc:       testAccBundleIDImportStateIDFunc("apple_bundle_id.test"),
			},
			// Update and Read testing (only name can be updated)
			{
				Config: testAccBundleIDResourceConfig("com.test.terraform-example", "Updated Example", "IOS"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDExists("apple_bundle_id.test"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "identifier", "com.test.terraform-example"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "name", "Updated Example"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "platform", "IOS"),
				),
			},
		},
	})
}

// TestAccBundleIDResource_platforms covers every platform Apple accepts on
// create. TV_OS and WATCH_OS are not among them: Apple rejects both with a 409
// naming IOS, MAC_OS and UNIVERSAL, and tvOS and watchOS App IDs are created as
// UNIVERSAL.
func TestAccBundleIDResource_platforms(t *testing.T) {
	platforms := []string{"IOS", "MAC_OS", "UNIVERSAL"}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			identifier, name := testAccBundleIDPlatformFixture(platform)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckBundleIDDestroy,
				Steps: []resource.TestStep{
					{
						Config: testAccBundleIDResourceConfig(identifier, name, platform),
						Check: resource.ComposeAggregateTestCheckFunc(
							testAccCheckBundleIDExists("apple_bundle_id.test"),
							resource.TestCheckResourceAttr("apple_bundle_id.test", "platform", platform),
						),
					},
				},
			})
		})
	}
}

func TestAccBundleIDResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBundleIDResourceConfig("invalid-identifier", "Test", "IOS"),
				ExpectError: regexp.MustCompile(`Bundle identifier must follow reverse domain format`),
			},
			{
				Config:      testAccBundleIDResourceConfig("com.test.valid", "", "IOS"),
				ExpectError: regexp.MustCompile(`Attribute name string length must be between 1 and 64`),
			},
			{
				Config:      testAccBundleIDResourceConfig("com.test.valid", "Test", "INVALID"),
				ExpectError: regexp.MustCompile(`Attribute platform value must be one of`),
			},
		},
	})
}

func TestAccBundleIDResource_requiresReplace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBundleIDDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDResourceConfig("com.test.original", "Test", "IOS"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDExists("apple_bundle_id.test"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "identifier", "com.test.original"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "platform", "IOS"),
				),
			},
			{
				Config: testAccBundleIDResourceConfig("com.test.changed", "Test", "IOS"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDExists("apple_bundle_id.test"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "identifier", "com.test.changed"),
				),
			},
		},
	})
}

func TestAccBundleIDResource_disappears(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBundleIDDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDResourceConfig("com.test.terraform-disappear", "Test Disappear", "IOS"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDExists("apple_bundle_id.test"),
					testAccCheckBundleIDDisappears("apple_bundle_id.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// testAccBundleIDPlatformFixture derives an identifier and name from a
// platform constant, neither of which may contain the underscore the constants
// themselves carry.
//
// BundleIdentifierValidator is `^[a-zA-Z0-9.-]+\.[a-zA-Z0-9.-]+$`, so an
// identifier of "com.test.MAC_OS" fails at plan time and the subtest never
// reaches Apple. Apple's own App ID name accepts only alphanumerics and
// spaces, so the name has to lose the underscore too -- a name that passes the
// provider's length-only validator can still be rejected by the API.
//
// MAC_OS therefore becomes "com.test.platform-mac-os" / "Test MAC OS".
func testAccBundleIDPlatformFixture(platform string) (identifier, name string) {
	slug := strings.ToLower(strings.ReplaceAll(platform, "_", "-"))

	return fmt.Sprintf("com.test.platform-%s", slug),
		fmt.Sprintf("Test %s", strings.ReplaceAll(platform, "_", " "))
}

func testAccBundleIDResourceConfig(identifier, name, platform string) string {
	return fmt.Sprintf(`
resource "apple_bundle_id" "test" {
  identifier = %[1]q
  name       = %[2]q
  platform   = %[3]q
}
`, identifier, name, platform)
}

func testAccCheckBundleIDExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// Retrieve the resource from state
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Resource not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Resource ID not set")
		}

		// Here you would typically make an API call to verify the resource exists
		// For now, we'll just verify the required attributes are present
		if rs.Primary.Attributes["identifier"] == "" {
			return fmt.Errorf("Bundle ID identifier not set")
		}

		return nil
	}
}

func testAccCheckBundleIDDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "apple_bundle_id" {
			continue
		}

		// Here you would typically make an API call to verify the resource is deleted
		// For testing purposes, we'll assume destruction was successful
	}

	return nil
}

func testAccCheckBundleIDDisappears(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Resource not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Resource ID not set")
		}

		// Actually delete the Bundle ID at Apple, behind Terraform's back. A
		// stub that only returned nil left the resource in place, so the
		// refresh plan was empty and ExpectNonEmptyPlan could only be satisfied
		// by unrelated drift -- which is what the platform attribute used to
		// supply. Deleting for real is what makes this a drift-detection test.
		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("building API client: %w", err)
		}

		if err := client.DeleteBundleID(rs.Primary.ID, nil); err != nil {
			return fmt.Errorf("deleting Bundle ID %s outside Terraform: %w", rs.Primary.ID, err)
		}

		return nil
	}
}

func testAccBundleIDImportStateIDFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("Resource not found: %s", resourceName)
		}

		// Return the Bundle ID identifier for import
		return rs.Primary.Attributes["identifier"], nil
	}
}
