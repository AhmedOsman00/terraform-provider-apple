// Package bundle contains all Bundle ID related resources, data sources, and shared functionality
// for the Apple Terraform provider.
package bundle

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// bundleIDModel maps the Bundle ID schema data for both resource and data source.
// This is the canonical representation of a Bundle ID in Terraform state.
type bundleIDModel struct {
	ID         types.String `tfsdk:"id"`
	Identifier types.String `tfsdk:"identifier"`
	Name       types.String `tfsdk:"name"`
	Platform   types.String `tfsdk:"platform"`
	SeedID     types.String `tfsdk:"seed_id"`
}

// bundleIDsDataSourceModel maps the data source schema for listing Bundle IDs.
type bundleIDsDataSourceModel struct {
	// Filter configuration
	Platform          types.String `tfsdk:"platform"`
	Platforms         types.List   `tfsdk:"platforms"`
	IdentifierPattern types.String `tfsdk:"identifier_pattern"`
	IdentifierPrefix  types.String `tfsdk:"identifier_prefix"`
	NamePattern       types.String `tfsdk:"name_pattern"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	BundleIDs []bundleIDModel `tfsdk:"bundle_ids"`

	// Computed metadata
	TotalCount    types.Int64  `tfsdk:"total_count"`
	FilteredCount types.Int64  `tfsdk:"filtered_count"`
	LastUpdated   types.String `tfsdk:"last_updated"`
}

// Bundle ID platform constants and validators
var (
	// ValidPlatforms contains all supported Bundle ID platforms
	ValidPlatforms = []string{"IOS", "MAC_OS", "TV_OS", "WATCH_OS"}

	// PlatformValidator validates platform values
	PlatformValidator = stringvalidator.OneOf(ValidPlatforms...)

	// BundleIdentifierValidator validates Bundle ID identifier format
	BundleIdentifierValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-zA-Z0-9.-]+$`),
		"Bundle identifier must follow reverse domain format (e.g., com.example.myapp)",
	)

	// SortByValidator validates sort field options
	SortByValidator = stringvalidator.OneOf("name", "identifier", "platform")

	// SortOrderValidator validates sort order options
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetIdentifierValidator returns validators for Bundle ID identifier fields
func GetIdentifierValidator() []validator.String {
	return []validator.String{
		BundleIdentifierValidator,
		stringvalidator.LengthBetween(1, 255),
	}
}

// GetNameValidator returns validators for Bundle ID name fields
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
