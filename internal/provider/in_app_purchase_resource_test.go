// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccPreCheckInAppPurchase skips unless credentials and a test app are both
// available.
//
// In-app purchases hang off an app record for the same reason subscriptions do,
// and Apple's API cannot create one, so the same APPLE_TEST_APP_ID check
// applies -- testAccPreCheckSubscription (subscription_resource_test.go) is it.
func testAccPreCheckInAppPurchase(t *testing.T) {
	t.Helper()
	testAccPreCheckSubscription(t)
}

// testAccInAppPurchaseProductID returns a product identifier unique to this run.
//
// Randomised for the same reason the subscription ones are: Apple never
// releases a product identifier, not even one belonging to a purchase deleted
// before it was ever approved, so a fixed identifier would pass exactly once
// per account and fail on every run afterwards.
func testAccInAppPurchaseProductID() string {
	return fmt.Sprintf("com.test.terraform.iap%s", strings.ToLower(acctest.RandString(8)))
}

func TestAccInAppPurchaseResource_basic(t *testing.T) {
	productID := testAccInAppPurchaseProductID()
	name := testAccProductName("Pro Unlock")
	renamed := testAccProductName("Pro Renamed")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckInAppPurchase(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInAppPurchaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccInAppPurchaseConfig(productID, name, "NON_CONSUMABLE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInAppPurchaseExists("apple_in_app_purchase.test"),
					resource.TestCheckResourceAttr("apple_in_app_purchase.test", "product_id", productID),
					resource.TestCheckResourceAttr("apple_in_app_purchase.test", "name", name),
					resource.TestCheckResourceAttr("apple_in_app_purchase.test", "in_app_purchase_type", "NON_CONSUMABLE"),
					resource.TestCheckResourceAttr("apple_in_app_purchase.test", "app_id", testAccAppID()),
					// A purchase with no localization, price or availability is
					// incomplete, and Apple says so.
					resource.TestCheckResourceAttr("apple_in_app_purchase.test", "state", "MISSING_METADATA"),
					resource.TestCheckResourceAttrSet("apple_in_app_purchase.test", "id"),
				),
			},
			// name, family_sharable and review_note are the only attributes
			// Apple's update request accepts, so this is the whole in-place
			// update path.
			{
				Config: testAccInAppPurchaseUpdatedConfig(productID, renamed),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInAppPurchaseExists("apple_in_app_purchase.test"),
					resource.TestCheckResourceAttr("apple_in_app_purchase.test", "name", renamed),
					resource.TestCheckResourceAttr("apple_in_app_purchase.test", "family_sharable", "true"),
					resource.TestCheckResourceAttr("apple_in_app_purchase.test", "review_note", "Open Settings, then tap Unlock Pro."),
				),
			},
			// Only the composite form yields complete state: Apple never
			// reports which app a purchase belongs to.
			{
				ResourceName:      "apple_in_app_purchase.test",
				ImportState:       true,
				ImportStateIdFunc: testAccInAppPurchaseImportID("apple_in_app_purchase.test"),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccInAppPurchaseResource_importRejectsBareID(t *testing.T) {
	productID := testAccInAppPurchaseProductID()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckInAppPurchase(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInAppPurchaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccInAppPurchaseConfig(productID, "Import Test", "NON_CONSUMABLE"),
				Check:  testAccCheckInAppPurchaseExists("apple_in_app_purchase.test"),
			},
			{
				ResourceName:  "apple_in_app_purchase.test",
				ImportState:   true,
				ImportStateId: "6739472901",
				ExpectError:   regexp.MustCompile("Invalid Import ID"),
			},
		},
	})
}

