// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccInAppPurchasesDataSource_basic creates two purchases of different
// kinds and exercises filtering and sorting over them.
//
// total_count is not asserted: the app named by APPLE_TEST_APP_ID may already
// carry purchases this suite did not create, so only the filters that name
// these two are checked.
func TestAccInAppPurchasesDataSource_basic(t *testing.T) {
	primary := testAccInAppPurchaseProductID()
	secondary := testAccInAppPurchaseProductID()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckInAppPurchase(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInAppPurchaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccInAppPurchasesDataSourceConfig(primary, secondary),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInAppPurchaseExists("apple_in_app_purchase.test"),
					testAccCheckInAppPurchaseExists("apple_in_app_purchase.second"),

					// The product ID pattern is a regex, anchored here so it
					// matches exactly one purchase.
					resource.TestCheckResourceAttr("data.apple_in_app_purchases.primary", "filtered_count", "1"),
					resource.TestCheckResourceAttr("data.apple_in_app_purchases.primary", "in_app_purchases.0.product_id", primary),
					resource.TestCheckResourceAttr("data.apple_in_app_purchases.primary", "in_app_purchases.0.in_app_purchase_type", "NON_CONSUMABLE"),
					// app_id is echoed from the argument -- Apple's in-app
					// purchase resource has no app relationship to read it from.
					resource.TestCheckResourceAttr("data.apple_in_app_purchases.primary", "in_app_purchases.0.app_id", testAccAppID()),

					// The type filter is an exact match on Apple's enum.
					resource.TestCheckResourceAttr("data.apple_in_app_purchases.consumables", "in_app_purchases.0.product_id", secondary),

					resource.TestCheckResourceAttr("data.apple_in_app_purchases.none", "filtered_count", "0"),
				),
			},
		},
	})
}

// TestAccInAppPurchasePricePointsDataSource_basic reads Apple's price catalogue
// for a purchase. Nothing is priced here -- the catalogue is read-only.
func TestAccInAppPurchasePricePointsDataSource_basic(t *testing.T) {
	productID := testAccInAppPurchaseProductID()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckInAppPurchase(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInAppPurchaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccInAppPurchaseConfig(productID, "Price Points", "NON_CONSUMABLE") + `
data "apple_in_app_purchase_price_points" "usa" {
  in_app_purchase_id = apple_in_app_purchase.test.id
  territories        = ["USA"]
  limit              = 10
}

data "apple_in_app_purchase_price_points" "two_territories" {
  in_app_purchase_id = apple_in_app_purchase.test.id
  territories        = ["USA", "GBR"]
  limit              = 10
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInAppPurchaseExists("apple_in_app_purchase.test"),
					resource.TestCheckResourceAttrSet("data.apple_in_app_purchase_price_points.usa", "price_points.0.id"),
					resource.TestCheckResourceAttrSet("data.apple_in_app_purchase_price_points.usa", "price_points.0.customer_price"),
					resource.TestCheckResourceAttrSet("data.apple_in_app_purchase_price_points.usa", "price_points.0.proceeds"),
					// include=territory is what makes a price point
					// identifiable; without it this attribute would be null.
					resource.TestCheckResourceAttr("data.apple_in_app_purchase_price_points.usa", "price_points.0.territory_id", "USA"),
					resource.TestCheckResourceAttrSet("data.apple_in_app_purchase_price_points.two_territories", "total_count"),
				),
			},
		},
	})
}

func testAccInAppPurchasesDataSourceConfig(primary, secondary string) string {
	return testAccInAppPurchaseConfig(primary, "Pro Unlock", "NON_CONSUMABLE") + fmt.Sprintf(`
resource "apple_in_app_purchase" "second" {
  app_id               = %[1]q
  product_id           = %[2]q
  name                 = "Coin Pack"
  in_app_purchase_type = "CONSUMABLE"
}

data "apple_in_app_purchases" "primary" {
  app_id             = %[1]q
  product_id_pattern = "^%[3]s$"

  depends_on = [apple_in_app_purchase.test, apple_in_app_purchase.second]
}

data "apple_in_app_purchases" "consumables" {
  app_id               = %[1]q
  product_id_pattern   = "^%[2]s$"
  in_app_purchase_type = "CONSUMABLE"

  depends_on = [apple_in_app_purchase.test, apple_in_app_purchase.second]
}

data "apple_in_app_purchases" "none" {
  app_id             = %[1]q
  product_id_pattern = "^com\\.test\\.terraform\\.nothing\\.matches\\.this$"

  depends_on = [apple_in_app_purchase.test, apple_in_app_purchase.second]
}
`, testAccAppID(), secondary, primary)
}
