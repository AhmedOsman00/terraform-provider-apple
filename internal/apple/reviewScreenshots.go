// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"fmt"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// A subscription and an in-app purchase each hold at most one App Store review
// screenshot: the image App Review is shown of the product being sold. They are
// separate Apple resources -- subscriptionAppStoreReviewScreenshots and
// inAppPurchaseAppStoreReviewScreenshots -- reachable only through their parent
// and replaceable only by deleting and uploading again; neither has a PATCH that
// swaps the file.

// GetSubscriptionReviewScreenshot reads a subscription's review screenshot.
func (c *Client) GetSubscriptionReviewScreenshot(subscriptionID string) (*models.MediaAsset, error) {
	return c.getParentMedia(fmt.Sprintf("/v1/subscriptions/%s/appStoreReviewScreenshot", subscriptionID),
		fmt.Sprintf("subscription '%s'", subscriptionID))
}

// UploadSubscriptionReviewScreenshot uploads a subscription's review
// screenshot. Apple refuses a second one while the first exists.
func (c *Client) UploadSubscriptionReviewScreenshot(subscriptionID, fileName string, data []byte) (*models.MediaAsset, error) {
	return c.uploadMedia(subscriptionReviewScreenshotKind, subscriptionID, fileName, data)
}

// DeleteSubscriptionReviewScreenshot removes a subscription's review screenshot.
func (c *Client) DeleteSubscriptionReviewScreenshot(screenshotID string, authToken *string) error {
	return c.deleteMedia(subscriptionReviewScreenshotKind, screenshotID, authToken)
}

// GetInAppPurchaseReviewScreenshot reads an in-app purchase's review screenshot.
// Like everything else about a purchase it is reached through /v2.
func (c *Client) GetInAppPurchaseReviewScreenshot(purchaseID string) (*models.MediaAsset, error) {
	return c.getParentMedia(fmt.Sprintf("/v2/inAppPurchases/%s/appStoreReviewScreenshot", purchaseID),
		fmt.Sprintf("in-app purchase '%s'", purchaseID))
}

// UploadInAppPurchaseReviewScreenshot uploads an in-app purchase's review
// screenshot. The reservation's relationship is inAppPurchaseV2, typed
// inAppPurchases -- the v1 name of a v2 record.
func (c *Client) UploadInAppPurchaseReviewScreenshot(purchaseID, fileName string, data []byte) (*models.MediaAsset, error) {
	return c.uploadMedia(inAppPurchaseReviewScreenshotKind, purchaseID, fileName, data)
}

// DeleteInAppPurchaseReviewScreenshot removes an in-app purchase's review
// screenshot.
func (c *Client) DeleteInAppPurchaseReviewScreenshot(screenshotID string, authToken *string) error {
	return c.deleteMedia(inAppPurchaseReviewScreenshotKind, screenshotID, authToken)
}
