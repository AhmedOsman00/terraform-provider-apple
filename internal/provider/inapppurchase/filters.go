// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package inapppurchase

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// InAppPurchaseFilters represents the filtering configuration for in-app
// purchases.
type InAppPurchaseFilters struct {
	NamePattern       string
	ProductIDPattern  string
	InAppPurchaseType string
	State             string
}

// FilterInAppPurchases applies filters to a list of in-app purchases.
//
// Patterns are regular expressions matched as substrings unless anchored, which
// is the convention the device, merchant, passtypeid, profile and subscription
// packages follow. The bundle and certificate packages use globs instead.
func FilterInAppPurchases(purchases []models.InAppPurchase, filters InAppPurchaseFilters) ([]models.InAppPurchase, error) {
	if filters.NamePattern != "" {
		if _, err := regexp.Compile(filters.NamePattern); err != nil {
			return nil, fmt.Errorf("invalid name_pattern %q: %w", filters.NamePattern, err)
		}
	}
	if filters.ProductIDPattern != "" {
		if _, err := regexp.Compile(filters.ProductIDPattern); err != nil {
			return nil, fmt.Errorf("invalid product_id_pattern %q: %w", filters.ProductIDPattern, err)
		}
	}

	filtered := make([]models.InAppPurchase, 0, len(purchases))
	for _, purchase := range purchases {
		if shouldIncludeInAppPurchase(purchase, filters) {
			filtered = append(filtered, purchase)
		}
	}

	return filtered, nil
}

// shouldIncludeInAppPurchase determines whether one purchase passes filters.
func shouldIncludeInAppPurchase(purchase models.InAppPurchase, filters InAppPurchaseFilters) bool {
	if filters.NamePattern != "" {
		matched, err := regexp.MatchString(filters.NamePattern, purchase.Attributes.Name)
		if err != nil || !matched {
			return false
		}
	}

	if filters.ProductIDPattern != "" {
		matched, err := regexp.MatchString(filters.ProductIDPattern, purchase.Attributes.ProductID)
		if err != nil || !matched {
			return false
		}
	}

	// Type and state are exact matches on Apple's enums, not patterns. A
	// purchase Apple reported without the attribute cannot match a filter that
	// asks for one.
	if filters.InAppPurchaseType != "" {
		if purchase.Attributes.InAppPurchaseType == nil || string(*purchase.Attributes.InAppPurchaseType) != filters.InAppPurchaseType {
			return false
		}
	}

	if filters.State != "" {
		if purchase.Attributes.State == nil || string(*purchase.Attributes.State) != filters.State {
			return false
		}
	}

	return true
}

// SortInAppPurchases sorts in-app purchases in place.
func SortInAppPurchases(purchases []models.InAppPurchase, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(purchases, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "name":
			less = strings.ToLower(purchases[i].Attributes.Name) < strings.ToLower(purchases[j].Attributes.Name)
		case "state":
			less = purchaseStateOf(purchases[i]) < purchaseStateOf(purchases[j])
		case "in_app_purchase_type":
			less = purchaseTypeOf(purchases[i]) < purchaseTypeOf(purchases[j])
		case "product_id":
			less = purchases[i].Attributes.ProductID < purchases[j].Attributes.ProductID
		default:
			less = purchases[i].Attributes.ProductID < purchases[j].Attributes.ProductID
		}

		if ascending {
			return less
		}
		return !less
	})
}

// purchaseStateOf reads a state for sorting, treating a missing one as empty so
// it sorts consistently rather than panicking.
func purchaseStateOf(purchase models.InAppPurchase) string {
	if purchase.Attributes.State == nil {
		return ""
	}
	return string(*purchase.Attributes.State)
}

// purchaseTypeOf reads a type for sorting, treating a missing one as empty.
func purchaseTypeOf(purchase models.InAppPurchase) string {
	if purchase.Attributes.InAppPurchaseType == nil {
		return ""
	}
	return string(*purchase.Attributes.InAppPurchaseType)
}

// InAppPurchasePricePointFilters represents the in-memory filtering applied to
// price points after Apple has narrowed them by territory.
type InAppPurchasePricePointFilters struct {
	CustomerPrice string
}

// FilterInAppPurchasePricePoints applies filters to a list of price points.
//
// The territory filter is deliberately absent: it is applied server-side,
// because the unfiltered catalogue spans every territory the App Store sells in.
func FilterInAppPurchasePricePoints(points []models.InAppPurchasePricePoint, filters InAppPurchasePricePointFilters) []models.InAppPurchasePricePoint {
	if filters.CustomerPrice == "" {
		return points
	}

	filtered := make([]models.InAppPurchasePricePoint, 0, len(points))
	for _, point := range points {
		if point.Attributes.CustomerPrice == filters.CustomerPrice {
			filtered = append(filtered, point)
		}
	}

	return filtered
}
