package bundle

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// FilterOptions represents the filtering configuration for Bundle IDs.
type FilterOptions struct {
	Platform          string
	Platforms         []string
	IdentifierPattern string
	IdentifierPrefix  string
	NamePattern       string
	Limit             int64
	SortBy            string
	SortOrder         string
}

// FilterBundleIDs applies filters and sorting to a list of Bundle IDs.
func FilterBundleIDs(ctx context.Context, bundleIDs []models.BundleID, options FilterOptions) ([]models.BundleID, error) {
	tflog.Debug(ctx, "Filtering Bundle IDs", map[string]interface{}{
		"total_count":        len(bundleIDs),
		"platform":           options.Platform,
		"platforms_count":    len(options.Platforms),
		"identifier_pattern": options.IdentifierPattern,
		"identifier_prefix":  options.IdentifierPrefix,
		"name_pattern":       options.NamePattern,
		"limit":              options.Limit,
		"sort_by":            options.SortBy,
		"sort_order":         options.SortOrder,
	})

	// Reject malformed patterns up front. Left to the per-item match calls, a
	// typo silently matches nothing and the data source returns an empty list
	// instead of reporting the bad pattern.
	if options.IdentifierPattern != "" {
		if _, err := regexp.Compile(options.IdentifierPattern); err != nil {
			return nil, fmt.Errorf("invalid identifier_pattern %q: %w", options.IdentifierPattern, err)
		}
	}
	if options.NamePattern != "" {
		if _, err := filepath.Match(options.NamePattern, ""); err != nil {
			return nil, fmt.Errorf("invalid name_pattern %q: %w", options.NamePattern, err)
		}
	}

	filtered := make([]models.BundleID, 0, len(bundleIDs))

	// Apply filters
	for _, bundleID := range bundleIDs {
		if shouldIncludeBundleID(ctx, bundleID, options) {
			filtered = append(filtered, bundleID)
		}
	}

	tflog.Debug(ctx, "After filtering", map[string]interface{}{
		"filtered_count": len(filtered),
	})

	// Apply sorting
	if options.SortBy != "" {
		sortBundleIDs(filtered, options.SortBy, options.SortOrder)
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

// shouldIncludeBundleID determines if a Bundle ID should be included based on filters.
func shouldIncludeBundleID(ctx context.Context, bundleID models.BundleID, options FilterOptions) bool {
	// Platform filter (single)
	if options.Platform != "" && string(bundleID.Attributes.Platform) != options.Platform {
		return false
	}

	// Platforms filter (multiple)
	if len(options.Platforms) > 0 {
		platformMatch := false
		for _, platform := range options.Platforms {
			if string(bundleID.Attributes.Platform) == platform {
				platformMatch = true
				break
			}
		}
		if !platformMatch {
			return false
		}
	}

	// Identifier pattern filter (regex)
	if options.IdentifierPattern != "" {
		matched, err := regexp.MatchString(options.IdentifierPattern, bundleID.Attributes.Identifier)
		if err != nil {
			tflog.Warn(ctx, "Invalid regex pattern for identifier", map[string]interface{}{
				"pattern": options.IdentifierPattern,
				"error":   err.Error(),
			})
			return false
		}
		if !matched {
			return false
		}
	}

	// Identifier prefix filter
	if options.IdentifierPrefix != "" && !strings.HasPrefix(bundleID.Attributes.Identifier, options.IdentifierPrefix) {
		return false
	}

	// Name pattern filter (glob-style)
	if options.NamePattern != "" {
		matched, err := filepath.Match(options.NamePattern, bundleID.Attributes.Name)
		if err != nil {
			tflog.Warn(ctx, "Invalid glob pattern for name", map[string]interface{}{
				"pattern": options.NamePattern,
				"error":   err.Error(),
			})
			return false
		}
		if !matched {
			return false
		}
	}

	return true
}

// sortBundleIDs sorts Bundle IDs by the specified field and order.
func sortBundleIDs(bundleIDs []models.BundleID, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(bundleIDs, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "name":
			less = strings.ToLower(bundleIDs[i].Attributes.Name) < strings.ToLower(bundleIDs[j].Attributes.Name)
		case "identifier":
			less = bundleIDs[i].Attributes.Identifier < bundleIDs[j].Attributes.Identifier
		case "platform":
			less = string(bundleIDs[i].Attributes.Platform) < string(bundleIDs[j].Attributes.Platform)
		default:
			// Default to identifier sorting
			less = bundleIDs[i].Attributes.Identifier < bundleIDs[j].Attributes.Identifier
		}

		if ascending {
			return less
		}
		return !less
	})
}

// ParseFilterOptions extracts filter options from the data source model.
func ParseFilterOptions(ctx context.Context, config bundleIDsDataSourceModel) (FilterOptions, error) {
	options := FilterOptions{}

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

	// Identifier pattern filter
	if !config.IdentifierPattern.IsNull() && !config.IdentifierPattern.IsUnknown() {
		options.IdentifierPattern = config.IdentifierPattern.ValueString()
	}

	// Identifier prefix filter
	if !config.IdentifierPrefix.IsNull() && !config.IdentifierPrefix.IsUnknown() {
		options.IdentifierPrefix = config.IdentifierPrefix.ValueString()
	}

	// Name pattern filter
	if !config.NamePattern.IsNull() && !config.NamePattern.IsUnknown() {
		options.NamePattern = config.NamePattern.ValueString()
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
		"platform":           options.Platform,
		"platforms_count":    len(options.Platforms),
		"identifier_pattern": options.IdentifierPattern,
		"identifier_prefix":  options.IdentifierPrefix,
		"name_pattern":       options.NamePattern,
		"limit":              options.Limit,
		"sort_by":            options.SortBy,
		"sort_order":         options.SortOrder,
	})

	return options, nil
}
