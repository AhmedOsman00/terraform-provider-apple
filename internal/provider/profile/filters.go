package profile

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"terraform-provider-apple/internal/apple/models"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// applyFilters applies filters to a list of Profiles.
func (d *profilesDataSource) applyFilters(ctx context.Context, profiles []models.Profile, config profilesDataSourceModel) ([]models.Profile, error) {
	// Reject a malformed pattern up front rather than silently matching nothing.
	if !config.NamePattern.IsNull() && !config.NamePattern.IsUnknown() {
		if _, err := regexp.Compile(config.NamePattern.ValueString()); err != nil {
			return nil, fmt.Errorf("invalid name_pattern %q: %w", config.NamePattern.ValueString(), err)
		}
	}

	tflog.Debug(ctx, "Filtering Profiles", map[string]interface{}{
		"total_count": len(profiles),
	})

	filtered := make([]models.Profile, 0, len(profiles))

	// Apply filters
	for _, profile := range profiles {
		if d.shouldIncludeProfile(ctx, profile, config) {
			filtered = append(filtered, profile)
		}
	}

	tflog.Debug(ctx, "After filtering", map[string]interface{}{
		"filtered_count": len(filtered),
	})

	return filtered, nil
}

// shouldIncludeProfile checks if a profile should be included based on filter criteria.
func (d *profilesDataSource) shouldIncludeProfile(ctx context.Context, profile models.Profile, config profilesDataSourceModel) bool {
	// Platform filter (single)
	if !config.Platform.IsNull() && !config.Platform.IsUnknown() {
		if string(profile.Attributes.Platform) != config.Platform.ValueString() {
			return false
		}
	}

	// Platforms filter (multiple)
	if !config.Platforms.IsNull() && !config.Platforms.IsUnknown() {
		var platformElements []types.String
		config.Platforms.ElementsAs(ctx, &platformElements, false)

		platformMatch := false
		for _, platformElement := range platformElements {
			if string(profile.Attributes.Platform) == platformElement.ValueString() {
				platformMatch = true
				break
			}
		}
		if !platformMatch {
			return false
		}
	}

	// Name pattern filter
	if !config.NamePattern.IsNull() && !config.NamePattern.IsUnknown() {
		pattern := config.NamePattern.ValueString()
		matched, err := regexp.MatchString(pattern, profile.Attributes.Name)
		if err != nil {
			tflog.Warn(ctx, "Invalid regex pattern for name filter", map[string]interface{}{
				"pattern": pattern,
				"error":   err.Error(),
			})
			return false
		}
		if !matched {
			return false
		}
	}

	// Profile state filter
	if !config.ProfileState.IsNull() && !config.ProfileState.IsUnknown() {
		if profile.Attributes.ProfileState == nil || string(*profile.Attributes.ProfileState) != config.ProfileState.ValueString() {
			return false
		}
	}

	// Profile type filter
	if !config.ProfileType.IsNull() && !config.ProfileType.IsUnknown() {
		if profile.Attributes.ProfileType == nil || string(*profile.Attributes.ProfileType) != config.ProfileType.ValueString() {
			return false
		}
	}

	// Bundle ID filter (would require relationship data)
	if !config.BundleID.IsNull() && !config.BundleID.IsUnknown() {
		// This would require fetching relationship data from the API
		// For now, we'll skip this filter or implement it when relationship data is available
		tflog.Debug(ctx, "Bundle ID filter not yet implemented due to API relationship complexity")
	}

	return true
}

// applySorting sorts profiles based on the sort configuration.
func (d *profilesDataSource) applySorting(ctx context.Context, profiles []models.Profile, config profilesDataSourceModel) []models.Profile {
	if len(profiles) == 0 {
		return profiles
	}

	sortBy := "name" // default
	if !config.SortBy.IsNull() && !config.SortBy.IsUnknown() {
		sortBy = config.SortBy.ValueString()
	}

	sortOrder := "asc" // default
	if !config.SortOrder.IsNull() && !config.SortOrder.IsUnknown() {
		sortOrder = config.SortOrder.ValueString()
	}

	tflog.Debug(ctx, "Sorting profiles", map[string]interface{}{
		"sort_by":    sortBy,
		"sort_order": sortOrder,
	})

	// Create a copy to avoid modifying the original slice
	sorted := make([]models.Profile, len(profiles))
	copy(sorted, profiles)

	sort.Slice(sorted, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "name":
			less = strings.ToLower(sorted[i].Attributes.Name) < strings.ToLower(sorted[j].Attributes.Name)
		case "platform":
			less = string(sorted[i].Attributes.Platform) < string(sorted[j].Attributes.Platform)
		case "profile_state":
			stateI := ""
			stateJ := ""
			if sorted[i].Attributes.ProfileState != nil {
				stateI = string(*sorted[i].Attributes.ProfileState)
			}
			if sorted[j].Attributes.ProfileState != nil {
				stateJ = string(*sorted[j].Attributes.ProfileState)
			}
			less = stateI < stateJ
		case "profile_type":
			typeI := ""
			typeJ := ""
			if sorted[i].Attributes.ProfileType != nil {
				typeI = string(*sorted[i].Attributes.ProfileType)
			}
			if sorted[j].Attributes.ProfileType != nil {
				typeJ = string(*sorted[j].Attributes.ProfileType)
			}
			less = typeI < typeJ
		case "created_date":
			dateI := time.Time{}
			dateJ := time.Time{}
			if sorted[i].Attributes.CreatedDate != nil {
				dateI = *sorted[i].Attributes.CreatedDate
			}
			if sorted[j].Attributes.CreatedDate != nil {
				dateJ = *sorted[j].Attributes.CreatedDate
			}
			less = dateI.Before(dateJ)
		case "expiration_date":
			dateI := time.Time{}
			dateJ := time.Time{}
			if sorted[i].Attributes.ExpirationDate != nil {
				dateI = *sorted[i].Attributes.ExpirationDate
			}
			if sorted[j].Attributes.ExpirationDate != nil {
				dateJ = *sorted[j].Attributes.ExpirationDate
			}
			less = dateI.Before(dateJ)
		default:
			// Default to name sorting
			less = strings.ToLower(sorted[i].Attributes.Name) < strings.ToLower(sorted[j].Attributes.Name)
		}

		if sortOrder == "desc" {
			less = !less
		}

		return less
	})

	return sorted
}

// applyLimit applies the limit to the profile results.
func (d *profilesDataSource) applyLimit(ctx context.Context, profiles []models.Profile, config profilesDataSourceModel) []models.Profile {
	if config.Limit.IsNull() || config.Limit.IsUnknown() {
		return profiles
	}

	limit := int(config.Limit.ValueInt64())
	if limit <= 0 || limit >= len(profiles) {
		return profiles
	}

	tflog.Debug(ctx, "Applying limit to profiles", map[string]interface{}{
		"original_count": len(profiles),
		"limit":          limit,
	})

	return profiles[:limit]
}
