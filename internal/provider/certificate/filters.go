package certificate

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// FilterOptions represents the filtering configuration for Certificates
type FilterOptions struct {
	CertificateType  string
	CertificateTypes []string
	Platform         string
	Platforms        []string
	NamePattern      string
	SerialNumber     string
	Limit            int64
	SortBy           string
	SortOrder        string
}

// FilterCertificates applies filters and sorting to a list of Certificates
func FilterCertificates(ctx context.Context, certificates []models.Certificate, options FilterOptions) ([]models.Certificate, error) {
	tflog.Debug(ctx, "Filtering Certificates", map[string]interface{}{
		"total_count":             len(certificates),
		"certificate_type":        options.CertificateType,
		"certificate_types_count": len(options.CertificateTypes),
		"platform":                options.Platform,
		"platforms_count":         len(options.Platforms),
		"name_pattern":            options.NamePattern,
		"serial_number":           options.SerialNumber,
		"limit":                   options.Limit,
		"sort_by":                 options.SortBy,
		"sort_order":              options.SortOrder,
	})

	filtered := make([]models.Certificate, 0, len(certificates))

	// Apply filters
	for _, certificate := range certificates {
		if shouldIncludeCertificate(ctx, certificate, options) {
			filtered = append(filtered, certificate)
		}
	}

	tflog.Debug(ctx, "After filtering", map[string]interface{}{
		"filtered_count": len(filtered),
	})

	// Apply sorting
	if options.SortBy != "" {
		sortCertificates(filtered, options.SortBy, options.SortOrder)
		tflog.Debug(ctx, "Applied sorting", map[string]interface{}{
			"sort_by":    options.SortBy,
			"sort_order": options.SortOrder,
		})
	}

	// Apply limit
	if options.Limit > 0 && int64(len(filtered)) > options.Limit {
		filtered = filtered[:options.Limit]
		tflog.Debug(ctx, "Applied limit", map[string]interface{}{
			"limit":       options.Limit,
			"final_count": len(filtered),
		})
	}

	return filtered, nil
}

// shouldIncludeCertificate determines if a Certificate should be included based on filters
func shouldIncludeCertificate(ctx context.Context, certificate models.Certificate, options FilterOptions) bool {
	// Certificate type filter (single)
	if options.CertificateType != "" && string(certificate.Attributes.CertificateType) != options.CertificateType {
		return false
	}

	// Certificate types filter (multiple)
	if len(options.CertificateTypes) > 0 {
		typeMatch := false
		for _, certType := range options.CertificateTypes {
			if string(certificate.Attributes.CertificateType) == certType {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			return false
		}
	}

	// Platform filter (single)
	if options.Platform != "" && certificate.Attributes.Platform != nil && string(*certificate.Attributes.Platform) != options.Platform {
		return false
	}

	// Platforms filter (multiple)
	if len(options.Platforms) > 0 && certificate.Attributes.Platform != nil {
		platformMatch := false
		for _, platform := range options.Platforms {
			if string(*certificate.Attributes.Platform) == platform {
				platformMatch = true
				break
			}
		}
		if !platformMatch {
			return false
		}
	}

	// Serial number exact match filter
	if options.SerialNumber != "" && certificate.Attributes.SerialNumber != options.SerialNumber {
		return false
	}

	// Name pattern filter (glob-style) - checks both display name and name
	if options.NamePattern != "" {
		displayNameMatched, err1 := filepath.Match(options.NamePattern, certificate.Attributes.DisplayName)
		nameMatched, err2 := filepath.Match(options.NamePattern, certificate.Attributes.Name)

		if err1 != nil && err2 != nil {
			tflog.Warn(ctx, "Invalid glob pattern for name", map[string]interface{}{
				"pattern": options.NamePattern,
				"error1":  err1.Error(),
				"error2":  err2.Error(),
			})
			return false
		}

		// Include if either display name or name matches
		if !displayNameMatched && !nameMatched {
			return false
		}
	}

	return true
}

// sortCertificates sorts Certificates by the specified field and order
func sortCertificates(certificates []models.Certificate, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(certificates, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "display_name":
			less = strings.ToLower(certificates[i].Attributes.DisplayName) < strings.ToLower(certificates[j].Attributes.DisplayName)
		case "name":
			less = strings.ToLower(certificates[i].Attributes.Name) < strings.ToLower(certificates[j].Attributes.Name)
		case "serial_number":
			less = certificates[i].Attributes.SerialNumber < certificates[j].Attributes.SerialNumber
		case "certificate_type":
			less = string(certificates[i].Attributes.CertificateType) < string(certificates[j].Attributes.CertificateType)
		case "expiration_date":
			// Handle nil expiration dates
			date1 := certificates[i].Attributes.ExpirationDate
			date2 := certificates[j].Attributes.ExpirationDate

			if date1 == nil && date2 == nil {
				less = false // Equal
			} else if date1 == nil {
				less = false // Nil dates go last
			} else if date2 == nil {
				less = true // Nil dates go last
			} else {
				less = date1.Before(*date2)
			}
		default:
			// Default to display name sorting
			less = strings.ToLower(certificates[i].Attributes.DisplayName) < strings.ToLower(certificates[j].Attributes.DisplayName)
		}

		if ascending {
			return less
		}
		return !less
	})
}

