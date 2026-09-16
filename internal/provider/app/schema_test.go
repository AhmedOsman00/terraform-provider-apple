// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestResourceSchemas checks every resource schema in this package against the
// framework's own rules.
//
// The framework validates a schema when the provider server starts, which means
// a mistake like an attribute that is both Required and Computed, or a nested
// block with no attributes, would otherwise surface only in the acceptance
// tier -- and that tier needs credentials and a real app, so it does not run in
// CI. This is the credential-free check that the nine resources added for app
// metadata are well formed.
func TestResourceSchemas(t *testing.T) {
	t.Parallel()

	resources := map[string]func() resource.Resource{
		"apple_app_settings":                   NewAppSettingsResource,
		"apple_app_info":                       NewAppInfoResource,
		"apple_app_info_localization":          NewAppInfoLocalizationResource,
		"apple_app_age_rating_declaration":     NewAgeRatingDeclarationResource,
		"apple_app_store_version":              NewAppStoreVersionResource,
		"apple_app_store_version_localization": NewAppStoreVersionLocalizationResource,
		"apple_app_store_review_detail":        NewAppStoreReviewDetailResource,
		"apple_app_price_schedule":             NewAppPriceScheduleResource,
		"apple_app_availability":               NewAppAvailabilityResource,
	}

	for name, newResource := range resources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
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

// TestDataSourceSchemas is TestResourceSchemas for the data sources.
func TestDataSourceSchemas(t *testing.T) {
	t.Parallel()

	dataSources := map[string]func() datasource.DataSource{
		"apple_apps":               NewAppsDataSource,
		"apple_app_categories":     NewAppCategoriesDataSource,
		"apple_app_price_points":   NewAppPricePointsDataSource,
		"apple_app_store_versions": NewAppStoreVersionsDataSource,
	}

	for name, newDataSource := range dataSources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			resp := &datasource.SchemaResponse{}
			newDataSource().Schema(ctx, datasource.SchemaRequest{}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("schema returned errors: %+v", resp.Diagnostics)
			}

			if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
				t.Fatalf("schema failed validation: %+v", diags)
			}
		})
	}
}

// TestResourceTypeNames pins the resource type names.
//
// A rename is a breaking change to a published provider, and the names are
// otherwise only asserted by acceptance tests that do not run in CI.
func TestResourceTypeNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		expected    string
		newResource func() resource.Resource
	}{
		{"apple_app_settings", NewAppSettingsResource},
		{"apple_app_info", NewAppInfoResource},
		{"apple_app_info_localization", NewAppInfoLocalizationResource},
		{"apple_app_age_rating_declaration", NewAgeRatingDeclarationResource},
		{"apple_app_store_version", NewAppStoreVersionResource},
		{"apple_app_store_version_localization", NewAppStoreVersionLocalizationResource},
		{"apple_app_store_review_detail", NewAppStoreReviewDetailResource},
		{"apple_app_price_schedule", NewAppPriceScheduleResource},
		{"apple_app_availability", NewAppAvailabilityResource},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()

			resp := &resource.MetadataResponse{}
			tc.newResource().Metadata(
				context.Background(),
				resource.MetadataRequest{ProviderTypeName: "apple"},
				resp,
			)

			if resp.TypeName != tc.expected {
				t.Errorf("type name = %q, want %q", resp.TypeName, tc.expected)
			}
		})
	}
}
