// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package certificate contains all Certificate related resources, data sources, and shared functionality
// for the Apple Terraform provider.
package certificate

import (
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// certificateModel maps the Certificate schema data for both resource and data source.
// This is the canonical representation of a Certificate in Terraform state.
type certificateModel struct {
	ID                 types.String `tfsdk:"id"`
	SerialNumber       types.String `tfsdk:"serial_number"`
	CertificateContent types.String `tfsdk:"certificate_content"`
	DisplayName        types.String `tfsdk:"display_name"`
	Name               types.String `tfsdk:"name"`
	CsrContent         types.String `tfsdk:"csr_content"`
	Platform           types.String `tfsdk:"platform"`
	ExpirationDate     types.String `tfsdk:"expiration_date"`
	CertificateType    types.String `tfsdk:"certificate_type"`
	RequesterFirstName types.String `tfsdk:"requester_first_name"`
	RequesterLastName  types.String `tfsdk:"requester_last_name"`
	RequesterEmail     types.String `tfsdk:"requester_email"`
}

// certificateResourceModel extends certificateModel with the resource-only
// attributes that drive early renewal. It is kept separate because
// certificateModel doubles as the data source's nested object, where a
// user-supplied renewal window has no meaning.
type certificateResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	SerialNumber       types.String `tfsdk:"serial_number"`
	CertificateContent types.String `tfsdk:"certificate_content"`
	DisplayName        types.String `tfsdk:"display_name"`
	Name               types.String `tfsdk:"name"`
	CsrContent         types.String `tfsdk:"csr_content"`
	Platform           types.String `tfsdk:"platform"`
	ExpirationDate     types.String `tfsdk:"expiration_date"`
	CertificateType    types.String `tfsdk:"certificate_type"`
	RequesterFirstName types.String `tfsdk:"requester_first_name"`
	RequesterLastName  types.String `tfsdk:"requester_last_name"`
	RequesterEmail     types.String `tfsdk:"requester_email"`

	// Early renewal
	EarlyRenewalHours types.Int64 `tfsdk:"early_renewal_hours"`
	ReadyForRenewal   types.Bool  `tfsdk:"ready_for_renewal"`
}

// certificatesDataSourceModel maps the data source schema for listing Certificates.
type certificatesDataSourceModel struct {
	// Filter configuration
	CertificateType  types.String `tfsdk:"certificate_type"`
	CertificateTypes types.List   `tfsdk:"certificate_types"`
	Platform         types.String `tfsdk:"platform"`
	Platforms        types.List   `tfsdk:"platforms"`
	NamePattern      types.String `tfsdk:"name_pattern"`
	SerialNumber     types.String `tfsdk:"serial_number"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	Certificates []certificateModel `tfsdk:"certificates"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// Certificate type constants and validators.
var (
	// ValidCertificateTypes contains all supported Certificate types.
	ValidCertificateTypes = []string{
		"IOS_DEVELOPMENT",
		"IOS_DISTRIBUTION",
		"MAC_APP_DEVELOPMENT",
		"MAC_APP_DISTRIBUTION",
		"MAC_INSTALLER_DISTRIBUTION",
		"DEVELOPER_ID_KEXT",
		"DEVELOPER_ID_APPLICATION",
		"DEVELOPMENT",
		"DISTRIBUTION",
		"PASS_TYPE_ID",
		"PASS_TYPE_ID_WITH_NFC",
		"DEVELOPER_ID_INSTALLER",
		"DEVELOPER_ID_APPLICATION_G2",
		"DEVELOPER_ID_INSTALLER_G2",
		"DEVELOPER_ID_KEXT_G2",
	}

	// ValidPlatforms contains all supported Certificate platforms.
	ValidPlatforms = []string{"IOS", "MAC_OS", "TV_OS", "WATCH_OS"}

	// CertificateTypeValidator validates certificate type values.
	CertificateTypeValidator = stringvalidator.OneOf(ValidCertificateTypes...)

	// PlatformValidator validates platform values.
	PlatformValidator = stringvalidator.OneOf(ValidPlatforms...)

	// SortByValidator validates sort field options.
	SortByValidator = stringvalidator.OneOf("display_name", "name", "serial_number", "certificate_type", "expiration_date")

	// SortOrderValidator validates sort order options.
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetCsrContentValidator returns validators for CSR content fields.
func GetCsrContentValidator() []validator.String {
	return []validator.String{
		stringvalidator.RegexMatches(
			regexp.MustCompile(`-----BEGIN CERTIFICATE REQUEST-----[\s\S]*-----END CERTIFICATE REQUEST-----`),
			"CSR content must be a valid PEM-encoded certificate signing request",
		),
		stringvalidator.LengthBetween(1, 8192), // Reasonable length limit for CSR
	}
}

// GetNameValidator returns validators for Certificate name fields.
func GetNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 255),
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

// GetSerialNumberValidator returns validators for serial number fields.
func GetSerialNumberValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 64),
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^[A-Fa-f0-9]+$`),
			"Serial number must be a hexadecimal string",
		),
	}
}

// readyForRenewal reports whether a certificate expiring at expirationDate has
// entered its early renewal window. A zero, negative, null or unknown window
// disables renewal, as does an expiration date Apple did not report.
func readyForRenewal(expirationDate types.String, earlyRenewalHours types.Int64, now time.Time) (bool, error) {
	if earlyRenewalHours.IsNull() || earlyRenewalHours.IsUnknown() || earlyRenewalHours.ValueInt64() <= 0 {
		return false, nil
	}

	if expirationDate.IsNull() || expirationDate.IsUnknown() {
		return false, nil
	}

	expiresAt, err := time.Parse(time.RFC3339, expirationDate.ValueString())
	if err != nil {
		return false, fmt.Errorf("could not parse expiration_date %q as RFC3339: %w", expirationDate.ValueString(), err)
	}

	renewAt := expiresAt.Add(-time.Duration(earlyRenewalHours.ValueInt64()) * time.Hour)

	return now.After(renewAt), nil
}
