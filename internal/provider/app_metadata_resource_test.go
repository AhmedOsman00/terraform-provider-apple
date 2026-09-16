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

// App listing metadata hangs off an app record Apple's API cannot create, so
// these tests need APPLE_TEST_APP_ID the same way the subscription and in-app
// purchase tests do -- testAccPreCheckSubscription (subscription_resource_test.go)
// is that check.
//
// They differ from the rest of the suite in what they leave behind. A
// subscription test creates a product and deletes it; these edit the listing of
// an app that already exists and mostly cannot put it back:
//
//   - apple_app_settings, apple_app_info and apple_app_age_rating_declaration
//     adopt records Apple created with the app. There is nothing to delete, so
//     CheckDestroy only asserts that the app is still readable, and the values
//     the test wrote stay on the app afterwards.
//   - apple_app_store_version creates a real version, which is deletable only
//     while it is still being prepared. An interrupted run leaves one to delete
//     in App Store Connect.
//   - The localizations and the review detail hang off that version and go away
//     with it.
//
// Nothing here touches the app's price or the storefronts it sells in. Those
// two tests are guarded separately -- see the comment on
// testAccPreCheckAppPricing.
//
// Use a scratch app for APPLE_TEST_APP_ID. These tests rewrite its categories,
// its age rating answers and its localized name.

// testAccAppPricingEnvVar opts in to the two tests that change what an app
// costs and where it sells.
//
// Those are not reversible in the way the rest of the suite is: Apple publishes
// no DELETE for either a price schedule or an app availability, so the test
// cannot put back what was there before, and on a live app it would change what
// customers pay and which storefronts can buy it. Credentials plus a test app
// are deliberately not enough to run them.
const testAccAppPricingEnvVar = "APPLE_TEST_ALLOW_APP_PRICING"

// testAccPreCheckAppPricing skips unless pricing changes are explicitly allowed.
func testAccPreCheckAppPricing(t *testing.T) {
	t.Helper()
	testAccPreCheckSubscription(t)

	if os.Getenv(testAccAppPricingEnvVar) == "" {
		t.Skipf("skipping app pricing acceptance test: set %s=1 to allow it. "+
			"This test changes the price and availability of the app named by %s, and Apple publishes "+
			"no DELETE for either record, so it cannot be undone. Never point it at a live app.",
			testAccAppPricingEnvVar, testAccAppIDEnvVar)
	}
}

// testAccVersionString returns a version string unique to this run.
//
// An app holds only one editable version per platform at a time, so a fixed
// string would collide with whatever an interrupted run left behind. The major
// number is pinned high enough not to collide with a real release on a scratch
// app.
func testAccVersionString() string {
	return fmt.Sprintf("99.%d.%d", acctest.RandIntRange(0, 999), acctest.RandIntRange(0, 999))
}

// --- apple_app_settings -------------------------------------------------------

func TestAccAppSettingsResource_contentRights(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStillExists,
		Steps: []resource.TestStep{
			{
				Config: testAccAppSettingsConfig("DOES_NOT_USE_THIRD_PARTY_CONTENT"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_settings.test", "app_id", testAccAppID()),
					resource.TestCheckResourceAttr("apple_app_settings.test",
						"content_rights_declaration", "DOES_NOT_USE_THIRD_PARTY_CONTENT"),
					resource.TestCheckResourceAttrSet("apple_app_settings.test", "bundle_id"),
					resource.TestCheckResourceAttrSet("apple_app_settings.test", "sku"),
				),
			},
			// The whole point of the resource: flipping the declaration App
			// Store Connect blocks a submission on.
			{
				Config: testAccAppSettingsConfig("USES_THIRD_PARTY_CONTENT"),
				Check: resource.TestCheckResourceAttr("apple_app_settings.test",
					"content_rights_declaration", "USES_THIRD_PARTY_CONTENT"),
			},
			{
				ResourceName:      "apple_app_settings.test",
				ImportState:       true,
				ImportStateId:     testAccAppID(),
				ImportStateVerify: true,
				// Apple omits a URL it holds no value for, so Read leaves the
				// optional ones null rather than writing over configuration.
				ImportStateVerifyIgnore: []string{
					"accessibility_url",
					"subscription_status_url",
					"subscription_status_url_version",
					"subscription_status_url_for_sandbox",
					"subscription_status_url_version_for_sandbox",
				},
			},
		},
	})
}

