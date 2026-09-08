// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAppsDataSource_basic reads the account's apps. It creates nothing:
// apps are read-only, because Apple's API cannot make one.
func TestAccAppsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + `
data "apple_apps" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_apps.all", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_apps.all", "filtered_count"),
					// The account must hold at least the app the subscription
					// tests run against, or those tests could not work either.
					resource.TestCheckResourceAttrSet("data.apple_apps.all", "apps.0.id"),
					resource.TestCheckResourceAttrSet("data.apple_apps.all", "apps.0.bundle_id"),
				),
			},
		},
	})
}

// TestAccAppsDataSource_filtering checks that a filter actually narrows the
// result rather than being silently ignored.
func TestAccAppsDataSource_filtering(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + `
data "apple_apps" "all" {}

data "apple_apps" "none" {
  name_pattern = "^this name matches no app whatsoever$"
}

data "apple_apps" "limited" {
  limit = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.apple_apps.none", "filtered_count", "0"),
					resource.TestCheckResourceAttr("data.apple_apps.none", "apps.#", "0"),
					// total_count reports what Apple returned, before filtering,
					// so it is unaffected by a filter that matches nothing.
					resource.TestCheckResourceAttrPair(
						"data.apple_apps.none", "total_count",
						"data.apple_apps.all", "total_count",
					),
					resource.TestCheckResourceAttr("data.apple_apps.limited", "apps.#", "1"),
				),
			},
		},
	})
}

// TestAccSubscriptionGroupsDataSource_basic creates a group and reads it back.
func TestAccSubscriptionGroupsDataSource_basic(t *testing.T) {
	referenceName := "Terraform DS " + acctest.RandString(6)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSubscriptionGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionGroupConfig(referenceName) + fmt.Sprintf(`
data "apple_subscription_groups" "all" {
  app_id     = %[1]q
  depends_on = [apple_subscription_group.test]
}

data "apple_subscription_groups" "matching" {
  app_id                 = %[1]q
  reference_name_pattern = "^%[2]s$"
  depends_on             = [apple_subscription_group.test]
}
`, testAccAppID(), regexp.QuoteMeta(referenceName)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionGroupExists("apple_subscription_group.test"),
					resource.TestCheckResourceAttr("data.apple_subscription_groups.matching", "filtered_count", "1"),
					resource.TestCheckResourceAttr("data.apple_subscription_groups.matching", "subscription_groups.0.reference_name", referenceName),
					resource.TestCheckResourceAttrPair(
						"data.apple_subscription_groups.matching", "subscription_groups.0.id",
						"apple_subscription_group.test", "id",
					),
					// app_id is echoed from the argument, since Apple's group
					// response carries no app linkage.
					resource.TestCheckResourceAttr("data.apple_subscription_groups.matching", "subscription_groups.0.app_id", testAccAppID()),
				),
			},
		},
	})
}

// TestAccSubscriptionsDataSource_basic creates two subscriptions at different
// group levels and exercises filtering and sorting over them.
func TestAccSubscriptionsDataSource_basic(t *testing.T) {
	referenceName := "Terraform SubDS " + acctest.RandString(6)
	primary := testAccSubscriptionProductID()
	secondary := testAccSubscriptionProductID()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionsDataSourceConfig(referenceName, primary, secondary),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					testAccCheckSubscriptionExists("apple_subscription.second"),

					resource.TestCheckResourceAttr("data.apple_subscriptions.all", "total_count", "2"),

					// Every subscription here is new, so all of them are
					// incomplete until a localization and a price exist.
					resource.TestCheckResourceAttr("data.apple_subscriptions.incomplete", "filtered_count", "2"),

					// The period filter is an exact match on Apple's enum.
					resource.TestCheckResourceAttr("data.apple_subscriptions.yearly", "filtered_count", "1"),
					resource.TestCheckResourceAttr("data.apple_subscriptions.yearly", "subscriptions.0.product_id", secondary),
					resource.TestCheckResourceAttr("data.apple_subscriptions.yearly", "subscriptions.0.subscription_period", "ONE_YEAR"),

					// Sorting by group level puts level 1 before level 2.
					resource.TestCheckResourceAttr("data.apple_subscriptions.by_level", "subscriptions.0.group_level", "1"),
					resource.TestCheckResourceAttr("data.apple_subscriptions.by_level", "subscriptions.1.group_level", "2"),

					resource.TestCheckResourceAttr("data.apple_subscriptions.none", "filtered_count", "0"),
				),
			},
		},
	})
}

// TestAccSubscriptionPricePointsDataSource_basic reads Apple's price catalogue
// for a subscription. Nothing is priced here -- the catalogue is read-only.
func TestAccSubscriptionPricePointsDataSource_basic(t *testing.T) {
	referenceName := "Terraform PP " + acctest.RandString(6)
	productID := testAccSubscriptionProductID()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionConfig(referenceName, productID, testAccProductName("Pro Monthly"), "ONE_MONTH") + `
data "apple_subscription_price_points" "usa" {
  subscription_id = apple_subscription.test.id
  territories     = ["USA"]
  limit           = 10
}

data "apple_subscription_price_points" "two_territories" {
  subscription_id = apple_subscription.test.id
  territories     = ["USA", "GBR"]
  limit           = 10
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttrSet("data.apple_subscription_price_points.usa", "price_points.0.id"),
					resource.TestCheckResourceAttrSet("data.apple_subscription_price_points.usa", "price_points.0.customer_price"),
					resource.TestCheckResourceAttrSet("data.apple_subscription_price_points.usa", "price_points.0.proceeds"),
					// include=territory is what makes a price point
					// identifiable; without it this attribute would be null.
					resource.TestCheckResourceAttr("data.apple_subscription_price_points.usa", "price_points.0.territory_id", "USA"),
					resource.TestCheckResourceAttrSet("data.apple_subscription_price_points.two_territories", "total_count"),
				),
			},
		},
	})
}

func testAccSubscriptionsDataSourceConfig(referenceName, primary, secondary string) string {
	return testAccSubscriptionConfig(referenceName, primary, testAccProductName("Pro Monthly"), "ONE_MONTH") + fmt.Sprintf(`
resource "apple_subscription" "second" {
  group_id            = apple_subscription_group.test.id
  product_id          = %[1]q
  name                = %[2]q
  subscription_period = "ONE_YEAR"
  group_level         = 2
}

data "apple_subscriptions" "all" {
  group_id   = apple_subscription_group.test.id
  depends_on = [apple_subscription.test, apple_subscription.second]
}

data "apple_subscriptions" "incomplete" {
  group_id   = apple_subscription_group.test.id
  state      = "MISSING_METADATA"
  depends_on = [apple_subscription.test, apple_subscription.second]
}

data "apple_subscriptions" "yearly" {
  group_id            = apple_subscription_group.test.id
  subscription_period = "ONE_YEAR"
  depends_on          = [apple_subscription.test, apple_subscription.second]
}

data "apple_subscriptions" "by_level" {
  group_id   = apple_subscription_group.test.id
  sort_by    = "group_level"
  sort_order = "asc"
  depends_on = [apple_subscription.test, apple_subscription.second]
}

data "apple_subscriptions" "none" {
  group_id     = apple_subscription_group.test.id
  name_pattern = "^no subscription is named this$"
  depends_on   = [apple_subscription.test, apple_subscription.second]
}
`, secondary, testAccProductName("Basic Yearly"))
}
