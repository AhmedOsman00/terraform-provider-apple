// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package media_test

import (
	"context"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/provider/inapppurchase"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/provider/subscription"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestReviewScreenshotResources validates both instantiations of the shared
// review screenshot resource and pins their type names. The parent attribute
// is supplied by configuration rather than a struct tag, so a typo there is
// exactly what ValidateImplementation and the attribute check below exist for.
func TestReviewScreenshotResources(t *testing.T) {
	t.Parallel()

	cases := []struct {
		typeName        string
		parentAttribute string
		newResource     func() resource.Resource
	}{
		{"apple_subscription_review_screenshot", "subscription_id", subscription.NewSubscriptionReviewScreenshotResource},
		{"apple_in_app_purchase_review_screenshot", "in_app_purchase_id", inapppurchase.NewInAppPurchaseReviewScreenshotResource},
	}

	for _, tc := range cases {
		t.Run(tc.typeName, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			r := tc.newResource()

			meta := &resource.MetadataResponse{}
			r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "apple"}, meta)
			if meta.TypeName != tc.typeName {
				t.Errorf("type name = %q, want %q", meta.TypeName, tc.typeName)
			}

			resp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("schema returned errors: %+v", resp.Diagnostics)
			}
			if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
				t.Fatalf("schema failed validation: %+v", diags)
			}

			for _, name := range []string{"id", tc.parentAttribute, "file", "source_file_checksum"} {
				if _, ok := resp.Schema.Attributes[name]; !ok {
					t.Errorf("schema has no %q attribute", name)
				}
			}
		})
	}
}
