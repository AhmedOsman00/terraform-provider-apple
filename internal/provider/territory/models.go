// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package territory exposes the App Store's storefronts to the Apple Terraform
// provider as a read-only data source.
//
// There is no apple_territory resource and cannot be one: territories are
// Apple's, not the account's. The data source exists for one reason -- an
// availability record has to name every territory a product sells in, and
// "everywhere" is therefore only expressible as a literal list. Reading it from
// Apple is the difference between that list being current and it being whatever
// was pasted into the configuration the day it was written.
package territory

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// territoryModel maps one territory record.
type territoryModel struct {
	ID       types.String `tfsdk:"id"`
	Currency types.String `tfsdk:"currency"`
}

// territoriesDataSourceModel maps the listing data source for territories.
type territoriesDataSourceModel struct {
	// Filter configuration
	Currency  types.String `tfsdk:"currency"`
	IDPattern types.String `tfsdk:"id_pattern"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	Territories []territoryModel `tfsdk:"territories"`
	IDs         []types.String   `tfsdk:"ids"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// Territory validators.
var (
	// SortByValidator validates sort field options.
	SortByValidator = stringvalidator.OneOf("id", "currency")

	// SortOrderValidator validates sort order options.
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetCurrencyValidator returns validators for the currency filter.
//
// Apple reports a territory's currency as an ISO 4217 alphabetic code -- USD,
// GBP, EGP -- so anything else is a typo rather than a filter that matches
// nothing.
func GetCurrencyValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(3, 3),
	}
}

// GetPatternValidator returns validators for regex pattern fields.
func GetPatternValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 255),
	}
}
