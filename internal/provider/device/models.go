// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package device contains all Device related resources, data sources, and shared functionality
// for the Apple Terraform provider.
package device

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// deviceModel maps the Device schema data for both resource and data source.
// This is the canonical representation of a Device in Terraform state.
type deviceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	UDID        types.String `tfsdk:"udid"`
	Platform    types.String `tfsdk:"platform"`
	DeviceClass types.String `tfsdk:"device_class"`
	Model       types.String `tfsdk:"model"`
	Status      types.String `tfsdk:"status"`
	AddedDate   types.String `tfsdk:"added_date"`
}

// devicesDataSourceModel maps the data source schema for listing Devices.
type devicesDataSourceModel struct {
	// Filter configuration
	Platform      types.String `tfsdk:"platform"`
	Platforms     types.List   `tfsdk:"platforms"`
	DeviceClass   types.String `tfsdk:"device_class"`
	DeviceClasses types.List   `tfsdk:"device_classes"`
	Status        types.String `tfsdk:"status"`
	NamePattern   types.String `tfsdk:"name_pattern"`
	UDIDPattern   types.String `tfsdk:"udid_pattern"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	Devices []deviceModel `tfsdk:"devices"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// Device platform constants and validators.
var (
	// ValidPlatforms contains all supported Device platforms.
	ValidPlatforms = []string{"IOS", "MAC_OS", "TV_OS", "VISION_OS"}

	// ValidDeviceClasses contains all supported Device classes.
	ValidDeviceClasses = []string{"IPHONE", "IPAD", "IPOD", "APPLE_TV", "APPLE_WATCH", "MAC", "APPLE_VISION_PRO"}

	// ValidStatuses contains all supported Device statuses.
	ValidStatuses = []string{"ENABLED", "PROCESSING", "INELIGIBLE"}

	// PlatformValidator validates platform values.
	PlatformValidator = stringvalidator.OneOf(ValidPlatforms...)

	// DeviceClassValidator validates device class values.
	DeviceClassValidator = stringvalidator.OneOf(ValidDeviceClasses...)

	// StatusValidator validates status values.
	StatusValidator = stringvalidator.OneOf(ValidStatuses...)

	// UDIDValidator keeps out values that cannot be a UDID at all -- a
	// placeholder, an empty string, a path -- and deliberately does not pin the
	// exact shape. Apple has shipped at least three (40 hex up to the iPhone X;
	// 8-16 hex such as 00008030-000A4D8E0AB8802E since the A12; a UUID on Mac,
	// Apple TV and Vision Pro) and owes no notice before shipping a fourth.
	//
	// Apple rejects a malformed UDID at create time with its own message, and
	// Create surfaces that as an "Invalid Device Data" diagnostic, so a stricter
	// rule here would buy a plan-time error rather than an apply-time one and
	// cost a provider release every time the guess went stale. An earlier
	// exact-shape regex did exactly that: it rejected the form most devices in
	// service report. A wrong UDID that is still well formed -- the mistake that
	// actually costs a slot in the annual device allowance -- is invisible to any
	// regex.
	UDIDValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[0-9a-fA-F-]+$`),
		"UDID must contain only hexadecimal characters and dashes",
	)

	// SortByValidator validates sort field options.
	SortByValidator = stringvalidator.OneOf("name", "udid", "platform", "device_class", "status", "added_date")

	// SortOrderValidator validates sort order options.
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetUDIDValidator returns validators for Device UDID fields. The bounds are
// loose on purpose: the shortest UDID Apple currently issues is 25 characters
// and the longest 40, so this only rules out lengths no format plausibly takes.
func GetUDIDValidator() []validator.String {
	return []validator.String{
		UDIDValidator,
		stringvalidator.LengthBetween(16, 64),
	}
}

// GetNameValidator returns validators for Device name fields.
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
