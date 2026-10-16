// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Screenshots are generated rather than checked in. Apple checks each image's
// pixel dimensions against the display type, and 1290x2796 is the 6.7" iPhone
// portrait size -- legal for APP_IPHONE_67 and accepted as a review screenshot.
// A solid colour compresses to a few kilobytes, and a different colour is a
// different checksum, which is all the tests need to tell files apart.
const (
	testAccScreenshotWidth  = 1290
	testAccScreenshotHeight = 2796
)

// testAccScreenshot writes a solid-colour PNG into the test's temporary
// directory and returns its path.
func testAccScreenshot(t *testing.T, name string, c color.RGBA) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, testAccScreenshotWidth, testAccScreenshotHeight))
	for y := 0; y < testAccScreenshotHeight; y++ {
		for x := 0; x < testAccScreenshotWidth; x++ {
			img.SetRGBA(x, y, c)
		}
	}

	p := filepath.Join(t.TempDir(), name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}

	return p
}

func testAccFileChecksum(t *testing.T, p string) string {
	t.Helper()

	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	return apple.MediaChecksum(data)
}

// TestAccAppScreenshotSetResource_basic covers the three things a set does:
// upload in order, reorder without uploading, and replace changed content.
//
// It creates a real version to hang the set off, deletable only while it is
// being prepared, so an interrupted run leaves one to remove by hand -- the same
// cost TestAccAppStoreVersionLocalizationResource_basic carries.
func TestAccAppScreenshotSetResource_basic(t *testing.T) {
	versionString := testAccVersionString()
	promo := "Terraform " + acctest.RandString(10)

	red := testAccScreenshot(t, "red.png", color.RGBA{R: 200, A: 255})
	blue := testAccScreenshot(t, "blue.png", color.RGBA{B: 200, A: 255})
	green := testAccScreenshot(t, "green.png", color.RGBA{G: 200, A: 255})

	const name = "apple_app_screenshot_set.test"

	// The IDs of the first upload, so the reorder step can show nothing was
	// uploaded again.
	var firstIDs [2]string
	captureIDs := func(s *terraform.State) error {
		rs, err := testAccStateResource(s, name)
		if err != nil {
			return err
		}
		firstIDs = [2]string{rs.Attributes["screenshot_ids.0"], rs.Attributes["screenshot_ids.1"]}
		return nil
	}
	checkSwapped := func(s *terraform.State) error {
		rs, err := testAccStateResource(s, name)
		if err != nil {
			return err
		}
		if rs.Attributes["screenshot_ids.0"] != firstIDs[1] || rs.Attributes["screenshot_ids.1"] != firstIDs[0] {
			return fmt.Errorf("reordering uploaded again: IDs %s, %s were %s, %s",
				rs.Attributes["screenshot_ids.0"], rs.Attributes["screenshot_ids.1"], firstIDs[0], firstIDs[1])
		}
		return nil
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppStoreVersionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAppScreenshotSetConfig(versionString, promo, red, blue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "screenshot_display_type", "APP_IPHONE_67"),
					resource.TestCheckResourceAttr(name, "screenshot_ids.#", "2"),
					resource.TestCheckResourceAttr(name, "screenshot_checksums.0", testAccFileChecksum(t, red)),
					resource.TestCheckResourceAttr(name, "screenshot_checksums.1", testAccFileChecksum(t, blue)),
					captureIDs,
				),
			},
			{
				Config: testAccAppScreenshotSetConfig(versionString, promo, blue, red),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "screenshot_checksums.0", testAccFileChecksum(t, blue)),
					resource.TestCheckResourceAttr(name, "screenshot_checksums.1", testAccFileChecksum(t, red)),
					checkSwapped,
				),
			},
			{
				Config: testAccAppScreenshotSetConfig(versionString, promo, blue, green),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "screenshot_ids.#", "2"),
					resource.TestCheckResourceAttr(name, "screenshot_checksums.1", testAccFileChecksum(t, green)),
				),
			},
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
				// Apple keeps the bytes, not where they came from.
				ImportStateVerifyIgnore: []string{"screenshots"},
			},
		},
	})
}

