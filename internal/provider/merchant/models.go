// Package merchant contains all Merchant ID related resources, data sources, and shared functionality
// for the Apple Terraform provider.
package merchant

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// merchantIDModel maps the Merchant ID schema data for both resource and data source.
// This is the canonical representation of a Merchant ID in Terraform state.
type merchantIDModel struct {
	ID          types.String `tfsdk:"id"`
	Identifier  types.String `tfsdk:"identifier"`
	DisplayName types.String `tfsdk:"display_name"`
}

// merchantIDsDataSourceModel maps the data source schema for listing Merchant IDs.
type merchantIDsDataSourceModel struct {
	// Filter configuration
	IdentifierPattern  types.String `tfsdk:"identifier_pattern"`
	IdentifierPrefix   types.String `tfsdk:"identifier_prefix"`
	DisplayNamePattern types.String `tfsdk:"display_name_pattern"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	MerchantIDs []merchantIDModel `tfsdk:"merchant_ids"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// Merchant ID identifier constants and validators.
var (
	// MerchantIdentifierValidator validates Merchant ID identifier format.
	MerchantIdentifierValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^merchant\.[a-zA-Z0-9.-]+\.[a-zA-Z0-9.-]+$`),
		"Merchant identifier must follow format merchant.domain.identifier (e.g., merchant.example.com.myapp)",
	)

	// SortByValidator validates sort field options.
	SortByValidator = stringvalidator.OneOf("display_name", "identifier")

	// SortOrderValidator validates sort order options.
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetIdentifierValidator returns validators for Merchant ID identifier fields.
func GetIdentifierValidator() []validator.String {
	return []validator.String{
		MerchantIdentifierValidator,
		stringvalidator.LengthBetween(1, 255),
	}
}

// GetDisplayNameValidator returns validators for Merchant ID display name fields.
func GetDisplayNameValidator() []validator.String {
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
