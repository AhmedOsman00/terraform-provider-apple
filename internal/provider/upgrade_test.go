// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"image/color"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Upgrade tests apply a configuration with the last released provider, then
// plan the same configuration with the working tree and expect nothing to
// change. That is the 1.x promise -- state written by any release plans
// cleanly on any later one -- and the kind of regression that has actually
// shipped: a refresh or plan modifier that reads the same Apple record
// differently from the release before it, so a provider upgrade alone planned
// an update or a replacement (bundle ID platform, a beta group's
// has_access_to_all_builds).
//
// Each case reuses the configuration of the acceptance test for the same
// resource, under the same guards, and creates the same things at Apple. The
// previous release comes from APPLE_TEST_UPGRADE_FROM or, failing that, the
// newest v* tag reachable from HEAD, and is downloaded from the Registry.
//
// Run them alone with: make testacc-local RUN=TestAccUpgrade, or one with
// RUN=TestAccUpgrade/bundleID.

const testAccUpgradeFromEnvVar = "APPLE_TEST_UPGRADE_FROM"

// testAccUpgradeFrom is the provider release the upgrade tests apply with.
func testAccUpgradeFrom(t *testing.T) string {
	t.Helper()
	if v := strings.TrimPrefix(strings.TrimSpace(os.Getenv(testAccUpgradeFromEnvVar)), "v"); v != "" {
		return v
	}
	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*").Output()
	if err != nil {
		t.Skipf("skipping upgrade test: no v* tag is reachable from HEAD (a shallow checkout has none); set %s to the release to upgrade from", testAccUpgradeFromEnvVar)
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
}

// upgradeCase is one configuration to apply with the previous release and
// plan with the working tree.
type upgradeCase struct {
	// since is the first release that has every resource the configuration
	// uses. A case whose since is newer than the release being upgraded from
	// is skipped: there is nothing to upgrade from yet.
	since        string
	preCheck     func(t *testing.T)
	config       func(t *testing.T) string
	checkDestroy resource.TestCheckFunc
}

func testAccUpgrade(t *testing.T, c upgradeCase) {
	t.Helper()
	// The guards come first, so a run without credentials skips before it
	// asks git for a tag or builds a configuration that generates keys.
	c.preCheck(t)

	from := testAccUpgradeFrom(t)
	if c.since != "" && compareReleases(c.since, from) > 0 {
		t.Skipf("skipping upgrade test: the configuration needs provider %s, newer than %s", c.since, from)
	}
	config := c.config(t)

	resource.Test(t, resource.TestCase{
		CheckDestroy: c.checkDestroy,
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"apple": {Source: "registry.terraform.io/ahmedosman00/apple", VersionConstraint: "= " + from},
				},
				Config: config,
			},
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// compareReleases orders two x.y.z versions.
func compareReleases(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func TestCompareReleases(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"0.8.2", "0.8.2", 0},
		{"0.8.10", "0.8.9", 1},
		{"0.9.0", "1.0.0", -1},
		{"1.0", "1.0.0", 0},
	} {
		if got := compareReleases(tc.a, tc.b); got != tc.want {
			t.Errorf("compareReleases(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func static(config string) func(*testing.T) string {
	return func(*testing.T) string { return config }
}

func testAccUpgradeSubscriptionDestroy() resource.TestCheckFunc {
	return testAccCheckDestroyAll(testAccCheckSubscriptionDestroy, testAccCheckSubscriptionGroupDestroy)
}

// upgradeCases is every configuration the upgrade test applies, by subtest
// name. TestUpgradeCasesCoverEveryResource fails on a registered resource that
// none of them creates and upgradeExempt does not excuse. It is a function
// rather than a variable so that each run draws new product identifiers:
// Apple never releases one, so a second run reusing them would fail.
func upgradeCases() map[string]upgradeCase {
	return map[string]upgradeCase{
		// --- Developer Portal ---
		"bundleID": {
			preCheck:     testAccPreCheck,
			config:       static(testAccBundleIDResourceConfig("com.test.terraform-upgrade", "Terraform Upgrade", "IOS")),
			checkDestroy: testAccCheckBundleIDDestroy,
		},
		"bundleIDCapability": {
			preCheck:     testAccPreCheck,
			config:       static(testAccBundleIDCapabilityResourceConfig("PUSH_NOTIFICATIONS", "")),
			checkDestroy: testAccCheckBundleIDCapabilityDestroy,
		},
		"merchantID": {
			preCheck:     testAccPreCheck,
			config:       static(testAccMerchantIDResourceConfig("merchant.com.test.terraform-upgrade", "Terraform Upgrade Merchant")),
			checkDestroy: testAccCheckMerchantIDDestroy,
		},
		"passTypeID": {
			preCheck:     testAccPreCheck,
			config:       static(testAccPassTypeIDResourceConfig("pass.com.test.terraform-upgrade", "Terraform Upgrade Pass")),
			checkDestroy: testAccCheckPassTypeIDDestroy,
		},
		"certificate": {
			preCheck: testAccPreCheck,
			config: func(t *testing.T) string {
				return testAccCertificateResourceConfig(testAccCertificateType, testAccCertificateCSR(t), "")
			},
			checkDestroy: testAccCheckCertificateDestroy,
		},
		"profile": {
			preCheck: testAccPreCheck,
			config: func(t *testing.T) string {
				return testAccProfileResourceConfig(testAccCertificateCSR(t), "Terraform Upgrade Profile", "IOS_APP_STORE")
			},
			checkDestroy: testAccCheckDestroyAll(testAccCheckProfileDestroy, testAccCheckCertificateDestroy),
		},

		// --- App Store listing ---
		"appSettings": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccAppSettingsConfig("DOES_NOT_USE_THIRD_PARTY_CONTENT")),
			checkDestroy: testAccCheckAppStillExists,
		},
		"appInfo": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccAppInfoConfig("FINANCE", "PRODUCTIVITY")),
			checkDestroy: testAccCheckAppStillExists,
		},
		"appInfoLocalization": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccAppInfoLocalizationConfig("Terraform " + acctest.RandString(6))),
			checkDestroy: testAccCheckAppStillExists,
		},
		"ageRatingDeclaration": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccAgeRatingConfig("NONE")),
			checkDestroy: testAccCheckAppStillExists,
		},
		"appStoreVersion": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccAppStoreVersionConfig(testAccVersionString(), "2026 Terraform Upgrade", "MANUAL")),
			checkDestroy: testAccCheckAppStoreVersionDestroy,
		},
		"appStoreVersionBuild": {
			preCheck:     testAccPreCheckBuild,
			config:       static(testAccAppStoreVersionBuildConfig(os.Getenv(testAccBuildTrainEnvVar), os.Getenv(testAccBuildNumberEnvVar))),
			checkDestroy: testAccCheckAppStoreVersionDestroy,
		},
		"appStoreVersionLocalization": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccAppStoreVersionLocalizationConfig(testAccVersionString(), "Terraform "+acctest.RandString(10), `["terraform", "upgrade"]`)),
			checkDestroy: testAccCheckAppStoreVersionDestroy,
		},
		"appStoreReviewDetail": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccAppStoreReviewDetailConfig(testAccVersionString(), "Terraform upgrade notes "+acctest.RandString(6))),
			checkDestroy: testAccCheckAppStoreVersionDestroy,
		},
		"appScreenshotSet": {
			preCheck: testAccPreCheckSubscription,
			config: func(t *testing.T) string {
				red := testAccScreenshot(t, "red.png", color.RGBA{R: 200, A: 255})
				blue := testAccScreenshot(t, "blue.png", color.RGBA{B: 200, A: 255})
				return testAccAppScreenshotSetConfig(testAccVersionString(), "Terraform "+acctest.RandString(10), red, blue)
			},
			checkDestroy: testAccCheckAppStoreVersionDestroy,
		},
		// The two pricing cases change what the test app costs and where it sells,
		// and cannot put back what was there, so they sit behind the same guard as
		// their acceptance tests.

		"appPriceSchedule": {
			preCheck:     testAccPreCheckAppPricing,
			config:       static(testAccAppPriceScheduleConfig("0")),
			checkDestroy: testAccCheckAppStillExists,
		},
		"appAvailability": {
			preCheck: testAccPreCheckAppPricing,
			config: static(testAccAppAvailabilityConfig(`
    { territory = "USA" },
    { territory = "GBR" },
`)),
			checkDestroy: testAccCheckAppStillExists,
		},

		// --- Subscriptions ---
		"subscriptionGroupLocalization": {
			preCheck: testAccPreCheckSubscription,
			config: static(testAccSubscriptionGroupLocalizationConfig(
				"Terraform Upgrade "+acctest.RandString(6), testAccProductName("Premium"), "")),
			checkDestroy: testAccCheckDestroyAll(testAccCheckSubscriptionGroupLocalizationDestroy, testAccCheckSubscriptionGroupDestroy),
		},
		// subscriptionComplete covers the subscription, its localization, its
		// availability and a single price.
		"subscriptionComplete": {
			preCheck: testAccPreCheckSubscription,
			config: static(testAccSubscriptionCompleteConfig(
				"Terraform Upgrade "+acctest.RandString(6), testAccSubscriptionProductID(), testAccProductName("Pro Monthly"))),
			checkDestroy: testAccUpgradeSubscriptionDestroy(),
		},
		"subscriptionPriceSchedule": {
			preCheck: testAccPreCheckSubscription,
			config: static(testAccSubscriptionPriceScheduleConfig(
				"Terraform Upgrade "+acctest.RandString(6), testAccSubscriptionProductID(), testAccProductName("Pro Schedule"), false)),
			checkDestroy: testAccUpgradeSubscriptionDestroy(),
		},
		"subscriptionIntroductoryOffer": {
			preCheck: testAccPreCheckSubscription,
			config: static(testAccSubscriptionIntroductoryOfferConfig(
				"Terraform Upgrade "+acctest.RandString(6), testAccSubscriptionProductID(), testAccProductName("Pro Intro"), "")),
			checkDestroy: testAccUpgradeSubscriptionDestroy(),
		},
		"subscriptionIntroductoryOfferSchedule": {
			preCheck: testAccPreCheckSubscription,
			config: static(testAccSubscriptionIntroductoryOfferScheduleConfig(
				"Terraform Upgrade "+acctest.RandString(6), testAccSubscriptionProductID(), testAccProductName("Pro Intro Schedule"), false)),
			checkDestroy: testAccUpgradeSubscriptionDestroy(),
		},
		"subscriptionReviewScreenshot": {
			preCheck: testAccPreCheckSubscription,
			config: func(t *testing.T) string {
				red := testAccScreenshot(t, "red.png", color.RGBA{R: 200, A: 255})
				return testAccSubscriptionConfig("Terraform Upgrade "+acctest.RandString(6), testAccSubscriptionProductID(), testAccProductName("Pro Monthly"), "ONE_MONTH") +
					testAccReviewScreenshotConfig("subscription", "apple_subscription.test.id", red)
			},
			checkDestroy: testAccUpgradeSubscriptionDestroy(),
		},

		// --- In-app purchases ---

		// inAppPurchaseComplete covers the purchase, its localization, its price
		// schedule and its availability.
		"inAppPurchaseComplete": {
			preCheck:     testAccPreCheckInAppPurchase,
			config:       static(testAccInAppPurchaseCompleteConfig(testAccInAppPurchaseProductID(), testAccProductName("Pro Unlock"))),
			checkDestroy: testAccCheckInAppPurchaseDestroy,
		},
		"inAppPurchaseReviewScreenshot": {
			preCheck: testAccPreCheckInAppPurchase,
			config: func(t *testing.T) string {
				red := testAccScreenshot(t, "red.png", color.RGBA{R: 200, A: 255})
				return testAccInAppPurchaseConfig(testAccInAppPurchaseProductID(), testAccProductName("Pro Unlock"), "NON_CONSUMABLE") +
					testAccReviewScreenshotConfig("in_app_purchase", "apple_in_app_purchase.test.id", red)
			},
			checkDestroy: testAccCheckInAppPurchaseDestroy,
		},

		// --- TestFlight ---
		"betaGroupExternal": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccBetaGroupConfig(testAccBetaGroupName("Terraform Upgrade External"), false, true)),
			checkDestroy: testAccCheckBetaGroupDestroy,
		},
		"betaGroupInternal": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccBetaGroupInternalConfig(testAccBetaGroupName("Terraform Upgrade Internal"))),
			checkDestroy: testAccCheckBetaGroupDestroy,
		},
		"betaTester": {
			preCheck:     testAccPreCheckBetaTester,
			config:       static(testAccBetaTesterConfig(testAccBetaGroupName("Terraform Upgrade Testers"), testAccBetaTesterEmail())),
			checkDestroy: testAccCheckDestroyAll(testAccCheckBetaTesterDestroy, testAccCheckBetaGroupDestroy),
		},
		"betaAppLocalization": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccBetaAppLocalizationConfig("Terraform upgrade test build. " + acctest.RandString(8))),
			checkDestroy: testAccCheckBetaAppLocalizationDestroy,
		},
		"betaBuildLocalization": {
			preCheck:     testAccPreCheckBuild,
			config:       static(testAccBetaBuildLocalizationConfig("Terraform upgrade test note. " + acctest.RandString(8))),
			checkDestroy: testAccCheckBetaBuildLocalizationDestroy,
		},
		"betaGroupBuild": {
			preCheck:     testAccPreCheckBuild,
			config:       static(testAccBetaGroupBuildConfig(testAccBetaGroupName("Terraform Upgrade Builds"))),
			checkDestroy: testAccCheckDestroyAll(testAccCheckBetaGroupBuildDestroy, testAccCheckBetaGroupDestroy),
		},
		"betaAppReviewDetail": {
			preCheck:     testAccPreCheckSubscription,
			config:       static(testAccBetaAppReviewDetailConfig("Terraform upgrade test. " + acctest.RandString(8))),
			checkDestroy: testAccCheckAppStillExists,
		},

		// --- Users and Access ---
		"userInvitation": {
			preCheck:     testAccPreCheckUserInvitation,
			config:       static(testAccUserInvitationConfig(testAccUserInviteEmail())),
			checkDestroy: testAccCheckUserInvitationDestroy,
		},
		"user": {
			preCheck:     testAccPreCheckUser,
			config:       static(testAccUserConfig(testAccUserEmail(), `["DEVELOPER"]`, false)),
			checkDestroy: testAccCheckUserDestroy,
		},
	}
}

