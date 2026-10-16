// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package inapppurchase

import (
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/provider/media"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// NewInAppPurchaseReviewScreenshotResource manages the image App Review is shown
// of a one-time purchase. The implementation is shared with the subscription
// one in the media package: the two Apple resources differ only in their
// parent.
func NewInAppPurchaseReviewScreenshotResource() resource.Resource {
	return media.NewReviewScreenshotResource(media.ReviewScreenshotConfig{
		TypeNameSuffix:  "_in_app_purchase_review_screenshot",
		ParentAttribute: "in_app_purchase_id",
		ParentResource:  "apple_in_app_purchase",
		Product:         "in-app purchase",
		Get:             (*apple.Client).GetInAppPurchaseReviewScreenshot,
		Upload:          (*apple.Client).UploadInAppPurchaseReviewScreenshot,
		Delete:          (*apple.Client).DeleteInAppPurchaseReviewScreenshot,
	})
}
