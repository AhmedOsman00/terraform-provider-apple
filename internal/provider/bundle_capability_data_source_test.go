// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccBundleIDCapabilitiesDataSource_basic creates a Bundle ID with two
// capabilities and reads them back. Filtering by Bundle ID makes the counts
// exact whatever else the account holds.
func TestAccBundleIDCapabilitiesDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBundleIDCapabilityDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDCapabilitiesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_bundle_id_capabilities.all", "capabilities.#"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_id_capabilities.all", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_id_capabilities.all", "filtered_count"),

					// Both fixtures belong to the one Bundle ID. Apple enables
					// some capabilities on a new Bundle ID by itself, so this
					// asserts a floor rather than an exact count.
					resource.TestMatchResourceAttr("data.apple_bundle_id_capabilities.by_bundle", "filtered_count", regexp.MustCompile(`^([2-9]|[1-9][0-9]+)$`)),

					// Narrowed to one type, the count is exact.
					resource.TestCheckResourceAttr("data.apple_bundle_id_capabilities.by_type", "filtered_count", "1"),
					resource.TestCheckResourceAttr("data.apple_bundle_id_capabilities.by_type", "capabilities.0.capability_type", "PUSH_NOTIFICATIONS"),
					resource.TestCheckResourceAttrPair(
						"data.apple_bundle_id_capabilities.by_type", "capabilities.0.id",
						"apple_bundle_id_capability.push", "id",
					),

					resource.TestCheckResourceAttr("data.apple_bundle_id_capabilities.limited", "capabilities.#", "1"),
				),
			},
		},
	})
}

func TestAccBundleIDCapabilitiesDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "apple_bundle_id_capabilities" "test" {
  capability_type = "NOT_A_CAPABILITY"
}
`,
				ExpectError: regexp.MustCompile(`Attribute capability_type value must be one of`),
			},
			{
				Config: `
data "apple_bundle_id_capabilities" "test" {
  limit = 201
}
`,
				ExpectError: regexp.MustCompile(`Attribute limit value must be between 1 and 200`),
			},
			{
				Config: `
data "apple_bundle_id_capabilities" "test" {
  sort_by = "not_a_field"
}
`,
				ExpectError: regexp.MustCompile(`Attribute sort_by value must be one of`),
			},
		},
	})
}

func testAccBundleIDCapabilitiesDataSourceConfig() string {
	return `
resource "apple_bundle_id" "test" {
  identifier = "com.test.terraform-capability-ds"
  name       = "Terraform Capability DS Test"
  platform   = "IOS"
}

resource "apple_bundle_id_capability" "push" {
  bundle_id       = apple_bundle_id.test.id
  capability_type = "PUSH_NOTIFICATIONS"
}

resource "apple_bundle_id_capability" "health" {
  bundle_id       = apple_bundle_id.test.id
  capability_type = "HEALTH_KIT"
}

data "apple_bundle_id_capabilities" "all" {
  depends_on = [apple_bundle_id_capability.push, apple_bundle_id_capability.health]
}

data "apple_bundle_id_capabilities" "by_bundle" {
  bundle_id = apple_bundle_id.test.id

  depends_on = [apple_bundle_id_capability.push, apple_bundle_id_capability.health]
}

data "apple_bundle_id_capabilities" "by_type" {
  bundle_id       = apple_bundle_id.test.id
  capability_type = apple_bundle_id_capability.push.capability_type

  depends_on = [apple_bundle_id_capability.push, apple_bundle_id_capability.health]
}

data "apple_bundle_id_capabilities" "limited" {
  bundle_id = apple_bundle_id.test.id
  limit     = 1

  depends_on = [apple_bundle_id_capability.push, apple_bundle_id_capability.health]
}
`
}
