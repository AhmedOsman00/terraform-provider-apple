// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package app manages an App Store Connect app's listing: the settings on the
// app record, its categories, its localized name and product page, its age
// rating, its review information, its price and the storefronts it sells in.
//
// There is deliberately no apple_app resource. Apple's API documentation states
// "Don't use this API to create new apps; instead, create new apps on the App
// Store Connect website", and there is no endpoint to delete one either, so the
// app record itself is always looked up rather than owned -- which is what the
// apple_apps data source is for.
//
// Everything else here hangs off that record, and three things shape all of it:
//
// The metadata is split across two lifetimes. What describes the app -- its
// name, subtitle, categories, privacy policy link and age rating -- lives on an
// AppInfo and survives every release. What describes one release -- the
// description, keywords, promotional text, copyright and release notes -- lives
// on an AppStoreVersion and is replaced with it. Tools that present a single
// flat metadata file paper over that split; Terraform cannot, because the two
// are separate Apple records with separate lifecycles.
//
// Several of these records already exist. Apple creates the AppInfo, the age
// rating declaration and (for a released app) the price schedule and
// availability along with the app, and publishes no POST for most of them. The
// resources covering those adopt what is there on create and drop it from state
// on destroy, rather than pretending to own a lifecycle they do not.
//
// An AppInfo accepts a write only while it is being prepared. A record in
// review or already distributed is frozen, and unlike an in-app purchase
// version there is no way to create a fresh one -- Apple publishes no POST
// /v1/appInfos -- so the categories, the localized name and the age rating
// cannot be changed until the current review finishes.
//
// One thing App Store Connect requires is missing from all of this: App
// Privacy. Apple publishes no endpoint for the data-collection questionnaire at
// all -- there is no appDataUsages resource in the API -- so it has to be
// answered once on the website. It is declared per app rather than per version,
// so it does not recur.
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
