// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMerchantIDsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMerchantIDDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMerchantIDsDataSourceConfigWithResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_merchant_ids.test", "merchant_ids.#"),
					resource.TestCheckResourceAttrSet("data.apple_merchant_ids.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_merchant_ids.test", "filtered_count"),
				),
			},
		},
	})
}

// TestAccMerchantIDsDataSource_filtering pins the filter results exactly.
// `merchant_ids.#` being set proves only that the data source returned, which
// an unfiltered list would also satisfy; the prefix here is unique to this test
// so the counts are exact regardless of what else the account holds.
func TestAccMerchantIDsDataSource_filtering(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMerchantIDDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMerchantIDsDataSourceConfigFiltering(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Both fixtures share the prefix.
					resource.TestCheckResourceAttr("data.apple_merchant_ids.by_prefix", "filtered_count", "2"),
					resource.TestCheckResourceAttr("data.apple_merchant_ids.by_prefix", "merchant_ids.#", "2"),
					// display_name_pattern is a case-insensitive regex, so the
					// lower-case pattern still matches "Terraform DS Alpha".
					resource.TestCheckResourceAttr("data.apple_merchant_ids.by_display_name", "filtered_count", "1"),
					resource.TestCheckResourceAttr("data.apple_merchant_ids.by_display_name", "merchant_ids.0.display_name", "Terraform DS Alpha"),
					// Sorted descending, the beta identifier comes first.
					resource.TestCheckResourceAttr("data.apple_merchant_ids.sorted_desc", "merchant_ids.0.identifier", "merchant.com.test.terraform-ds-beta"),
					resource.TestCheckResourceAttr("data.apple_merchant_ids.sorted_desc", "merchant_ids.1.identifier", "merchant.com.test.terraform-ds-alpha"),
					// limit truncates the result but leaves filtered_count at
					// the pre-limit total.
					resource.TestCheckResourceAttr("data.apple_merchant_ids.limited", "merchant_ids.#", "1"),
					resource.TestCheckResourceAttr("data.apple_merchant_ids.limited", "filtered_count", "2"),
				),
			},
		},
	})
}

func TestAccMerchantIDsDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "apple_merchant_ids" "test" {
  sort_by = "not_a_field"
}
`,
				ExpectError: regexp.MustCompile(`Attribute sort_by value must be one of`),
			},
			{
				Config: `
data "apple_merchant_ids" "test" {
  sort_order = "sideways"
}
`,
				ExpectError: regexp.MustCompile(`Attribute sort_order value must be one of`),
			},
		},
	})
}

func testAccMerchantIDsDataSourceConfigWithResource() string {
	return `
resource "apple_merchant_id" "test" {
  identifier   = "merchant.com.test.terraform-ds"
  display_name = "Terraform DS Merchant"
}

data "apple_merchant_ids" "test" {
  depends_on = [apple_merchant_id.test]
}
`
}

func testAccMerchantIDsDataSourceConfigFiltering() string {
	return `
resource "apple_merchant_id" "alpha" {
  identifier   = "merchant.com.test.terraform-ds-alpha"
  display_name = "Terraform DS Alpha"
}

resource "apple_merchant_id" "beta" {
  identifier   = "merchant.com.test.terraform-ds-beta"
  display_name = "Terraform DS Beta"
}

data "apple_merchant_ids" "by_prefix" {
  identifier_prefix = "merchant.com.test.terraform-ds-"

  depends_on = [apple_merchant_id.alpha, apple_merchant_id.beta]
}

data "apple_merchant_ids" "by_display_name" {
  identifier_prefix    = "merchant.com.test.terraform-ds-"
  display_name_pattern = "terraform ds alpha"

  depends_on = [apple_merchant_id.alpha, apple_merchant_id.beta]
}

data "apple_merchant_ids" "sorted_desc" {
  identifier_prefix = "merchant.com.test.terraform-ds-"
  sort_by           = "identifier"
  sort_order        = "desc"

  depends_on = [apple_merchant_id.alpha, apple_merchant_id.beta]
}

data "apple_merchant_ids" "limited" {
  identifier_prefix = "merchant.com.test.terraform-ds-"
  limit             = 1

  depends_on = [apple_merchant_id.alpha, apple_merchant_id.beta]
}
`
}
