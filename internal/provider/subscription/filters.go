// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// SubscriptionGroupFilters represents the filtering configuration for groups.
type SubscriptionGroupFilters struct {
	ReferenceNamePattern string
}

// FilterSubscriptionGroups applies filters to a list of subscription groups.
//
// Patterns are regular expressions matched as substrings unless anchored, which
// is the convention the device, merchant, passtypeid and profile packages
// follow. The bundle and certificate packages use globs instead; these are new
// data sources, so they take the more expressive of the two.
func FilterSubscriptionGroups(groups []models.SubscriptionGroup, filters SubscriptionGroupFilters) ([]models.SubscriptionGroup, error) {
	if filters.ReferenceNamePattern != "" {
		if _, err := regexp.Compile(filters.ReferenceNamePattern); err != nil {
			return nil, fmt.Errorf("invalid reference_name_pattern %q: %w", filters.ReferenceNamePattern, err)
		}
	}

	filtered := make([]models.SubscriptionGroup, 0, len(groups))
	for _, group := range groups {
		if filters.ReferenceNamePattern != "" {
			matched, err := regexp.MatchString(filters.ReferenceNamePattern, group.Attributes.ReferenceName)
			if err != nil || !matched {
				continue
			}
		}
		filtered = append(filtered, group)
	}

	return filtered, nil
}

// SortSubscriptionGroups sorts subscription groups in place.
func SortSubscriptionGroups(groups []models.SubscriptionGroup, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(groups, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "id":
			less = groups[i].ID < groups[j].ID
		case "reference_name":
			less = strings.ToLower(groups[i].Attributes.ReferenceName) < strings.ToLower(groups[j].Attributes.ReferenceName)
		default:
			less = strings.ToLower(groups[i].Attributes.ReferenceName) < strings.ToLower(groups[j].Attributes.ReferenceName)
		}

		if ascending {
			return less
		}
		return !less
	})
}

// SubscriptionFilters represents the filtering configuration for subscriptions.
type SubscriptionFilters struct {
	NamePattern      string
	ProductIDPattern string
	State            string
	Period           string
}

// FilterSubscriptions applies filters to a list of subscriptions.
func FilterSubscriptions(subscriptions []models.Subscription, filters SubscriptionFilters) ([]models.Subscription, error) {
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

	filtered := make([]models.Subscription, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		if shouldIncludeSubscription(subscription, filters) {
			filtered = append(filtered, subscription)
		}
	}

	return filtered, nil
}

// shouldIncludeSubscription determines whether one subscription passes filters.
func shouldIncludeSubscription(subscription models.Subscription, filters SubscriptionFilters) bool {
	if filters.NamePattern != "" {
		matched, err := regexp.MatchString(filters.NamePattern, subscription.Attributes.Name)
		if err != nil || !matched {
			return false
		}
	}

	if filters.ProductIDPattern != "" {
		matched, err := regexp.MatchString(filters.ProductIDPattern, subscription.Attributes.ProductID)
		if err != nil || !matched {
			return false
		}
	}

	// State and period are exact matches on Apple's enums, not patterns. A
	// subscription Apple reported without the attribute cannot match a filter
	// that asks for one.
	if filters.State != "" {
		if subscription.Attributes.State == nil || string(*subscription.Attributes.State) != filters.State {
			return false
		}
	}

	if filters.Period != "" {
		if subscription.Attributes.SubscriptionPeriod == nil || string(*subscription.Attributes.SubscriptionPeriod) != filters.Period {
			return false
		}
	}

	return true
}

// SortSubscriptions sorts subscriptions in place.
func SortSubscriptions(subscriptions []models.Subscription, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(subscriptions, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "name":
			less = strings.ToLower(subscriptions[i].Attributes.Name) < strings.ToLower(subscriptions[j].Attributes.Name)
		case "state":
			less = subscriptionStateOf(subscriptions[i]) < subscriptionStateOf(subscriptions[j])
		case "group_level":
			less = groupLevelOf(subscriptions[i]) < groupLevelOf(subscriptions[j])
		case "product_id":
			less = subscriptions[i].Attributes.ProductID < subscriptions[j].Attributes.ProductID
		default:
			less = subscriptions[i].Attributes.ProductID < subscriptions[j].Attributes.ProductID
		}

		if ascending {
			return less
		}
		return !less
	})
}

// subscriptionStateOf reads a state for sorting, treating a missing one as
// empty so it sorts consistently rather than panicking.
func subscriptionStateOf(subscription models.Subscription) string {
	if subscription.Attributes.State == nil {
		return ""
	}
	return string(*subscription.Attributes.State)
}

// groupLevelOf reads a group level for sorting. Apple assigns 1 when none was
// sent, so an absent level sorts as the highest service level.
func groupLevelOf(subscription models.Subscription) int {
	if subscription.Attributes.GroupLevel == nil {
		return 1
	}
	return *subscription.Attributes.GroupLevel
}

// SubscriptionPricePointFilters represents the in-memory filtering applied to
// price points after Apple has narrowed them by territory.
type SubscriptionPricePointFilters struct {
	CustomerPrice string
}

// FilterSubscriptionPricePoints applies filters to a list of price points.
//
// The territory filter is deliberately absent: it is applied server-side,
// because the unfiltered catalogue spans every territory the App Store sells in.
func FilterSubscriptionPricePoints(points []models.SubscriptionPricePoint, filters SubscriptionPricePointFilters) []models.SubscriptionPricePoint {
	if filters.CustomerPrice == "" {
		return points
	}

	filtered := make([]models.SubscriptionPricePoint, 0, len(points))
	for _, point := range points {
		if point.Attributes.CustomerPrice == filters.CustomerPrice {
			filtered = append(filtered, point)
		}
	}

	return filtered
}