// TestAccInAppPurchaseResource_completesMetadata exercises the three resources
// that make a purchase sellable: the localization, the price schedule and the
// availability. It is the only test that reaches the inline-price request shape
// and the version resolution behind a localization.
func TestAccInAppPurchaseResource_completesMetadata(t *testing.T) {
	productID := testAccInAppPurchaseProductID()
	name := testAccProductName("Pro Unlock")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckInAppPurchase(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInAppPurchaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccInAppPurchaseCompleteConfig(productID, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInAppPurchaseExists("apple_in_app_purchase.test"),
					resource.TestCheckResourceAttr("apple_in_app_purchase_localization.test", "locale", "en-US"),
					resource.TestCheckResourceAttr("apple_in_app_purchase_localization.test", "name", name),
					// The version is resolved by the provider, never configured.
					resource.TestCheckResourceAttrSet("apple_in_app_purchase_localization.test", "version_id"),
					resource.TestCheckResourceAttr("apple_in_app_purchase_price_schedule.test", "base_territory", "USA"),
					resource.TestCheckResourceAttr("apple_in_app_purchase_price_schedule.test", "prices.#", "1"),
					resource.TestCheckResourceAttrSet("apple_in_app_purchase_price_schedule.test", "id"),
					resource.TestCheckResourceAttr("apple_in_app_purchase_availability.test", "available_territories.#", "1"),
					resource.TestCheckResourceAttr("apple_in_app_purchase_availability.test", "available_in_new_territories", "false"),
				),
			},
			{
				ResourceName:      "apple_in_app_purchase_localization.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Both singular resources import by the in-app purchase ID: Apple
			// publishes no collection of price schedules or availabilities.
			{
				ResourceName:      "apple_in_app_purchase_price_schedule.test",
				ImportState:       true,
				ImportStateIdFunc: testAccInAppPurchaseParentID("apple_in_app_purchase_price_schedule.test"),
				ImportStateVerify: true,
				// Import adopts Apple's dates, while an applied schedule keeps
				// the configured ones -- deliberately, because Apple derives an
				// end date for any price a later one supersedes. The two agree
				// for a price with no dates set, but not reliably enough to
				// assert on; base_territory and the schedule ID still are.
				ImportStateVerifyIgnore: []string{"prices"},
			},
			{
				ResourceName:      "apple_in_app_purchase_availability.test",
				ImportState:       true,
				ImportStateIdFunc: testAccInAppPurchaseParentID("apple_in_app_purchase_availability.test"),
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccInAppPurchaseResource_validation covers the plan-time validators.
// Every step is rejected before a request reaches Apple, so this creates
// nothing and needs no app.
func TestAccInAppPurchaseResource_validation(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "apple_in_app_purchase" "test" {
  app_id               = "6478123456"
  product_id           = "com.test.terraform.iap"
  name                 = "Bad Type"
  in_app_purchase_type = "AUTO_RENEWABLE"
}
`,
				ExpectError: regexp.MustCompile(`Attribute in_app_purchase_type value must be one of`),
			},
			{
				Config: `
resource "apple_in_app_purchase" "test" {
  app_id               = "6478123456"
  product_id           = "com test terraform"
  name                 = "Bad Product ID"
  in_app_purchase_type = "NON_CONSUMABLE"
}
`,
				ExpectError: regexp.MustCompile(`Product ID must start with a letter or digit`),
			},
			{
				// 33 characters, past the 30 Apple accepts for a
				// customer-facing name. Counted in characters, not bytes.
				Config: `
resource "apple_in_app_purchase_localization" "test" {
  in_app_purchase_id = "6739472901"
  locale             = "en-US"
  name               = "Far Too Long A Name For The Store"
}
`,
				ExpectError: regexp.MustCompile(`Attribute name UTF-8 character count must be between 1 and 30`),
			},
			{
				Config: `
resource "apple_in_app_purchase_price_schedule" "test" {
  in_app_purchase_id = "6739472901"
  base_territory     = "US"

  prices = [{ price_point_id = "eyJzIjoi" }]
}
`,
				ExpectError: regexp.MustCompile(`Territory must be a three-letter`),
			},
			{
				Config: `
resource "apple_in_app_purchase_price_schedule" "test" {
  in_app_purchase_id = "6739472901"
  base_territory     = "USA"

  prices = [{
    price_point_id = "eyJzIjoi"
    start_date     = "2026-10-01T00:00:00Z"
  }]
}
`,
				ExpectError: regexp.MustCompile(`Date must be a plain date in YYYY-MM-DD form`),
			},
			{
				// A schedule with no prices prices nothing, and Apple rejects
				// it -- catch that before the request.
				Config: `
resource "apple_in_app_purchase_price_schedule" "test" {
  in_app_purchase_id = "6739472901"
  base_territory     = "USA"

  prices = []
}
`,
				ExpectError: regexp.MustCompile(`Attribute prices set must contain at least 1 elements`),
			},
			{
				Config: `
resource "apple_in_app_purchase_availability" "test" {
  in_app_purchase_id    = "6739472901"
  available_territories = ["US"]
}
`,
				ExpectError: regexp.MustCompile(`Territory must be a three-letter`),
			},
		},
	})
}

// --- configurations ---

func testAccInAppPurchaseConfig(productID, name, purchaseType string) string {
	return fmt.Sprintf(`
resource "apple_in_app_purchase" "test" {
  app_id               = %[1]q
  product_id           = %[2]q
  name                 = %[3]q
  in_app_purchase_type = %[4]q
}
`, testAccAppID(), productID, name, purchaseType)
}

func testAccInAppPurchaseUpdatedConfig(productID, name string) string {
	return fmt.Sprintf(`
resource "apple_in_app_purchase" "test" {
  app_id               = %[1]q
  product_id           = %[2]q
  name                 = %[3]q
  in_app_purchase_type = "NON_CONSUMABLE"
  family_sharable      = true
  review_note          = "Open Settings, then tap Unlock Pro."
}
`, testAccAppID(), productID, name)
}

// testAccInAppPurchaseCompleteConfig builds a purchase with everything Apple
// needs to consider it configured.
//
// The price point is read back from Apple rather than hardcoded: a price point
// ID encodes the purchase it belongs to, so it cannot be known before the
// purchase exists.
func testAccInAppPurchaseCompleteConfig(productID, name string) string {
	return fmt.Sprintf(`
resource "apple_in_app_purchase" "test" {
  app_id               = %[1]q
  product_id           = %[2]q
  name                 = %[3]q
  in_app_purchase_type = "NON_CONSUMABLE"
}

resource "apple_in_app_purchase_localization" "test" {
  in_app_purchase_id = apple_in_app_purchase.test.id
  locale             = "en-US"
  name               = %[3]q
  description        = "Unlock every Pro feature."
}

data "apple_in_app_purchase_price_points" "usa" {
  in_app_purchase_id = apple_in_app_purchase.test.id
  territories        = ["USA"]
  customer_price     = "0.99"
  limit              = 1
}

resource "apple_in_app_purchase_price_schedule" "test" {
  in_app_purchase_id = apple_in_app_purchase.test.id
  base_territory     = "USA"

  prices = [{
    price_point_id = data.apple_in_app_purchase_price_points.usa.price_points[0].id
  }]
}

resource "apple_in_app_purchase_availability" "test" {
  in_app_purchase_id           = apple_in_app_purchase.test.id
  available_in_new_territories = false
  available_territories        = ["USA"]
}
`, testAccAppID(), productID, name)
}

// --- import ID functions ---

func testAccInAppPurchaseImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", rs.Attributes["app_id"], rs.ID), nil
	}
}

// testAccInAppPurchaseParentID imports by the parent purchase rather than the
// resource's own ID, which is what the price schedule and availability need.
func testAccInAppPurchaseParentID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}
		return rs.Attributes["in_app_purchase_id"], nil
	}
}

// --- existence and destroy checks ---

// testAccCheckInAppPurchaseExists asks Apple rather than inspecting state, so a
// purchase that vanished from the portal is caught.
func testAccCheckInAppPurchaseExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build API client: %w", err)
		}

		purchase, err := client.GetInAppPurchase(rs.ID)
		if err != nil {
			return fmt.Errorf("in-app purchase %s not found at Apple: %w", rs.ID, err)
		}
		if purchase.ID != rs.ID {
			return fmt.Errorf("Apple returned in-app purchase %s, expected %s", purchase.ID, rs.ID)
		}

		return nil
	}
}

func testAccCheckInAppPurchaseDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("could not build API client: %w", err)
	}

	for name, rs := range s.RootModule().Resources {
		if rs.Type != "apple_in_app_purchase" {
			continue
		}

		if _, err := client.GetInAppPurchase(rs.Primary.ID); err == nil {
			return fmt.Errorf("in-app purchase %s (%s) still exists at Apple", rs.Primary.ID, name)
		} else if !testAccIsNotFound(err) {
			return fmt.Errorf("unexpected error checking in-app purchase %s: %w", rs.Primary.ID, err)
		}
	}

	return nil
}
