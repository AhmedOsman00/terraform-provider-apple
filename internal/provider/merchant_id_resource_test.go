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

// Merchant IDs are deletable through the API, so a completed run leaves nothing
// behind. A run interrupted mid-way does: the identifier has to be removed in
// the portal before the test can pass again, because Apple rejects a duplicate.
const testAccMerchantIdentifier = "merchant.com.test.terraform"

func TestAccMerchantIDResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMerchantIDDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMerchantIDResourceConfig(testAccMerchantIdentifier, "Terraform Test Merchant"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMerchantIDExists("apple_merchant_id.test", "Terraform Test Merchant"),
					resource.TestCheckResourceAttr("apple_merchant_id.test", "identifier", testAccMerchantIdentifier),
					resource.TestCheckResourceAttr("apple_merchant_id.test", "display_name", "Terraform Test Merchant"),
					resource.TestCheckResourceAttrSet("apple_merchant_id.test", "id"),
				),
			},
			// Import by the human identifier, which is what ImportState
			// documents and the only form that does not depend on the shape of
			// Apple's opaque ID.
			{
				ResourceName:      "apple_merchant_id.test",
				ImportState:       true,
				ImportStateId:     testAccMerchantIdentifier,
				ImportStateVerify: true,
			},
			// display_name is the only attribute Apple lets you change, and it
			// has no RequiresReplace: the plan must be an in-place update.
			{
				Config: testAccMerchantIDResourceConfig(testAccMerchantIdentifier, "Renamed Test Merchant"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("apple_merchant_id.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMerchantIDExists("apple_merchant_id.test", "Renamed Test Merchant"),
					resource.TestCheckResourceAttr("apple_merchant_id.test", "display_name", "Renamed Test Merchant"),
				),
			},
		},
	})
}

// TestAccMerchantIDResource_importByAppleID covers the other branch of
// ImportState, which is selected by the ID looking like a 36-character dashed
// UUID. If Apple hands out shorter opaque IDs for Merchant IDs, this fails
// while the identifier import above passes -- that is the heuristic in
// merchant/resource.go being wrong, not the import itself.
func TestAccMerchantIDResource_importByAppleID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMerchantIDDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMerchantIDResourceConfig("merchant.com.test.terraform-import", "Terraform Import Merchant"),
			},
			{
				ResourceName:      "apple_merchant_id.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccMerchantIDResource_requiresReplace covers the RequiresReplace plan
// modifier on identifier: Apple cannot rename a Merchant ID, so a changed
// identifier has to destroy and recreate rather than update in place.
func TestAccMerchantIDResource_requiresReplace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMerchantIDDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMerchantIDResourceConfig("merchant.com.test.terraform-before", "Terraform Replace Merchant"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMerchantIDExists("apple_merchant_id.test", "Terraform Replace Merchant"),
				),
			},
			{
				Config: testAccMerchantIDResourceConfig("merchant.com.test.terraform-after", "Terraform Replace Merchant"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("apple_merchant_id.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMerchantIDExists("apple_merchant_id.test", "Terraform Replace Merchant"),
					resource.TestCheckResourceAttr("apple_merchant_id.test", "identifier", "merchant.com.test.terraform-after"),
				),
			},
		},
	})
}

// TestAccMerchantIDResource_validation rejects bad configuration at plan time,
// so it never reaches Apple and creates nothing.
func TestAccMerchantIDResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccMerchantIDResourceConfig("com.test.no-merchant-prefix", "Test"),
				ExpectError: regexp.MustCompile(`Merchant identifier must follow format`),
			},
			{
				Config:      testAccMerchantIDResourceConfig("merchant.com.test.terraform", ""),
				ExpectError: regexp.MustCompile(`string length must be between 1 and 64`),
			},
			{
				Config:      testAccMerchantIDResourceConfig("merchant.com.test.terraform", strings.Repeat("x", 65)),
				ExpectError: regexp.MustCompile(`string length must be between 1 and 64`),
			},
		},
	})
}

func testAccMerchantIDResourceConfig(identifier, displayName string) string {
	return fmt.Sprintf(`
resource "apple_merchant_id" "test" {
  identifier   = %[1]q
  display_name = %[2]q
}
`, identifier, displayName)
}

// testAccCheckMerchantIDExists asks Apple for the Merchant ID recorded in
// state, which is the only way to tell a real create from a resource that only
// exists because the provider wrote its own plan back into state.
func testAccCheckMerchantIDExists(resourceName, displayName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		is, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("building API client: %w", err)
		}

		merchantID, err := client.GetMerchantID(is.ID)
		if err != nil {
			return fmt.Errorf("reading Merchant ID %s from Apple: %w", is.ID, err)
		}

		if got := merchantID.Attributes.Identifier; got != is.Attributes["identifier"] {
			return fmt.Errorf("Merchant ID %s has identifier %q at Apple, state says %q", is.ID, got, is.Attributes["identifier"])
		}

		if got := merchantID.Attributes.DisplayName; got != displayName {
			return fmt.Errorf("Merchant ID %s has display name %q at Apple, expected %q", is.ID, got, displayName)
		}

		return nil
	}
}

func testAccCheckMerchantIDDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("building API client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "apple_merchant_id" || rs.Primary == nil {
			continue
		}

		if _, err := client.GetMerchantID(rs.Primary.ID); err == nil {
			return fmt.Errorf("Merchant ID %s still exists at Apple after destroy", rs.Primary.ID)
		}
	}

	return nil
}
