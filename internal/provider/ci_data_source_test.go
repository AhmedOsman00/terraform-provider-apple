// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCiDataSources(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{Config: `
data "apple_ci_products" "test" {}
data "apple_scm_repositories" "test" {}
data "apple_ci_xcode_versions" "test" {}
data "apple_ci_mac_os_versions" "test" {}
`, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("data.apple_ci_products.test", "products.#"),
			resource.TestCheckResourceAttrSet("data.apple_scm_repositories.test", "repositories.#"),
			resource.TestCheckResourceAttrSet("data.apple_ci_xcode_versions.test", "versions.#"),
			resource.TestCheckResourceAttrSet("data.apple_ci_mac_os_versions.test", "versions.#"),
		)}},
	})
}
