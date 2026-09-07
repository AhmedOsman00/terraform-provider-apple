// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package passtypeid contains all Pass Type ID related resources, data sources, and shared functionality
// for the Apple Terraform provider.
package passtypeid

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// passTypeIDModel maps the Pass Type ID schema data for both resource and data source.
// This is the canonical representation of a Pass Type ID in Terraform state.
type passTypeIDModel struct {
	ID         types.String `tfsdk:"id"`
	Identifier types.String `tfsdk:"identifier"`
	Name       types.String `tfsdk:"name"`
}

// passTypeIDsDataSourceModel maps the data source schema for listing Pass Type IDs.
type passTypeIDsDataSourceModel struct {
	// Filter configuration
	IdentifierPattern types.String `tfsdk:"identifier_pattern"`
	IdentifierPrefix  types.String `tfsdk:"identifier_prefix"`
	NamePattern       types.String `tfsdk:"name_pattern"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	PassTypeIDs []passTypeIDModel `tfsdk:"pass_type_ids"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// GetIdentifierValidator returns validators for Pass Type ID identifiers.
func GetIdentifierValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthAtLeast(1),
		stringvalidator.LengthAtMost(255),
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^pass\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`),
			"Pass Type ID identifier must start with 'pass.' followed by reverse domain name format (e.g., 'pass.com.example.mypass')",
		),
	}
}

// GetNameValidator returns validators for Pass Type ID names.
func GetNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthAtLeast(1),
		stringvalidator.LengthAtMost(255),
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^[a-zA-Z0-9\s\-_.()]+$`),
			"Pass Type ID name can only contain letters, numbers, spaces, hyphens, underscores, periods, and parentheses",
		),
	}
}
