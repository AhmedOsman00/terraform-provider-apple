// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccAppCategoriesDataSource_basic reads Apple's category catalogue.
//
// Unlike almost everything else in this suite it needs no app: categories
// belong to the App Store rather than to the account, the same way territories
// do, so credentials alone are enough.
func TestAccAppCategoriesDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "apple_app_categories" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// The catalogue is fixed, so asserting a known member is
					// safe in a way asserting a count would not be.
					testAccCheckCategoryPresent("data.apple_app_categories.test", "FINANCE"),
					testAccCheckCategoryPresent("data.apple_app_categories.test", "PRODUCTIVITY"),
					resource.TestCheckResourceAttrSet("data.apple_app_categories.test", "total_count"),
				),
			},
			// GAMES has subcategories where most categories have none, which is
			// what makes it worth asserting.
			{
				Config: `
data "apple_app_categories" "test" {
  id_pattern = "^GAMES$"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.apple_app_categories.test", "categories.#", "1"),
					resource.TestCheckResourceAttr("data.apple_app_categories.test", "categories.0.id", "GAMES"),
					resource.TestCheckResourceAttrWith("data.apple_app_categories.test",
						"categories.0.subcategories.#", func(value string) error {
							if value == "0" {
								return fmt.Errorf("GAMES reported no subcategories")
							}

							return nil
						}),
				),
			},
			// top_level_only drops the subcategories, which is the difference
			// between a list you can assign from and one you cannot.
			{
				Config: `
data "apple_app_categories" "test" {
  platforms      = ["IOS"]
  top_level_only = true
  id_pattern     = "^GAMES"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.apple_app_categories.test", "categories.#", "1"),
					resource.TestCheckResourceAttr("data.apple_app_categories.test", "categories.0.id", "GAMES"),
					resource.TestCheckNoResourceAttr("data.apple_app_categories.test", "categories.0.parent_id"),
				),
			},
		},
	})
}

// TestAccAppPricePointsDataSource_basic reads the price catalogue of the test
// app.
//
// Reading is safe on any app: it changes nothing. Only
// apple_app_price_schedule writes, and that is guarded separately.
func TestAccAppPricePointsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "apple_app_price_points" "test" {
  app_id      = %[1]q
  territories = ["USA"]
}`, testAccAppID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_app_price_points.test", "price_points.0.id"),
					resource.TestCheckResourceAttr("data.apple_app_price_points.test",
						"price_points.0.territory_id", "USA"),
					resource.TestCheckResourceAttrSet("data.apple_app_price_points.test", "total_count"),
				),
			},
			// The zero price point is the one that makes an app free, so it is
			// worth knowing it is reachable.
			{
				Config: fmt.Sprintf(`
data "apple_app_price_points" "test" {
  app_id         = %[1]q
  territories    = ["USA"]
  customer_price = "0"
}`, testAccAppID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.apple_app_price_points.test", "filtered_count", "1"),
					resource.TestCheckResourceAttr("data.apple_app_price_points.test",
						"price_points.0.customer_price", "0"),
				),
			},
		},
	})
}

// TestAccAppStoreVersionsDataSource_basic lists the test app's versions.
func TestAccAppStoreVersionsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// total_count is not asserted: how many versions the app has
				// depends on its release history.
				Config: fmt.Sprintf(`
data "apple_app_store_versions" "test" {
  app_id   = %[1]q
  platform = "IOS"
}`, testAccAppID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_app_store_versions.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_app_store_versions.test", "filtered_count"),
				),
			},
			{
				Config: fmt.Sprintf(`
data "apple_app_store_versions" "test" {
  app_id     = %[1]q
  platform   = "IOS"
  sort_by    = "created_date"
  sort_order = "desc"
  limit      = 1
}`, testAccAppID()),
				Check: resource.TestCheckResourceAttrWith("data.apple_app_store_versions.test",
					"versions.#", func(value string) error {
						if value != "0" && value != "1" {
							return fmt.Errorf("limit = 1 returned %s versions", value)
						}

						return nil
					}),
			},
		},
	})
}

// testAccCheckCategoryPresent asserts a category ID appears somewhere in the
// list, without depending on its position.
func testAccCheckCategoryPresent(name, categoryID string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("data source %s not found in state", name)
		}

		count, ok := rs.Primary.Attributes["categories.#"]
		if !ok {
			return fmt.Errorf("data source %s reported no categories", name)
		}

		total, err := strconv.Atoi(count)
		if err != nil {
			return fmt.Errorf("unreadable category count %q: %w", count, err)
		}

		for i := 0; i < total; i++ {
			if rs.Primary.Attributes[fmt.Sprintf("categories.%d.id", i)] == categoryID {
				return nil
			}
		}

		return fmt.Errorf("category %s not found among the %d Apple returned", categoryID, total)
	}
}
