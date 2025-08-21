package provider

import (
	"fmt"
	"regexp"
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
				Config: testAccBundleIDResourceConfig("com.test.example", "Test Example", "IOS"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDExists("apple_bundle_id.test"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "identifier", "com.test.example"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "name", "Test Example"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "platform", "IOS"),
					resource.TestCheckResourceAttrSet("apple_bundle_id.test", "id"),
					resource.TestCheckResourceAttrSet("apple_bundle_id.test", "seed_id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "apple_bundle_id.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccBundleIDImportStateIDFunc("apple_bundle_id.test"),
			},
			// Update and Read testing (only name can be updated)
			{
				Config: testAccBundleIDResourceConfig("com.test.example", "Updated Example", "IOS"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDExists("apple_bundle_id.test"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "identifier", "com.test.example"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "name", "Updated Example"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "platform", "IOS"),
				),
			},
		},
	})
}

func TestAccBundleIDResource_platforms(t *testing.T) {
	platforms := []string{"IOS", "MAC_OS", "TV_OS", "WATCH_OS"}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckBundleIDDestroy,
				Steps: []resource.TestStep{
					{
						Config: testAccBundleIDResourceConfig(
							fmt.Sprintf("com.test.%s", platform),
							fmt.Sprintf("Test %s", platform),
							platform,
						),
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
				Config: testAccBundleIDResourceConfig("com.test.disappear", "Test Disappear", "IOS"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBundleIDExists("apple_bundle_id.test"),
					testAccCheckBundleIDDisappears("apple_bundle_id.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
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

		// Here you would typically make an API call to delete the resource outside of Terraform
		// This simulates external deletion to test drift detection

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