func testAccAppSettingsConfig(declaration string) string {
	return fmt.Sprintf(`
resource "apple_app_settings" "test" {
  app_id                     = %[1]q
  content_rights_declaration = %[2]q
}
`, testAccAppID(), declaration)
}

// --- apple_app_info -----------------------------------------------------------

func TestAccAppInfoResource_categories(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStillExists,
		Steps: []resource.TestStep{
			{
				Config: testAccAppInfoConfig("FINANCE", "PRODUCTIVITY"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_info.test", "primary_category", "FINANCE"),
					resource.TestCheckResourceAttr("apple_app_info.test", "secondary_category", "PRODUCTIVITY"),
					resource.TestCheckResourceAttrSet("apple_app_info.test", "id"),
					resource.TestCheckResourceAttrSet("apple_app_info.test", "state"),
				),
			},
			{
				Config: testAccAppInfoConfig("PRODUCTIVITY", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_info.test", "primary_category", "PRODUCTIVITY"),
					// An unset category is sent as an explicit null, so the
					// secondary really is removed rather than left behind.
					resource.TestCheckNoResourceAttr("apple_app_info.test", "secondary_category"),
				),
			},
			{
				ResourceName:      "apple_app_info.test",
				ImportState:       true,
				ImportStateId:     testAccAppID(),
				ImportStateVerify: true,
			},
		},
	})
}

func testAccAppInfoConfig(primary, secondary string) string {
	secondaryLine := ""
	if secondary != "" {
		secondaryLine = fmt.Sprintf("  secondary_category = %q", secondary)
	}

	return fmt.Sprintf(`
resource "apple_app_info" "test" {
  app_id           = %[1]q
  primary_category = %[2]q
%[3]s
}
`, testAccAppID(), primary, secondaryLine)
}

// --- apple_app_info_localization ----------------------------------------------

func TestAccAppInfoLocalizationResource_basic(t *testing.T) {
	subtitle := "Terraform " + acctest.RandString(6)
	renamed := "Terraform " + acctest.RandString(6)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStillExists,
		Steps: []resource.TestStep{
			{
				Config: testAccAppInfoLocalizationConfig(subtitle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_info_localization.test", "locale", "en-US"),
					resource.TestCheckResourceAttr("apple_app_info_localization.test", "subtitle", subtitle),
					resource.TestCheckResourceAttrSet("apple_app_info_localization.test", "app_info_id"),
				),
			},
			{
				Config: testAccAppInfoLocalizationConfig(renamed),
				Check: resource.TestCheckResourceAttr(
					"apple_app_info_localization.test", "subtitle", renamed),
			},
			{
				ResourceName:      "apple_app_info_localization.test",
				ImportState:       true,
				ImportStateId:     testAccAppID() + "/en-US",
				ImportStateVerify: true,
			},
		},
	})
}

// testAccAppInfoLocalizationConfig varies the subtitle, not the name.
//
// name is Required, so the test does rewrite the scratch app's localized name
// once -- to a fixed string, so a second run is a no-op rather than another
// rename. The subtitle is what varies, because it is the smallest field that
// still exercises create, update and import.
func testAccAppInfoLocalizationConfig(subtitle string) string {
	return fmt.Sprintf(`
resource "apple_app_info_localization" "test" {
  app_id   = %[1]q
  locale   = "en-US"
  name     = "TF Acceptance App"
  subtitle = %[2]q
}
`, testAccAppID(), subtitle)
}

// --- apple_app_age_rating_declaration -----------------------------------------

func TestAccAgeRatingDeclarationResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStillExists,
		Steps: []resource.TestStep{
			{
				Config: testAccAgeRatingConfig("NONE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_age_rating_declaration.test",
						"violence_cartoon_or_fantasy", "NONE"),
					resource.TestCheckResourceAttr("apple_app_age_rating_declaration.test",
						"user_generated_content", "false"),
					resource.TestCheckResourceAttrSet("apple_app_age_rating_declaration.test", "id"),
					resource.TestCheckResourceAttrSet("apple_app_age_rating_declaration.test", "app_info_id"),
				),
			},
			// Raising one answer is what proves the PATCH lands: Apple
			// recomputes the rating from it.
			{
				Config: testAccAgeRatingConfig("INFREQUENT_OR_MILD"),
				Check: resource.TestCheckResourceAttr("apple_app_age_rating_declaration.test",
					"violence_cartoon_or_fantasy", "INFREQUENT_OR_MILD"),
			},
			{
				ResourceName:      "apple_app_age_rating_declaration.test",
				ImportState:       true,
				ImportStateId:     testAccAppID(),
				ImportStateVerify: true,
				// Optional rather than Optional+Computed: an app outside the
				// Kids Category has no band, and a null in state is the same as
				// unset in configuration.
				ImportStateVerifyIgnore: []string{"kids_age_band", "developer_age_rating_info_url"},
			},
		},
	})
}

func testAccAgeRatingConfig(cartoonViolence string) string {
	return fmt.Sprintf(`
resource "apple_app_age_rating_declaration" "test" {
  app_id = %[1]q

  violence_cartoon_or_fantasy = %[2]q

  alcohol_tobacco_or_drug_use_or_references = "NONE"
  contests                                  = "NONE"
  gambling_simulated                        = "NONE"
  horror_or_fear_themes                     = "NONE"
  mature_or_suggestive_themes               = "NONE"
  medical_or_treatment_information          = "NONE"
  profanity_or_crude_humor                  = "NONE"
  sexual_content_graphic_and_nudity         = "NONE"
  sexual_content_or_nudity                  = "NONE"
  violence_realistic                        = "NONE"

  gambling                = false
  unrestricted_web_access = false
  user_generated_content  = false
}
`, testAccAppID(), cartoonViolence)
}

// --- apple_app_store_version and the records hanging off it -------------------

func TestAccAppStoreVersionResource_basic(t *testing.T) {
	versionString := testAccVersionString()
	copyright := "2026 Terraform " + acctest.RandString(6)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStoreVersionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAppStoreVersionConfig(versionString, copyright, "MANUAL"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAppStoreVersionExists("apple_app_store_version.test"),
					resource.TestCheckResourceAttr("apple_app_store_version.test", "version_string", versionString),
					resource.TestCheckResourceAttr("apple_app_store_version.test", "copyright", copyright),
					resource.TestCheckResourceAttr("apple_app_store_version.test", "release_type", "MANUAL"),
					resource.TestCheckResourceAttr("apple_app_store_version.test", "platform", "IOS"),
					resource.TestCheckResourceAttrSet("apple_app_store_version.test", "app_version_state"),
				),
			},
			// release_type is the attribute an "automatic release" setting maps
			// onto, and both it and the copyright update in place.
			{
				Config: testAccAppStoreVersionConfig(versionString, "2026 Renamed", "AFTER_APPROVAL"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_store_version.test", "copyright", "2026 Renamed"),
					resource.TestCheckResourceAttr("apple_app_store_version.test", "release_type", "AFTER_APPROVAL"),
				),
			},
			{
				ResourceName:      "apple_app_store_version.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Apple normalizes the timestamp to UTC whatever offset was
				// sent, so Read never refreshes it -- see applyAppStoreVersion.
				ImportStateVerifyIgnore: []string{"earliest_release_date"},
			},
		},
	})
}

