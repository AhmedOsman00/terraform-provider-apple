// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Subscriptions hang off an App Store Connect app record, and Apple's API
// cannot create one: "Don't use this API to create new apps; instead, create
// new apps on the App Store Connect website." These tests therefore need an app
// that already exists, named by APPLE_TEST_APP_ID, and skip without it.
const testAccAppIDEnvVar = "APPLE_TEST_APP_ID"

// testAccPreCheckSubscription skips unless credentials and a test app are both
// available.
func testAccPreCheckSubscription(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	if os.Getenv(testAccAppIDEnvVar) == "" {
		t.Skipf("skipping subscription acceptance test: %s must name an existing App Store Connect app. "+
			"Apple's API cannot create an app record, so one has to be made in App Store Connect by hand.",
			testAccAppIDEnvVar)
	}
}

func testAccAppID() string {
	return os.Getenv(testAccAppIDEnvVar)
}

// testAccSubscriptionProductID returns a product identifier unique to this run.
//
// Identifiers elsewhere in this suite are fixed, matching the existing tests.
// Subscriptions are the exception on purpose: Apple never releases a product
// identifier, not even one belonging to a subscription that was deleted before
// it was ever approved. A fixed identifier would therefore pass exactly once
// per account and fail on every run afterwards, the way the device tests do.
// Randomising costs nothing -- the subscription itself is deletable -- and
// keeps the suite repeatable.
func testAccSubscriptionProductID() string {
	return fmt.Sprintf("com.test.terraform.sub%s", strings.ToLower(acctest.RandString(8)))
}

// testAccProductName returns a display name unique to this run.
//
// Apple enforces name uniqueness across the live products of an app -- "This
// name is already being used by another in-app purchase associated with this
// app", where "in-app purchase" covers subscriptions too. A fixed name
// therefore collides with whatever an earlier failed test left behind, and
// with the test that ran a second before if Apple has not finished deleting
// its subscription yet. That is a weaker constraint than the one on product
// identifiers, which Apple never releases at all, but it needs the same fix.
//
// Names are capped at 30 characters, so base must leave room for " " plus six.
func testAccProductName(base string) string {
	return fmt.Sprintf("%s %s", base, acctest.RandString(6))
}

func TestAccSubscriptionGroupResource_basic(t *testing.T) {
	referenceName := "Terraform Test " + acctest.RandString(6)
	renamed := "Terraform Renamed " + acctest.RandString(6)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSubscriptionGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionGroupConfig(referenceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionGroupExists("apple_subscription_group.test"),
					resource.TestCheckResourceAttr("apple_subscription_group.test", "reference_name", referenceName),
					resource.TestCheckResourceAttr("apple_subscription_group.test", "app_id", testAccAppID()),
					resource.TestCheckResourceAttrSet("apple_subscription_group.test", "id"),
				),
			},
			// The reference name is the only attribute Apple's update request
			// accepts, so this is the whole of the in-place update path.
			{
				Config: testAccSubscriptionGroupConfig(renamed),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionGroupExists("apple_subscription_group.test"),
					resource.TestCheckResourceAttr("apple_subscription_group.test", "reference_name", renamed),
				),
			},
			// Only the composite form yields complete state: Apple never
			// reports which app a group belongs to.
			{
				ResourceName:      "apple_subscription_group.test",
				ImportState:       true,
				ImportStateIdFunc: testAccSubscriptionGroupImportID("apple_subscription_group.test"),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccSubscriptionGroupResource_importRejectsBareID(t *testing.T) {
	referenceName := "Terraform Import " + acctest.RandString(6)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSubscriptionGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionGroupConfig(referenceName),
				Check:  testAccCheckSubscriptionGroupExists("apple_subscription_group.test"),
			},
			{
				ResourceName:  "apple_subscription_group.test",
				ImportState:   true,
				ImportStateId: "21451234",
				ExpectError:   regexp.MustCompile("Invalid Import ID"),
			},
		},
	})
}

