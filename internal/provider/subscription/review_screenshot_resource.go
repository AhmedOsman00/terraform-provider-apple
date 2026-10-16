// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/provider/media"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// NewSubscriptionReviewScreenshotResource manages the image App Review is shown
// of a subscription. The implementation is shared with the in-app purchase one
// in the media package: the two Apple resources differ only in their parent.
func NewSubscriptionReviewScreenshotResource() resource.Resource {
	return media.NewReviewScreenshotResource(media.ReviewScreenshotConfig{
		TypeNameSuffix:  "_subscription_review_screenshot",
		ParentAttribute: "subscription_id",
		ParentResource:  "apple_subscription",
		Product:         "subscription",
		Get:             (*apple.Client).GetSubscriptionReviewScreenshot,
		Upload:          (*apple.Client).UploadSubscriptionReviewScreenshot,
		Delete:          (*apple.Client).DeleteSubscriptionReviewScreenshot,
	})
}
