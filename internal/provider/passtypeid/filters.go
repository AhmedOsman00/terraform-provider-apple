package passtypeid

import (
	"regexp"
	"sort"
	"strings"
	"terraform-provider-apple/internal/apple/models"
)

// FilterPassTypeIDs applies filtering to a list of Pass Type IDs based on the provided criteria
func FilterPassTypeIDs(passTypeIDs []models.PassTypeIDResource, identifierPattern, identifierPrefix, namePattern string) []models.PassTypeIDResource {
	var filtered []models.PassTypeIDResource

	for _, passTypeID := range passTypeIDs {
		// Apply identifier pattern filter
		if identifierPattern != "" {
			matched, err := regexp.MatchString(identifierPattern, passTypeID.Attributes.Identifier)
			if err != nil || !matched {
				continue
			}
		}

		// Apply identifier prefix filter
		if identifierPrefix != "" {
			if !strings.HasPrefix(passTypeID.Attributes.Identifier, identifierPrefix) {
				continue
			}
		}

		// Apply name pattern filter
		if namePattern != "" {
			matched, err := regexp.MatchString(namePattern, passTypeID.Attributes.Name)
			if err != nil || !matched {
				continue
			}
		}

		filtered = append(filtered, passTypeID)
	}

	return filtered
}

// SortPassTypeIDs sorts a list of Pass Type IDs based on the specified field and order
func SortPassTypeIDs(passTypeIDs []models.PassTypeIDResource, sortBy, sortOrder string) []models.PassTypeIDResource {
	if sortBy == "" {
		sortBy = "identifier" // Default sort field
	}
	if sortOrder == "" {
		sortOrder = "asc" // Default sort order
	}

	sort.Slice(passTypeIDs, func(i, j int) bool {
		var compare int

		switch strings.ToLower(sortBy) {
		case "name":
			compare = strings.Compare(
				strings.ToLower(passTypeIDs[i].Attributes.Name),
				strings.ToLower(passTypeIDs[j].Attributes.Name),
			)
		case "identifier":
			compare = strings.Compare(
				strings.ToLower(passTypeIDs[i].Attributes.Identifier),
				strings.ToLower(passTypeIDs[j].Attributes.Identifier),
			)
		default:
			// Default to identifier
			compare = strings.Compare(
				strings.ToLower(passTypeIDs[i].Attributes.Identifier),
				strings.ToLower(passTypeIDs[j].Attributes.Identifier),
			)
		}

		if strings.ToLower(sortOrder) == "desc" {
			return compare > 0
		}
		return compare < 0
	})

	return passTypeIDs
}

// LimitPassTypeIDs applies a limit to the list of Pass Type IDs
func LimitPassTypeIDs(passTypeIDs []models.PassTypeIDResource, limit int) []models.PassTypeIDResource {
	if limit <= 0 || limit >= len(passTypeIDs) {
		return passTypeIDs
	}
	return passTypeIDs[:limit]
}
