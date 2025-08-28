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
	TotalCount    types.Int64  `tfsdk:"total_count"`
	FilteredCount types.Int64  `tfsdk:"filtered_count"`
	LastUpdated   types.String `tfsdk:"last_updated"`
}

// Device platform constants and validators
var (
	// ValidPlatforms contains all supported Device platforms
	ValidPlatforms = []string{"IOS", "MAC_OS", "TV_OS", "VISION_OS"}

	// ValidDeviceClasses contains all supported Device classes
	ValidDeviceClasses = []string{"IPHONE", "IPAD", "IPOD", "APPLE_TV", "APPLE_WATCH", "MAC", "APPLE_VISION_PRO"}

	// ValidStatuses contains all supported Device statuses
	ValidStatuses = []string{"ENABLED", "PROCESSING", "INELIGIBLE"}

	// PlatformValidator validates platform values
	PlatformValidator = stringvalidator.OneOf(ValidPlatforms...)

	// DeviceClassValidator validates device class values
	DeviceClassValidator = stringvalidator.OneOf(ValidDeviceClasses...)

	// StatusValidator validates status values
	StatusValidator = stringvalidator.OneOf(ValidStatuses...)

	// UDIDValidator validates UDID format (40-character hex string for iOS devices, 8-4-4-4-12 format for others)
	UDIDValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^([0-9a-fA-F]{40}|[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$`),
		"UDID must be a 40-character hex string (iOS devices) or UUID format (other devices)",
	)

	// SortByValidator validates sort field options
	SortByValidator = stringvalidator.OneOf("name", "udid", "platform", "device_class", "status", "added_date")

	// SortOrderValidator validates sort order options
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetUDIDValidator returns validators for Device UDID fields
func GetUDIDValidator() []validator.String {
	return []validator.String{
		UDIDValidator,
		stringvalidator.LengthBetween(1, 255),
	}
}

// GetNameValidator returns validators for Device name fields
func GetNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 64),
	}
}

// GetPatternValidator returns validators for pattern fields (regex patterns)
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
