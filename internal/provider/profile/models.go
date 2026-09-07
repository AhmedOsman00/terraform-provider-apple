// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package profile contains all Profile related resources, data sources, and shared functionality
// for the Apple Terraform provider.
package profile

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// profileModel maps the Profile schema data for both resource and data source.
// This is the canonical representation of a Profile in Terraform state.
type profileModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Platform       types.String `tfsdk:"platform"`
	BundleID       types.String `tfsdk:"bundle_id"`
	Certificates   types.List   `tfsdk:"certificates"`
	Devices        types.List   `tfsdk:"devices"`
	ProfileContent types.String `tfsdk:"profile_content"`
	UUID           types.String `tfsdk:"uuid"`
	ProfileState   types.String `tfsdk:"profile_state"`
	ProfileType    types.String `tfsdk:"profile_type"`
	CreatedDate    types.String `tfsdk:"created_date"`
	ExpirationDate types.String `tfsdk:"expiration_date"`
}

// profilesDataSourceModel maps the data source schema for listing Profiles.
type profilesDataSourceModel struct {
	// Filter configuration
	Platform     types.String `tfsdk:"platform"`
	Platforms    types.List   `tfsdk:"platforms"`
	NamePattern  types.String `tfsdk:"name_pattern"`
	ProfileState types.String `tfsdk:"profile_state"`
	ProfileType  types.String `tfsdk:"profile_type"`
	BundleID     types.String `tfsdk:"bundle_id"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	Profiles []profileModel `tfsdk:"profiles"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// Profile platform constants and validators.
var (
	// ValidPlatforms contains all supported Profile platforms.
	ValidPlatforms = []string{"IOS", "MAC_OS", "TV_OS", "WATCH_OS"}

	// ValidProfileStates contains all supported Profile states.
	ValidProfileStates = []string{"ACTIVE", "INVALID", "EXPIRED"}

	// ValidProfileTypes contains all supported Profile types.
	ValidProfileTypes = []string{
		"IOS_APP_DEVELOPMENT",
		"IOS_APP_ADHOC",
		"IOS_APP_STORE",
		"IOS_APP_INHOUSE",
		"MAC_APP_DEVELOPMENT",
		"MAC_APP_STORE",
		"MAC_APP_DIRECT",
		"TVOS_APP_DEVELOPMENT",
		"TVOS_APP_ADHOC",
		"TVOS_APP_STORE",
		"TVOS_APP_INHOUSE",
	}

	// PlatformValidator validates platform values.
	PlatformValidator = stringvalidator.OneOf(ValidPlatforms...)

	// ProfileStateValidator validates profile state values.
	ProfileStateValidator = stringvalidator.OneOf(ValidProfileStates...)

	// ProfileTypeValidator validates profile type values.
	ProfileTypeValidator = stringvalidator.OneOf(ValidProfileTypes...)

	// SortByValidator validates sort field options.
	SortByValidator = stringvalidator.OneOf("name", "platform", "profile_state", "profile_type", "created_date", "expiration_date")

	// SortOrderValidator validates sort order options.
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetNameValidator returns validators for Profile name fields.
func GetNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 64),
	}
}

// GetPatternValidator returns validators for pattern fields (regex patterns).
func GetPatternValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 255),
		// Add regex validation to ensure valid regex pattern
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^.+$`), // Basic non-empty validation - actual regex validation happens in filters
			"Pattern must be a valid regular expression",
		),
	}
}
