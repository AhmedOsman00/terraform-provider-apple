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

// AppFilters represents the filtering configuration for apps.
type AppFilters struct {
	BundleID    string
	NamePattern string
	SKU         string
}

// FilterApps applies filters to a list of apps.
//
// bundle_id and sku are exact matches; name_pattern is a regular expression
// matched as a substring unless anchored.
func FilterApps(apps []models.App, filters AppFilters) ([]models.App, error) {
	if filters.NamePattern != "" {
		if _, err := regexp.Compile(filters.NamePattern); err != nil {
			return nil, fmt.Errorf("invalid name_pattern %q: %w", filters.NamePattern, err)
		}
	}

	filtered := make([]models.App, 0, len(apps))
	for _, app := range apps {
		if shouldIncludeApp(app, filters) {
			filtered = append(filtered, app)
		}
	}

	return filtered, nil
}

// shouldIncludeApp determines whether one app passes the filters.
func shouldIncludeApp(app models.App, filters AppFilters) bool {
	if filters.BundleID != "" && app.Attributes.BundleID != filters.BundleID {
		return false
	}

	if filters.SKU != "" && app.Attributes.SKU != filters.SKU {
		return false
	}

	if filters.NamePattern != "" {
		matched, err := regexp.MatchString(filters.NamePattern, app.Attributes.Name)
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// SortApps sorts apps in place.
func SortApps(apps []models.App, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(apps, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "bundle_id":
			less = apps[i].Attributes.BundleID < apps[j].Attributes.BundleID
		case "sku":
			less = apps[i].Attributes.SKU < apps[j].Attributes.SKU
		case "name":
			less = strings.ToLower(apps[i].Attributes.Name) < strings.ToLower(apps[j].Attributes.Name)
		default:
			less = strings.ToLower(apps[i].Attributes.Name) < strings.ToLower(apps[j].Attributes.Name)
		}

		if ascending {
			return less
		}
		return !less
	})
}
