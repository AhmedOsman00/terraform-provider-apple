// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPassTypeIDResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPassTypeIDDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccPassTypeIDResourceConfig("pass.com.test.terraform-example", "Test Example Pass"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPassTypeIDExists("apple_pass_type_id.test"),
					resource.TestCheckResourceAttr("apple_pass_type_id.test", "identifier", "pass.com.test.terraform-example"),
					resource.TestCheckResourceAttr("apple_pass_type_id.test", "name", "Test Example Pass"),
					resource.TestCheckResourceAttrSet("apple_pass_type_id.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "apple_pass_type_id.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccPassTypeIDImportStateIDFunc("apple_pass_type_id.test"),
			},
			// Update and Read testing (only name can be updated)
			{
				Config: testAccPassTypeIDResourceConfig("pass.com.test.terraform-example", "Updated Example Pass"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPassTypeIDExists("apple_pass_type_id.test"),
					resource.TestCheckResourceAttr("apple_pass_type_id.test", "identifier", "pass.com.test.terraform-example"),
					resource.TestCheckResourceAttr("apple_pass_type_id.test", "name", "Updated Example Pass"),
				),
			},
		},
	})
}

func TestAccPassTypeIDResource_importByIdentifier(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPassTypeIDDestroy,
		Steps: []resource.TestStep{
			// Create initial resource
			{
				Config: testAccPassTypeIDResourceConfig("pass.com.test.terraform-import", "Import Test Pass"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPassTypeIDExists("apple_pass_type_id.test"),
				),
			},
			// Test import by identifier
			{
				ResourceName:      "apple_pass_type_id.test",
				ImportState:       true,
				ImportStateId:     "pass.com.test.terraform-import",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPassTypeIDResourceConfig(identifier, name string) string {
	return fmt.Sprintf(`
resource "apple_pass_type_id" "test" {
  identifier = %[1]q
  name       = %[2]q
}
`, identifier, name)
}

func testAccCheckPassTypeIDExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Pass Type ID is not set")
		}

		return nil
	}
}

// testAccCheckPassTypeIDDestroy asks Apple rather than reading Terraform state.
//
// Inspecting state cannot detect what this check exists to catch -- a Pass Type
// ID still live in the portal -- and it also reports a false failure: after a
// step that imports, the state handed to CheckDestroy still lists the resource
// even though the destroy removed it at Apple. Only Apple's answer decides.
func testAccCheckPassTypeIDDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("building API client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "apple_pass_type_id" || rs.Primary == nil || rs.Primary.ID == "" {
			continue
		}

		if _, err := client.GetPassTypeID(rs.Primary.ID); err == nil {
			return fmt.Errorf("Pass Type ID %s still exists at Apple after destroy", rs.Primary.ID)
		}
	}

	return nil
}

func testAccPassTypeIDImportStateIDFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", resourceName)
		}

		// Return the identifier for import testing
		return rs.Primary.Attributes["identifier"], nil
	}
}

func TestAccPassTypeIDResource_invalidIdentifier(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPassTypeIDResourceConfig("invalid.identifier", "Test Pass"),
				ExpectError: regexp.MustCompile("Pass Type ID identifier must start with 'pass.'"),
			},
		},
	})
}

func TestAccPassTypeIDResource_invalidName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPassTypeIDResourceConfig("pass.com.test.terraform-example", "Invalid@Name!"),
				ExpectError: regexp.MustCompile("Pass Type ID name can only contain"),
			},
		},
	})
}
