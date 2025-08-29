package merchant

import (
	"regexp"
	"sort"
	"strings"
	"terraform-provider-apple/internal/apple/models"
)

// MerchantIDFilters represents the filtering configuration for Merchant IDs
type MerchantIDFilters struct {
	IdentifierPattern  string
	IdentifierPrefix   string
	DisplayNamePattern string
}

// FilterMerchantIDs applies filters to a list of Merchant IDs
func FilterMerchantIDs(merchantIDs []models.MerchantID, filters MerchantIDFilters) []models.MerchantID {
	filtered := make([]models.MerchantID, 0, len(merchantIDs))

	// Apply filters
	for _, merchantID := range merchantIDs {
		if shouldIncludeMerchantID(merchantID, filters) {
			filtered = append(filtered, merchantID)
		}
	}

	return filtered
}

// shouldIncludeMerchantID determines if a Merchant ID should be included based on filters
func shouldIncludeMerchantID(merchantID models.MerchantID, filters MerchantIDFilters) bool {
	// Identifier pattern filter (regex)
	if filters.IdentifierPattern != "" {
		matched, err := regexp.MatchString(filters.IdentifierPattern, merchantID.Attributes.Identifier)
		if err != nil {
			// Invalid regex pattern - exclude this item
			return false
		}
		if !matched {
			return false
		}
	}

	// Identifier prefix filter
	if filters.IdentifierPrefix != "" && !strings.HasPrefix(merchantID.Attributes.Identifier, filters.IdentifierPrefix) {
		return false
	}

	// Display name pattern filter (case-insensitive regex)
	if filters.DisplayNamePattern != "" {
		matched, err := regexp.MatchString("(?i)"+filters.DisplayNamePattern, merchantID.Attributes.DisplayName)
		if err != nil {
			// Invalid regex pattern - exclude this item
			return false
		}
		if !matched {
			return false
		}
	}

	return true
}

// SortMerchantIDs sorts Merchant IDs by the specified field and order
func SortMerchantIDs(merchantIDs []models.MerchantID, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(merchantIDs, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "display_name":
			less = strings.ToLower(merchantIDs[i].Attributes.DisplayName) < strings.ToLower(merchantIDs[j].Attributes.DisplayName)
		case "identifier":
			less = merchantIDs[i].Attributes.Identifier < merchantIDs[j].Attributes.Identifier
		default:
			// Default to identifier sorting
			less = merchantIDs[i].Attributes.Identifier < merchantIDs[j].Attributes.Identifier
		}

		if ascending {
			return less
		}
		return !less
	})
}