// ParseFilterOptions extracts filter options from the data source model
func ParseFilterOptions(ctx context.Context, config certificatesDataSourceModel) (FilterOptions, error) {
	options := FilterOptions{}

	// Single certificate type filter
	if !config.CertificateType.IsNull() && !config.CertificateType.IsUnknown() {
		options.CertificateType = config.CertificateType.ValueString()
	}

	// Multiple certificate types filter
	if !config.CertificateTypes.IsNull() && !config.CertificateTypes.IsUnknown() {
		var certificateTypes []string
		diags := config.CertificateTypes.ElementsAs(ctx, &certificateTypes, false)
		if diags.HasError() {
			tflog.Error(ctx, "Failed to parse certificate types filter", map[string]interface{}{
				"diagnostics": diags.Errors(),
			})
			return options, nil // Continue with other filters
		}
		options.CertificateTypes = certificateTypes
	}

	// Single platform filter
	if !config.Platform.IsNull() && !config.Platform.IsUnknown() {
		options.Platform = config.Platform.ValueString()
	}

	// Multiple platforms filter
	if !config.Platforms.IsNull() && !config.Platforms.IsUnknown() {
		var platforms []string
		diags := config.Platforms.ElementsAs(ctx, &platforms, false)
		if diags.HasError() {
			tflog.Error(ctx, "Failed to parse platforms filter", map[string]interface{}{
				"diagnostics": diags.Errors(),
			})
			return options, nil // Continue with other filters
		}
		options.Platforms = platforms
	}

	// Name pattern filter
	if !config.NamePattern.IsNull() && !config.NamePattern.IsUnknown() {
		options.NamePattern = config.NamePattern.ValueString()
	}

	// Serial number filter
	if !config.SerialNumber.IsNull() && !config.SerialNumber.IsUnknown() {
		options.SerialNumber = config.SerialNumber.ValueString()
	}

	// Limit
	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		options.Limit = config.Limit.ValueInt64()
	}

	// Sort by
	if !config.SortBy.IsNull() && !config.SortBy.IsUnknown() {
		options.SortBy = config.SortBy.ValueString()
	}

	// Sort order
	if !config.SortOrder.IsNull() && !config.SortOrder.IsUnknown() {
		options.SortOrder = config.SortOrder.ValueString()
	} else if options.SortBy != "" {
		options.SortOrder = "asc" // Default to ascending
	}

	tflog.Debug(ctx, "Parsed filter options", map[string]interface{}{
		"certificate_type":        options.CertificateType,
		"certificate_types_count": len(options.CertificateTypes),
		"platform":                options.Platform,
		"platforms_count":         len(options.Platforms),
		"name_pattern":            options.NamePattern,
		"serial_number":           options.SerialNumber,
		"limit":                   options.Limit,
		"sort_by":                 options.SortBy,
		"sort_order":              options.SortOrder,
	})

	return options, nil
}