func TestAccSubscriptionResource_basic(t *testing.T) {
	referenceName := "Terraform Sub " + acctest.RandString(6)
	productID := testAccSubscriptionProductID()
	name := testAccProductName("Pro Monthly")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionConfig(referenceName, productID, name, "ONE_MONTH"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription.test", "product_id", productID),
					resource.TestCheckResourceAttr("apple_subscription.test", "name", name),
					resource.TestCheckResourceAttr("apple_subscription.test", "subscription_period", "ONE_MONTH"),
					// A subscription with neither a localization nor a price is
					// incomplete, and Apple says so.
					resource.TestCheckResourceAttr("apple_subscription.test", "state", "MISSING_METADATA"),
					resource.TestCheckResourceAttrPair(
						"apple_subscription.test", "group_id",
						"apple_subscription_group.test", "id",
					),
				),
			},
			// Both the name and the period are updatable in place. The product
			// ID is not, and is covered separately.
			{
				Config: testAccSubscriptionConfig(referenceName, productID, "Pro Yearly", "ONE_YEAR"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription.test", "name", "Pro Yearly"),
					resource.TestCheckResourceAttr("apple_subscription.test", "subscription_period", "ONE_YEAR"),
				),
			},
			// A bare ID suffices: the provider reads with include=group.
			{
				ResourceName:      "apple_subscription.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccSubscriptionResource_completesMetadata covers the reason localizations
// and prices are separate resources: only once both exist does Apple move the
// subscription out of MISSING_METADATA.
func TestAccSubscriptionResource_completesMetadata(t *testing.T) {
	referenceName := "Terraform Full " + acctest.RandString(6)
	productID := testAccSubscriptionProductID()
	name := testAccProductName("Pro Monthly")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionCompleteConfig(referenceName, productID, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription_localization.test", "locale", "en-US"),
					resource.TestCheckResourceAttr("apple_subscription_localization.test", "name", name),
					resource.TestCheckResourceAttrSet("apple_subscription_localization.test", "state"),
					resource.TestCheckResourceAttrSet("apple_subscription_price.test", "id"),
					resource.TestCheckResourceAttrSet("apple_subscription_price.test", "price_point_id"),
					// The price point catalogue must have yielded something, or
					// the price could not have been created at all.
					resource.TestCheckResourceAttrSet(
						"data.apple_subscription_price_points.test", "price_points.0.customer_price"),
				),
			},
			{
				ResourceName:      "apple_subscription_localization.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "apple_subscription_price.test",
				ImportState:       true,
				ImportStateIdFunc: testAccSubscriptionPriceImportID("apple_subscription_price.test"),
				ImportStateVerify: true,
				// preserve_current_price is an instruction rather than a
				// property: Apple reports only the outcome, through
				// "preserved", so import cannot recover it.
				ImportStateVerifyIgnore: []string{"preserve_current_price"},
			},
		},
	})
}

// TestAccSubscriptionResource_requiresReplace covers the product ID being
// immutable. Note that this burns two identifiers rather than one.
func TestAccSubscriptionResource_requiresReplace(t *testing.T) {
	referenceName := "Terraform Replace " + acctest.RandString(6)
	before := testAccSubscriptionProductID()
	after := testAccSubscriptionProductID()
	name := testAccProductName("Pro Monthly")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionConfig(referenceName, before, name, "ONE_MONTH"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription.test", "product_id", before),
				),
			},
			{
				Config: testAccSubscriptionConfig(referenceName, after, name, "ONE_MONTH"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription.test", "product_id", after),
				),
			},
		},
	})
}

