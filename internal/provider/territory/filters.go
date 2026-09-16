// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package territory

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// TerritoryFilters represents the filtering configuration for territories.
type TerritoryFilters struct {
	Currency  string
	IDPattern string
}

// FilterTerritories applies filters to a list of territories.
//
// currency is an exact match, compared case-insensitively because Apple's codes
// are uppercase and a configuration written in lowercase is a typo rather than a
// different intent. id_pattern is a regular expression matched as a substring
// unless anchored, following the regex-matching packages rather than the glob
// ones.
func FilterTerritories(territories []models.Territory, filters TerritoryFilters) ([]models.Territory, error) {
	if filters.IDPattern != "" {
		if _, err := regexp.Compile(filters.IDPattern); err != nil {
			return nil, fmt.Errorf("invalid id_pattern %q: %w", filters.IDPattern, err)
		}
	}

	filtered := make([]models.Territory, 0, len(territories))
	for _, territory := range territories {
		if shouldIncludeTerritory(territory, filters) {
			filtered = append(filtered, territory)
		}
	}

	return filtered, nil
}

// shouldIncludeTerritory determines whether one territory passes the filters.
func shouldIncludeTerritory(territory models.Territory, filters TerritoryFilters) bool {
	if filters.Currency != "" && !strings.EqualFold(territory.Attributes.Currency, filters.Currency) {
		return false
	}

	if filters.IDPattern != "" {
		matched, err := regexp.MatchString(filters.IDPattern, territory.ID)
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// SortTerritories sorts territories in place.
//
// The default is by ID, which is also the order an availability list reads best
// in. Apple returns the collection in no documented order, so sorting is what
// keeps a plan from churning when it reshuffles.
func SortTerritories(territories []models.Territory, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(territories, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "currency":
			if territories[i].Attributes.Currency != territories[j].Attributes.Currency {
				less = territories[i].Attributes.Currency < territories[j].Attributes.Currency
			} else {
				less = territories[i].ID < territories[j].ID
			}
		default:
			less = territories[i].ID < territories[j].ID
		}

		if ascending {
			return less
		}
		return !less
	})
}