func TestAccAppStoreVersionLocalizationResource_basic(t *testing.T) {
	versionString := testAccVersionString()
	promo := "Terraform " + acctest.RandString(10)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStoreVersionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAppStoreVersionLocalizationConfig(versionString, promo,
					`["budget", "expenses", "spending"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_store_version_localization.test", "locale", "en-US"),
					resource.TestCheckResourceAttr("apple_app_store_version_localization.test", "promotional_text", promo),
					// The list round-trips through Apple's single
					// comma-separated string.
					resource.TestCheckResourceAttr("apple_app_store_version_localization.test", "keywords.#", "3"),
					resource.TestCheckResourceAttr("apple_app_store_version_localization.test", "keywords.0", "budget"),
					resource.TestCheckResourceAttr("apple_app_store_version_localization.test", "keywords.2", "spending"),
				),
			},
			{
				Config: testAccAppStoreVersionLocalizationConfig(versionString, promo,
					`["budget", "money"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_store_version_localization.test", "keywords.#", "2"),
					resource.TestCheckResourceAttr("apple_app_store_version_localization.test", "keywords.1", "money"),
				),
			},
			{
				ResourceName:      "apple_app_store_version_localization.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccAppStoreVersionLocalizationResource_keywordsTooLong covers the one
// validation no per-element validator can express: the cap applies to the
// comma-joined string, so individually legal keywords can be too long together.
func TestAccAppStoreVersionLocalizationResource_keywordsTooLong(t *testing.T) {
	long := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		long = append(long, fmt.Sprintf("%q", "keyword"+acctest.RandString(6)))
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAppStoreVersionLocalizationConfig(
					testAccVersionString(), "Promo", "["+strings.Join(long, ", ")+"]"),
				ExpectError: regexp.MustCompile("Keywords Too Long"),
			},
		},
	})
}

func TestAccAppStoreReviewDetailResource_basic(t *testing.T) {
	versionString := testAccVersionString()
	notes := "Terraform test notes " + acctest.RandString(6)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStoreVersionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAppStoreReviewDetailConfig(versionString, notes),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_store_review_detail.test", "notes", notes),
					resource.TestCheckResourceAttr("apple_app_store_review_detail.test", "demo_account_required", "false"),
					resource.TestCheckResourceAttr("apple_app_store_review_detail.test", "contact_email", "review@example.com"),
					resource.TestCheckResourceAttrSet("apple_app_store_review_detail.test", "id"),
				),
			},
			{
				Config: testAccAppStoreReviewDetailConfig(versionString, "Updated "+notes),
				Check: resource.TestCheckResourceAttr(
					"apple_app_store_review_detail.test", "notes", "Updated "+notes),
			},
			{
				ResourceName:      "apple_app_store_review_detail.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Apple returns the password masked on some accounts and
				// omitted on others, so Read never refreshes it.
				ImportStateVerifyIgnore: []string{"demo_account_password"},
			},
		},
	})
}

func testAccAppStoreVersionConfig(versionString, copyright, releaseType string) string {
	return fmt.Sprintf(`
resource "apple_app_store_version" "test" {
  app_id         = %[1]q
  platform       = "IOS"
  version_string = %[2]q
  copyright      = %[3]q
  release_type   = %[4]q
}
`, testAccAppID(), versionString, copyright, releaseType)
}

func testAccAppStoreVersionLocalizationConfig(versionString, promo, keywords string) string {
	return testAccAppStoreVersionConfig(versionString, "2026 Terraform", "MANUAL") + fmt.Sprintf(`
resource "apple_app_store_version_localization" "test" {
  app_store_version_id = apple_app_store_version.test.id
  locale               = "en-US"

  description      = "A test product page written by the Terraform provider's acceptance suite."
  promotional_text = %[1]q
  keywords         = %[2]s
  support_url      = "https://example.com/support"
}
`, promo, keywords)
}

func testAccAppStoreReviewDetailConfig(versionString, notes string) string {
	return testAccAppStoreVersionConfig(versionString, "2026 Terraform", "MANUAL") + fmt.Sprintf(`
resource "apple_app_store_review_detail" "test" {
  app_store_version_id = apple_app_store_version.test.id

  contact_first_name = "Terraform"
  contact_last_name  = "Acceptance"
  contact_email      = "review@example.com"
  contact_phone      = "+1 555 0100"

  demo_account_required = false
  notes                 = %[1]q
}
`, notes)
}

// --- apple_app_price_schedule and apple_app_availability ----------------------