// TestAccSubscriptionResource_validation exercises the schema validators. It
// creates nothing at Apple: every step fails at plan time.
func TestAccSubscriptionResource_validation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError *regexp.Regexp
	}{
		{
			name: "invalid subscription period",
			config: `
resource "apple_subscription" "test" {
  group_id            = "12345"
  product_id          = "com.test.terraform.invalid"
  name                = "Invalid"
  subscription_period = "TWO_WEEKS"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
		{
			name: "name longer than Apple's 30 character limit",
			config: `
resource "apple_subscription" "test" {
  group_id   = "12345"
  product_id = "com.test.terraform.invalid"
  name       = "A name that is far too long to be accepted by Apple"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Length`),
		},
		{
			name: "product id with a space",
			config: `
resource "apple_subscription" "test" {
  group_id   = "12345"
  product_id = "com.test.terraform invalid"
  name       = "Invalid"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
		{
			name: "start date given as a timestamp rather than a date",
			config: `
resource "apple_subscription_price" "test" {
  subscription_id = "12345"
  price_point_id  = "abcdef"
  start_date      = "2027-01-01T00:00:00Z"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
		{
			name: "two-letter territory code instead of Apple's three-letter one",
			config: `
resource "apple_subscription_price" "test" {
  subscription_id = "12345"
  price_point_id  = "abcdef"
  territory_id    = "US"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
		{
			name: "malformed locale",
			config: `
resource "apple_subscription_localization" "test" {
  subscription_id = "12345"
  locale          = "English"
  name            = "Pro"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccProviderConfig + tt.config,
						PlanOnly:    true,
						ExpectError: tt.expectError,
					},
				},
			})
		})
	}
}

// --- configurations ---

func testAccSubscriptionGroupConfig(referenceName string) string {
	return testAccProviderConfig + fmt.Sprintf(`
resource "apple_subscription_group" "test" {
  app_id         = %[1]q
  reference_name = %[2]q
}
`, testAccAppID(), referenceName)
}

func testAccSubscriptionConfig(referenceName, productID, name, period string) string {
	return testAccSubscriptionGroupConfig(referenceName) + fmt.Sprintf(`
resource "apple_subscription" "test" {
  group_id            = apple_subscription_group.test.id
  product_id          = %[1]q
  name                = %[2]q
  subscription_period = %[3]q
}
`, productID, name, period)
}

// testAccSubscriptionCompleteConfig builds a subscription with everything Apple
// needs to consider it complete. The price point is looked up rather than
// hardcoded: a price point ID encodes the subscription it belongs to, so it
// cannot be known before the subscription exists.
func testAccSubscriptionCompleteConfig(referenceName, productID, name string) string {
	return testAccSubscriptionConfig(referenceName, productID, name, "ONE_MONTH") + fmt.Sprintf(`
resource "apple_subscription_localization" "test" {
  subscription_id = apple_subscription.test.id
  locale          = "en-US"
  name            = %[1]q
  description     = "Everything in Pro, billed monthly."
}

data "apple_subscription_price_points" "test" {
  subscription_id = apple_subscription.test.id
  territories     = ["USA"]
  limit           = 5
}

resource "apple_subscription_price" "test" {
  subscription_id = apple_subscription.test.id
  price_point_id  = data.apple_subscription_price_points.test.price_points[0].id
}
`, name)
}

// --- import ID functions ---

func testAccSubscriptionGroupImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", rs.Attributes["app_id"], rs.ID), nil
	}
}

func testAccSubscriptionPriceImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", rs.Attributes["subscription_id"], rs.ID), nil
	}
}

// --- existence and destroy checks ---

// testAccCheckSubscriptionGroupExists asks Apple rather than inspecting state,
// so a group that vanished from the portal is caught.
func testAccCheckSubscriptionGroupExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build API client: %w", err)
		}

		group, err := client.GetSubscriptionGroup(rs.ID)
		if err != nil {
			return fmt.Errorf("subscription group %s not found at Apple: %w", rs.ID, err)
		}
		if group.ID != rs.ID {
			return fmt.Errorf("Apple returned group %s, expected %s", group.ID, rs.ID)
		}

		return nil
	}
}

func testAccCheckSubscriptionExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build API client: %w", err)
		}

		subscription, err := client.GetSubscription(rs.ID)
		if err != nil {
			return fmt.Errorf("subscription %s not found at Apple: %w", rs.ID, err)
		}
		if subscription.ID != rs.ID {
			return fmt.Errorf("Apple returned subscription %s, expected %s", subscription.ID, rs.ID)
		}

		return nil
	}
}

func testAccCheckSubscriptionGroupDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("could not build API client: %w", err)
	}

	for name, rs := range s.RootModule().Resources {
		if rs.Type != "apple_subscription_group" {
			continue
		}

		if _, err := client.GetSubscriptionGroup(rs.Primary.ID); err == nil {
			return fmt.Errorf("subscription group %s (%s) still exists at Apple", rs.Primary.ID, name)
		} else if !testAccIsNotFound(err) {
			return fmt.Errorf("unexpected error checking subscription group %s: %w", rs.Primary.ID, err)
		}
	}

	return nil
}

func testAccCheckSubscriptionDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("could not build API client: %w", err)
	}

	for name, rs := range s.RootModule().Resources {
		if rs.Type != "apple_subscription" {
			continue
		}

		if _, err := client.GetSubscription(rs.Primary.ID); err == nil {
			return fmt.Errorf("subscription %s (%s) still exists at Apple", rs.Primary.ID, name)
		} else if !testAccIsNotFound(err) {
			return fmt.Errorf("unexpected error checking subscription %s: %w", rs.Primary.ID, err)
		}
	}

	return nil
}

// testAccIsNotFound reports whether an API error means the resource is gone,
// as opposed to the request having failed for some other reason.
func testAccIsNotFound(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not found") || strings.Contains(msg, "404")
}
