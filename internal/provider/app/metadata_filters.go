// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// AppCategoryFilters represents the in-memory filtering configuration for the
// category catalogue.
//
// Platforms are absent: Apple publishes filter[platforms] for this collection,
// so that one is applied server-side and never reaches here.
type AppCategoryFilters struct {
	TopLevelOnly bool
	IDPattern    string
}

// FilterAppCategories applies filters to Apple's category catalogue.
//
// id_pattern is a regular expression matched as a substring unless anchored,
// following the convention the device, merchant and profile filters use rather
// than the glob the bundle and certificate ones do.
func FilterAppCategories(categories []models.AppCategory, filters AppCategoryFilters) ([]models.AppCategory, error) {
	var pattern *regexp.Regexp
	if filters.IDPattern != "" {
		compiled, err := regexp.Compile(filters.IDPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid id_pattern %q: %w", filters.IDPattern, err)
		}
		pattern = compiled
	}

	filtered := make([]models.AppCategory, 0, len(categories))
	for _, category := range categories {
		// A subcategory is the one with a parent. Apple returns both in the
		// same collection, and only a top-level category can be assigned as an
		// app's primary or secondary category.
		if filters.TopLevelOnly && category.Relationships != nil && category.Relationships.Parent != nil &&
			category.Relationships.Parent.Data.ID != "" {
			continue
		}
		if pattern != nil && !pattern.MatchString(category.ID) {
			continue
		}
		filtered = append(filtered, category)
	}

	return filtered, nil
}

// SortAppCategories orders categories by ID.
//
// There is no sort_by on the category data source: a category has nothing else
// to sort by, and Apple returns the catalogue in an order of its own that is
// not stable enough to preserve.
func SortAppCategories(categories []models.AppCategory) {
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].ID < categories[j].ID
	})
}

// AppPricePointFilters represents the in-memory filtering configuration for the
// app price catalogue.
//
// Territories are absent for the same reason as above: filter[territory] is
// applied by Apple.
type AppPricePointFilters struct {
	CustomerPrice string
}

// FilterAppPricePoints applies filters to Apple's app price catalogue.
//
// customer_price is an exact string match, not a numeric comparison: Apple
// reports prices as decimal strings, and "9.99" and "9.990" are different
// records rather than the same number written twice.
func FilterAppPricePoints(points []models.AppPricePoint, filters AppPricePointFilters) []models.AppPricePoint {
	if filters.CustomerPrice == "" {
		return points
	}

	filtered := make([]models.AppPricePoint, 0, len(points))
	for _, point := range points {
		if point.Attributes.CustomerPrice == filters.CustomerPrice {
			filtered = append(filtered, point)
		}
	}

	return filtered
}

// AppStoreVersionFilters represents the in-memory filtering configuration for
// the version listing.
//
// Platform is absent: Apple publishes filter[platform], so it is applied
// server-side.
type AppStoreVersionFilters struct {
	AppVersionState      string
	VersionStringPattern string
}

// FilterAppStoreVersions applies filters to a list of versions.
//
// app_version_state is an exact match; version_string_pattern is a regular
// expression matched as a substring unless anchored.
func FilterAppStoreVersions(versions []models.AppStoreVersion, filters AppStoreVersionFilters) ([]models.AppStoreVersion, error) {
	var pattern *regexp.Regexp
	if filters.VersionStringPattern != "" {
		compiled, err := regexp.Compile(filters.VersionStringPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid version_string_pattern %q: %w", filters.VersionStringPattern, err)
		}
		pattern = compiled
	}

	filtered := make([]models.AppStoreVersion, 0, len(versions))
	for _, version := range versions {
		if filters.AppVersionState != "" {
			if version.Attributes.AppVersionState == nil ||
				!strings.EqualFold(*version.Attributes.AppVersionState, filters.AppVersionState) {
				continue
			}
		}
		if pattern != nil && !pattern.MatchString(version.Attributes.VersionString) {
			continue
		}
		filtered = append(filtered, version)
	}

	return filtered, nil
}

// SortAppStoreVersions orders versions by the requested field.
//
// created_date sorts newest first under "desc", which is what a caller asking
// for the latest version wants. version_string is compared as a string rather
// than parsed: "1.10" sorting before "1.9" is wrong arithmetically but matches
// what every other string sort in this provider does, and a caller who cares
// about release order should sort by created_date.
func SortAppStoreVersions(versions []models.AppStoreVersion, sortBy, sortOrder string) {
	if sortBy == "" {
		return
	}

	descending := strings.EqualFold(sortOrder, "desc")

	sort.SliceStable(versions, func(i, j int) bool {
		var left, right string

		switch sortBy {
		case "version_string":
			left, right = versions[i].Attributes.VersionString, versions[j].Attributes.VersionString
		case "created_date":
			left = derefString(versions[i].Attributes.CreatedDate)
			right = derefString(versions[j].Attributes.CreatedDate)
		case "app_version_state":
			left = derefString(versions[i].Attributes.AppVersionState)
			right = derefString(versions[j].Attributes.AppVersionState)
		case "platform":
			left, right = versions[i].Attributes.Platform, versions[j].Attributes.Platform
		default:
			return false
		}

		if descending {
			return left > right
		}

		return left < right
	})
}

// LimitAppStoreVersions truncates the list to at most limit entries.
func LimitAppStoreVersions(versions []models.AppStoreVersion, limit int) []models.AppStoreVersion {
	if limit > 0 && limit < len(versions) {
		return versions[:limit]
	}

	return versions
}

// derefString reads an optional string attribute, treating an absent value as
// empty so it sorts consistently rather than panicking.
func derefString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
