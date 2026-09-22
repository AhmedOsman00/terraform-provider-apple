// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package beta

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestResourceSchemas checks every TestFlight resource schema against the
// framework's own rules.
//
// The framework validates a schema when the provider server starts, so a
// mistake like an attribute that is both Required and Computed would otherwise
// surface only in the acceptance tier -- and that tier needs credentials and a
// real app, so it does not run in CI. This is the credential-free net under the
// five resources, and it pins the resource type names, since renaming one is
// breaking.
func TestResourceSchemas(t *testing.T) {
	t.Parallel()

	resources := map[string]func() resource.Resource{
		"apple_beta_group":              NewBetaGroupResource,
		"apple_beta_tester":             NewBetaTesterResource,
		"apple_beta_app_localization":   NewBetaAppLocalizationResource,
		"apple_beta_build_localization": NewBetaBuildLocalizationResource,
		"apple_beta_app_review_detail":  NewBetaAppReviewDetailResource,
	}

	for name, newResource := range resources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			metadataResp := &resource.MetadataResponse{}
			newResource().Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "apple"}, metadataResp)
			if metadataResp.TypeName != name {
				t.Fatalf("resource type name is %q, want %q", metadataResp.TypeName, name)
			}

			resp := &resource.SchemaResponse{}
			newResource().Schema(ctx, resource.SchemaRequest{}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("schema returned errors: %+v", resp.Diagnostics)
			}

			if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
				t.Fatalf("schema failed validation: %+v", diags)
			}
		})
	}
}
