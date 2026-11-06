// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/types"
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

// PricePointIDsByTerritory indexes price points by the territory they price.
//
// This is what makes an equalization read usable from a for_each: a map from
// territory code to price point ID, so a price resource can look its own
// storefront up instead of filtering the nested list. A point Apple returns
// without a territory linkage is skipped rather than keyed under the empty
// string -- an unidentifiable point is not addressable as anything.
//
// Apple returns one point per territory here, so a duplicate key should not
// occur; if one does, the first is kept, because the alternative is a map whose
// contents depend on response order.
func PricePointIDsByTerritory(points []models.SubscriptionPricePoint) map[string]string {
	ids := make(map[string]string, len(points))

	for _, point := range points {
		if point.Relationships == nil || point.Relationships.Territory == nil {
			continue
		}

		territory := point.Relationships.Territory.Data.ID
		if territory == "" {
			continue
		}
		if _, seen := ids[territory]; seen {
			continue
		}

		ids[territory] = point.ID
	}

	return ids
}

// introductoryOfferPricePointProblem reports what is wrong with an offer's price
// point for its mode, or "" when nothing is.
//
// A free trial charges nothing and so references no price point; the two paid
// modes are a reduced price, and a price is always a price point. Apple refuses
// both mistakes, but at apply time and without naming the mode.
func introductoryOfferPricePointProblem(mode string, hasPricePoint bool) string {
	switch models.SubscriptionOfferMode(mode) {
	case models.SubscriptionOfferModeFreeTrial:
		if hasPricePoint {
			return "A FREE_TRIAL offer charges nothing, so it takes no price_point_id. Remove price_point_id, " +
				"or use PAY_AS_YOU_GO or PAY_UP_FRONT for a reduced price."
		}
	case models.SubscriptionOfferModePayAsYouGo, models.SubscriptionOfferModePayUpFront:
		if !hasPricePoint {
			return fmt.Sprintf("A %s offer is a reduced price, and a price is always a price point. Set "+
				"price_point_id to a point from the apple_subscription_price_points data source for this "+
				"subscription and territory, or use FREE_TRIAL.", mode)
		}
	}
	return ""
}

// offerTerritory reads an offer's territory, or "" when Apple did not populate it.
func offerTerritory(offer models.SubscriptionIntroductoryOffer) string {
	if offer.Relationships == nil {
		return ""
	}
	return territoryID(offer.Relationships.Territory).ValueString()
}

// offerPricePoint reads an offer's price point, or "" for a free trial.
func offerPricePoint(offer models.SubscriptionIntroductoryOffer) string {
	if offer.Relationships == nil || offer.Relationships.SubscriptionPricePoint == nil {
		return ""
	}
	return offer.Relationships.SubscriptionPricePoint.Data.ID
}

// introductoryOfferEntryFromAPI records an offer with Apple's own values, as an
// import does. A free trial's price point is null rather than "".
func introductoryOfferEntryFromAPI(offer models.SubscriptionIntroductoryOffer) introductoryOfferValues {
	attributes := offer.Attributes

	entry := introductoryOfferValues{
		TerritoryID:                types.StringNull(),
		OfferMode:                  types.StringNull(),
		Duration:                   types.StringNull(),
		NumberOfPeriods:            types.Int64Null(),
		PricePointID:               types.StringNull(),
		StartDate:                  stringOrNull(attributes.StartDate),
		EndDate:                    stringOrNull(attributes.EndDate),
		TargetSubscriptionPlanType: types.StringNull(),
	}
	if territory := offerTerritory(offer); territory != "" {
		entry.TerritoryID = types.StringValue(territory)
	}
	if point := offerPricePoint(offer); point != "" {
		entry.PricePointID = types.StringValue(point)
	}
	if attributes.OfferMode != nil {
		entry.OfferMode = types.StringValue(string(*attributes.OfferMode))
	}
	if attributes.Duration != nil {
		entry.Duration = types.StringValue(string(*attributes.Duration))
	}
	if attributes.NumberOfPeriods != nil {
		entry.NumberOfPeriods = types.Int64Value(int64(*attributes.NumberOfPeriods))
	}
	if attributes.TargetSubscriptionPlanType != nil {
		entry.TargetSubscriptionPlanType = types.StringValue(string(*attributes.TargetSubscriptionPlanType))
	}
	return entry
}

// introductoryOfferInput converts an offer's values to the client's input.
func introductoryOfferInput(entry introductoryOfferValues) apple.SubscriptionIntroductoryOfferInput {
	input := apple.SubscriptionIntroductoryOfferInput{
		TerritoryID:     entry.TerritoryID.ValueString(),
		OfferMode:       models.SubscriptionOfferMode(entry.OfferMode.ValueString()),
		Duration:        models.SubscriptionOfferDuration(entry.Duration.ValueString()),
		NumberOfPeriods: int(entry.NumberOfPeriods.ValueInt64()),
		PricePointID:    entry.PricePointID.ValueString(),
		StartDate:       stringOrNil(entry.StartDate),
		EndDate:         stringOrNil(entry.EndDate),
	}
	if planType := stringOrNil(entry.TargetSubscriptionPlanType); planType != nil {
		t := models.SubscriptionPlanType(*planType)
		input.TargetPlanType = &t
	}
	return input
}