// TestAccSubscriptionReviewScreenshotResource_basic covers upload, replacement
// on a content change, and import by the subscription's ID.
func TestAccSubscriptionReviewScreenshotResource_basic(t *testing.T) {
	referenceName := "Terraform Sub " + acctest.RandString(6)
	productID := testAccSubscriptionProductID()
	productName := testAccProductName("Pro Monthly")

	red := testAccScreenshot(t, "red.png", color.RGBA{R: 200, A: 255})
	blue := testAccScreenshot(t, "blue.png", color.RGBA{B: 200, A: 255})

	const name = "apple_subscription_review_screenshot.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionConfig(referenceName, productID, productName, "ONE_MONTH") +
					testAccReviewScreenshotConfig("subscription", "apple_subscription.test.id", red),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(name, "id"),
					resource.TestCheckResourceAttr(name, "source_file_checksum", testAccFileChecksum(t, red)),
				),
			},
			{
				Config: testAccSubscriptionConfig(referenceName, productID, productName, "ONE_MONTH") +
					testAccReviewScreenshotConfig("subscription", "apple_subscription.test.id", blue),
				Check: resource.TestCheckResourceAttr(name, "source_file_checksum", testAccFileChecksum(t, blue)),
			},
			{
				ResourceName:            name,
				ImportState:             true,
				ImportStateIdFunc:       testAccSubscriptionParentID(name),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"file"},
			},
		},
	})
}

// TestAccInAppPurchaseReviewScreenshotResource_basic is the subscription test
// for a one-time purchase.
func TestAccInAppPurchaseReviewScreenshotResource_basic(t *testing.T) {
	productID := testAccInAppPurchaseProductID()
	productName := testAccProductName("Pro Unlock")

	red := testAccScreenshot(t, "red.png", color.RGBA{R: 200, A: 255})
	blue := testAccScreenshot(t, "blue.png", color.RGBA{B: 200, A: 255})

	const name = "apple_in_app_purchase_review_screenshot.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckInAppPurchase(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInAppPurchaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccInAppPurchaseConfig(productID, productName, "NON_CONSUMABLE") +
					testAccReviewScreenshotConfig("in_app_purchase", "apple_in_app_purchase.test.id", red),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(name, "id"),
					resource.TestCheckResourceAttr(name, "source_file_checksum", testAccFileChecksum(t, red)),
				),
			},
			{
				Config: testAccInAppPurchaseConfig(productID, productName, "NON_CONSUMABLE") +
					testAccReviewScreenshotConfig("in_app_purchase", "apple_in_app_purchase.test.id", blue),
				Check: resource.TestCheckResourceAttr(name, "source_file_checksum", testAccFileChecksum(t, blue)),
			},
			{
				ResourceName:            name,
				ImportState:             true,
				ImportStateIdFunc:       testAccInAppPurchaseParentID(name),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"file"},
			},
		},
	})
}

func testAccAppScreenshotSetConfig(versionString, promo, first, second string) string {
	return testAccAppStoreVersionLocalizationConfig(versionString, promo, `["budget"]`) + fmt.Sprintf(`
resource "apple_app_screenshot_set" "test" {
  app_store_version_localization_id = apple_app_store_version_localization.test.id
  screenshot_display_type           = "APP_IPHONE_67"
  screenshots                       = [%[1]q, %[2]q]
}
`, first, second)
}

// testAccReviewScreenshotConfig builds either review screenshot resource; the
// two differ only in their type name and parent attribute.
func testAccReviewScreenshotConfig(product, parentRef, file string) string {
	return fmt.Sprintf(`
resource "apple_%[1]s_review_screenshot" "test" {
  %[1]s_id = %[2]s
  file = %[3]q
}
`, product, parentRef, file)
}