// upgradeExempt names the resources with no upgrade case, and why.
var upgradeExempt = map[string]string{
	"apple_device":      "Apple has no device delete, so every run would register a UDID in the team for good; TestAccDeviceResource passes once per team, ever.",
	"apple_ci_workflow": "Its acceptance test needs an Xcode Cloud product and repository; add a case when that test runs in the staged order.",
}

// TestAccUpgrade runs every case as a subtest, one after another: they share
// the test app, and Apple serializes writes to it badly.
func TestAccUpgrade(t *testing.T) {
	cases := upgradeCases()
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) { testAccUpgrade(t, cases[name]) })
	}
}

// TestUpgradeCasesCoverEveryResource builds every case's configuration, which
// needs no credentials, and checks that between them they create every
// registered resource.
func TestUpgradeCasesCoverEveryResource(t *testing.T) {
	covered := map[string]bool{}
	block := regexp.MustCompile(`resource\s+"(apple_[a-z_]+)"`)
	for name, c := range upgradeCases() {
		for _, m := range block.FindAllStringSubmatch(c.config(t), -1) {
			covered[m[1]] = true
		}
		if c.preCheck == nil || c.checkDestroy == nil {
			t.Errorf("upgrade case %s needs a preCheck and a checkDestroy", name)
		}
	}

	ctx := context.Background()
	for _, newResource := range New("test")().Resources(ctx) {
		metadata := &frameworkresource.MetadataResponse{}
		newResource().Metadata(ctx, frameworkresource.MetadataRequest{ProviderTypeName: "apple"}, metadata)
		name := metadata.TypeName
		_, exempt := upgradeExempt[name]
		switch {
		case covered[name] && exempt:
			t.Errorf("%s has an upgrade case and an exemption; drop the exemption", name)
		case !covered[name] && !exempt:
			t.Errorf("no upgrade case creates %s: add one to upgradeCases, or a reason to upgradeExempt", name)
		}
	}
}
