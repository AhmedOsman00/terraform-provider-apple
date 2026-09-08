// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package app exposes App Store Connect app records to the Apple Terraform
// provider as a read-only data source.
//
// There is deliberately no apple_app resource. Apple's API documentation states
// "Don't use this API to create new apps; instead, create new apps on the App
// Store Connect website", and there is no endpoint to delete one either, so an
// app cannot be managed as a Terraform resource in any meaningful sense. What
// the provider needs from an app is its ID, which every subscription group
// hangs off -- so it looks one up rather than owning it.
package app

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// appModel maps one app record.
type appModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	BundleID      types.String `tfsdk:"bundle_id"`
	SKU           types.String `tfsdk:"sku"`
	PrimaryLocale types.String `tfsdk:"primary_locale"`
}

// appsDataSourceModel maps the listing data source for apps.
type appsDataSourceModel struct {
	// Filter configuration
	BundleID    types.String `tfsdk:"bundle_id"`
	NamePattern types.String `tfsdk:"name_pattern"`
	SKU         types.String `tfsdk:"sku"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	Apps []appModel `tfsdk:"apps"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// App validators.
var (
	// SortByValidator validates sort field options.
	SortByValidator = stringvalidator.OneOf("name", "bundle_id", "sku")

	// SortOrderValidator validates sort order options.
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetBundleIDValidator returns validators for the bundle identifier filter.
//
// This is the app's bundle identifier as a string, matching the identifier of
// an apple_bundle_id resource. It is matched exactly, not as a pattern.
func GetBundleIDValidator() []validator.String {
	return []validator.String{
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-zA-Z0-9.-]+$`),
			"Bundle ID must be a reverse-DNS identifier such as com.example.app",
		),
		stringvalidator.LengthBetween(1, 255),
	}
}

// GetPatternValidator returns validators for regex pattern fields.
func GetPatternValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 255),
	}
}