// TestAccAppPriceScheduleResource_basic changes what the test app costs and
// cannot put it back -- see testAccPreCheckAppPricing.
func TestAccAppPriceScheduleResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckAppPricing(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		// No CheckDestroy against Apple: the schedule cannot be deleted, so the
		// only honest assertion is that the app is still there.
		CheckDestroy: testAccCheckAppStillExists,
		Steps: []resource.TestStep{
			{
				Config: testAccAppPriceScheduleConfig("0"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_price_schedule.test", "base_territory", "USA"),
					resource.TestCheckResourceAttr("apple_app_price_schedule.test", "prices.#", "1"),
					resource.TestCheckResourceAttrSet("apple_app_price_schedule.test", "id"),
				),
			},
			{
				ResourceName:      "apple_app_price_schedule.test",
				ImportState:       true,
				ImportStateId:     testAccAppID(),
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccAppAvailabilityResource_basic changes which storefronts the test app
// sells in and cannot put it back -- see testAccPreCheckAppPricing.
func TestAccAppAvailabilityResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckAppPricing(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStillExists,
		Steps: []resource.TestStep{
			{
				Config: testAccAppAvailabilityConfig(`
    { territory = "USA" },
    { territory = "GBR" },
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_app_availability.test", "territories.#", "2"),
					resource.TestCheckResourceAttr("apple_app_availability.test",
						"available_in_new_territories", "true"),
					resource.TestCheckResourceAttrSet("apple_app_availability.test", "id"),
				),
			},
			// Apple replaces the record on every POST and issues a new ID, so
			// this also exercises the ModifyPlan that marks the ID unknown.
			{
				Config: testAccAppAvailabilityConfig(`
    { territory = "USA" },
    { territory = "GBR" },
    { territory = "DEU" },
`),
				Check: resource.TestCheckResourceAttr("apple_app_availability.test", "territories.#", "3"),
			},
			{
				ResourceName:      "apple_app_availability.test",
				ImportState:       true,
				ImportStateId:     testAccAppID(),
				ImportStateVerify: true,
			},
		},
	})
}

func testAccAppPriceScheduleConfig(customerPrice string) string {
	return fmt.Sprintf(`
data "apple_app_price_points" "test" {
  app_id         = %[1]q
  territories    = ["USA"]
  customer_price = %[2]q
}

resource "apple_app_price_schedule" "test" {
  app_id         = %[1]q
  base_territory = "USA"

  prices = [
    {
      price_point_id = data.apple_app_price_points.test.price_points[0].id
    },
  ]
}
`, testAccAppID(), customerPrice)
}

func testAccAppAvailabilityConfig(territories string) string {
	return fmt.Sprintf(`
resource "apple_app_availability" "test" {
  app_id                       = %[1]q
  available_in_new_territories = true

  territories = [
%[2]s
  ]
}
`, testAccAppID(), territories)
}

// --- Checks --------------------------------------------------------------------

// testAccCheckAppStillExists is the CheckDestroy for the resources that adopt a
// record rather than creating one.
//
// A CheckDestroy that only inspected Terraform state would pass trivially here,
// and there is nothing at Apple to assert gone: destroying one of these drops
// it from state and leaves the record alone by design. What is worth asserting
// is that the test did not somehow remove the app.
func testAccCheckAppStillExists(_ *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return err
	}

	if _, err := client.GetApp(testAccAppID()); err != nil {
		return fmt.Errorf("app %s is no longer readable after the test: %w", testAccAppID(), err)
	}

	return nil
}

// testAccCheckAppStoreVersionExists asks Apple, not Terraform state.
func testAccCheckAppStoreVersionExists(name string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("resource %s has no ID", name)
		}

		client, err := testAccAPIClient()
		if err != nil {
			return err
		}

		if _, err := client.GetAppStoreVersion(rs.Primary.ID); err != nil {
			return fmt.Errorf("app store version %s not found at Apple: %w", rs.Primary.ID, err)
		}

		return nil
	}
}

// testAccCheckAppStoreVersionDestroy confirms the version really went away.
//
// Only an editable version can be deleted; one that reached review would have
// warned and dropped state instead, so a surviving record here means the test
// left something to clean up by hand.
func testAccCheckAppStoreVersionDestroy(state *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return err
	}

	for _, rs := range state.RootModule().Resources {
		if rs.Type != "apple_app_store_version" {
			continue
		}

		if _, err := client.GetAppStoreVersion(rs.Primary.ID); err == nil {
			return fmt.Errorf("app store version %s still exists at Apple; delete it in App Store Connect",
				rs.Primary.ID)
		}
	}

	return nil
}
