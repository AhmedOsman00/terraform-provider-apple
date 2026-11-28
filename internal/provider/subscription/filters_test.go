// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func state(s models.SubscriptionState) *models.SubscriptionState { return &s }
func period(p models.SubscriptionPeriod) *models.SubscriptionPeriod {
	return &p
}
func level(i int) *int { return &i }

func subscriptions() []models.Subscription {
	return []models.Subscription{
		{ID: "1", Attributes: models.SubscriptionAttributes{
			Name:               "Pro Monthly",
			ProductID:          "com.example.app.pro.monthly",
			State:              state(models.SubscriptionStateApproved),
			SubscriptionPeriod: period(models.SubscriptionPeriodOneMonth),
			GroupLevel:         level(1),
		}},
		{ID: "2", Attributes: models.SubscriptionAttributes{
			Name:               "Pro Yearly",
			ProductID:          "com.example.app.pro.yearly",
			State:              state(models.SubscriptionStateReadyToSubmit),
			SubscriptionPeriod: period(models.SubscriptionPeriodOneYear),
			GroupLevel:         level(1),
		}},
		{ID: "3", Attributes: models.SubscriptionAttributes{
			Name:               "Basic Monthly",
			ProductID:          "com.example.app.basic.monthly",
			State:              state(models.SubscriptionStateMissingMetadata),
			SubscriptionPeriod: period(models.SubscriptionPeriodOneMonth),
			GroupLevel:         level(2),
		}},
		// Apple omits attributes it has no value for. A subscription without a
		// state or period must not match a filter that asks for one, and must
		// not panic the sort comparators either.
		{ID: "4", Attributes: models.SubscriptionAttributes{
			Name:      "Bare",
			ProductID: "com.example.app.bare",
		}},
	}
}

func productIDs(list []models.Subscription) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, s.Attributes.ProductID)
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestFilterSubscriptions(t *testing.T) {
	tests := []struct {
		name    string
		filters SubscriptionFilters
		want    []string
	}{
		{
			name: "no filters returns everything",
			want: []string{
				"com.example.app.pro.monthly",
				"com.example.app.pro.yearly",
				"com.example.app.basic.monthly",
				"com.example.app.bare",
			},
		},
		{
			name:    "name pattern matches as substring",
			filters: SubscriptionFilters{NamePattern: "Pro"},
			want:    []string{"com.example.app.pro.monthly", "com.example.app.pro.yearly"},
		},
		{
			name:    "anchored name pattern",
			filters: SubscriptionFilters{NamePattern: "^Basic Monthly$"},
			want:    []string{"com.example.app.basic.monthly"},
		},
		{
			name:    "product id pattern",
			filters: SubscriptionFilters{ProductIDPattern: `\.monthly$`},
			want:    []string{"com.example.app.pro.monthly", "com.example.app.basic.monthly"},
		},
		{
			name:    "state is an exact match",
			filters: SubscriptionFilters{State: string(models.SubscriptionStateApproved)},
			want:    []string{"com.example.app.pro.monthly"},
		},
		{
			name:    "period is an exact match",
			filters: SubscriptionFilters{Period: string(models.SubscriptionPeriodOneYear)},
			want:    []string{"com.example.app.pro.yearly"},
		},
		{
			name: "filters combine",
			filters: SubscriptionFilters{
				ProductIDPattern: `^com\.example\.app\.pro`,
				Period:           string(models.SubscriptionPeriodOneMonth),
			},
			want: []string{"com.example.app.pro.monthly"},
		},
		{
			name:    "a subscription without a state never matches a state filter",
			filters: SubscriptionFilters{State: string(models.SubscriptionStateMissingMetadata)},
			want:    []string{"com.example.app.basic.monthly"},
		},
		{
			name:    "no matches yields an empty result rather than everything",
			filters: SubscriptionFilters{NamePattern: "nothing matches this"},
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterSubscriptions(subscriptions(), tt.filters)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalStrings(productIDs(got), tt.want) {
				t.Errorf("got %v, want %v", productIDs(got), tt.want)
			}
		})
	}
}

func TestFilterSubscriptionsRejectsMalformedPatterns(t *testing.T) {
	tests := []struct {
		name    string
		filters SubscriptionFilters
	}{
		{name: "name pattern", filters: SubscriptionFilters{NamePattern: "["}},
		{name: "product id pattern", filters: SubscriptionFilters{ProductIDPattern: "(unclosed"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := FilterSubscriptions(subscriptions(), tt.filters); err == nil {
				t.Fatal("expected an error for a malformed pattern, got nil")
			}
		})
	}
}

func TestSortSubscriptions(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{
			name: "defaults to product id ascending",
			want: []string{
				"com.example.app.bare",
				"com.example.app.basic.monthly",
				"com.example.app.pro.monthly",
				"com.example.app.pro.yearly",
			},
		},
		{
			name:      "product id descending",
			sortBy:    "product_id",
			sortOrder: "desc",
			want: []string{
				"com.example.app.pro.yearly",
				"com.example.app.pro.monthly",
				"com.example.app.basic.monthly",
				"com.example.app.bare",
			},
		},
		{
			name:   "name is case-insensitive",
			sortBy: "name",
			want: []string{
				"com.example.app.bare",
				"com.example.app.basic.monthly",
				"com.example.app.pro.monthly",
				"com.example.app.pro.yearly",
			},
		},
		{
			name:   "group level, with a missing level treated as 1",
			sortBy: "group_level",
			want: []string{
				"com.example.app.pro.monthly",
				"com.example.app.pro.yearly",
				"com.example.app.bare",
				"com.example.app.basic.monthly",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := subscriptions()
			SortSubscriptions(list, tt.sortBy, tt.sortOrder)
			if !equalStrings(productIDs(list), tt.want) {
				t.Errorf("got %v, want %v", productIDs(list), tt.want)
			}
		})
	}
}

func TestLimitIsAppliedAfterSorting(t *testing.T) {
	// The data sources limit after sorting rather than before, so a limit of 1
	// must yield the first record in sort order, not an arbitrary one.
	list := subscriptions()
	SortSubscriptions(list, "product_id", "asc")
	if got := productIDs(list)[:1]; !equalStrings(got, []string{"com.example.app.bare"}) {
		t.Errorf("got %v, want [com.example.app.bare]", got)
	}
}

func groups() []models.SubscriptionGroup {
	return []models.SubscriptionGroup{
		{ID: "10", Attributes: models.SubscriptionGroupAttributes{ReferenceName: "Premium"}},
		{ID: "11", Attributes: models.SubscriptionGroupAttributes{ReferenceName: "legacy tiers"}},
		{ID: "12", Attributes: models.SubscriptionGroupAttributes{ReferenceName: "Trials"}},
	}
}

func referenceNames(list []models.SubscriptionGroup) []string {
	out := make([]string, 0, len(list))
	for _, g := range list {
		out = append(out, g.Attributes.ReferenceName)
	}
	return out
}

func TestFilterSubscriptionGroups(t *testing.T) {
	tests := []struct {
		name    string
		filters SubscriptionGroupFilters
		want    []string
	}{
		{
			name: "no filters returns everything",
			want: []string{"Premium", "legacy tiers", "Trials"},
		},
		{
			name:    "substring match",
			filters: SubscriptionGroupFilters{ReferenceNamePattern: "ium"},
			want:    []string{"Premium"},
		},
		{
			name:    "anchored match",
			filters: SubscriptionGroupFilters{ReferenceNamePattern: "^Trials$"},
			want:    []string{"Trials"},
		},
		{
			name:    "matching is case-sensitive unless the pattern says otherwise",
			filters: SubscriptionGroupFilters{ReferenceNamePattern: "LEGACY"},
			want:    []string{},
		},
		{
			name:    "case-insensitive via an inline flag",
			filters: SubscriptionGroupFilters{ReferenceNamePattern: "(?i)LEGACY"},
			want:    []string{"legacy tiers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterSubscriptionGroups(groups(), tt.filters)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalStrings(referenceNames(got), tt.want) {
				t.Errorf("got %v, want %v", referenceNames(got), tt.want)
			}
		})
	}
}

func TestFilterSubscriptionGroupsRejectsMalformedPattern(t *testing.T) {
	if _, err := FilterSubscriptionGroups(groups(), SubscriptionGroupFilters{ReferenceNamePattern: "["}); err == nil {
		t.Fatal("expected an error for a malformed pattern, got nil")
	}
}

func TestSortSubscriptionGroups(t *testing.T) {
	// Sorting is case-insensitive, so "legacy tiers" sorts between "Premium"
	// and "Trials" rather than after both as a byte comparison would put it.
	list := groups()
	SortSubscriptionGroups(list, "reference_name", "asc")
	if want := []string{"legacy tiers", "Premium", "Trials"}; !equalStrings(referenceNames(list), want) {
		t.Errorf("got %v, want %v", referenceNames(list), want)
	}

	SortSubscriptionGroups(list, "id", "desc")
	if want := []string{"Trials", "legacy tiers", "Premium"}; !equalStrings(referenceNames(list), want) {
		t.Errorf("got %v, want %v", referenceNames(list), want)
	}
}

func pricePoints() []models.SubscriptionPricePoint {
	return []models.SubscriptionPricePoint{
		{ID: "p1", Attributes: models.SubscriptionPricePointAttributes{CustomerPrice: "9.99", Proceeds: "6.99"}},
		{ID: "p2", Attributes: models.SubscriptionPricePointAttributes{CustomerPrice: "19.99", Proceeds: "13.99"}},
		{ID: "p3", Attributes: models.SubscriptionPricePointAttributes{CustomerPrice: "9.99", Proceeds: "7.49"}},
	}
}

func TestFilterSubscriptionPricePoints(t *testing.T) {
	tests := []struct {
		name    string
		filters SubscriptionPricePointFilters
		want    []string
	}{
		{
			name: "no filter returns everything",
			want: []string{"p1", "p2", "p3"},
		},
		{
			name:    "exact price match can return several territories",
			filters: SubscriptionPricePointFilters{CustomerPrice: "9.99"},
			want:    []string{"p1", "p3"},
		},
		{
			name:    "a price that is not in the catalogue matches nothing",
			filters: SubscriptionPricePointFilters{CustomerPrice: "9.9"},
			want:    []string{},
		},
		{
			name:    "a trailing zero names the same price",
			filters: SubscriptionPricePointFilters{CustomerPrice: "19.990"},
			want:    []string{"p2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterSubscriptionPricePoints(pricePoints(), tt.filters)
			ids := make([]string, 0, len(got))
			for _, p := range got {
				ids = append(ids, p.ID)
			}
			if !equalStrings(ids, tt.want) {
				t.Errorf("got %v, want %v", ids, tt.want)
			}
		})
	}
}

// territoryPoint builds a price point carrying a territory linkage, the shape
// an equalization read returns.
func territoryPoint(id, territory string) models.SubscriptionPricePoint {
	return models.SubscriptionPricePoint{
		ID: id,
		Relationships: &models.SubscriptionPricePointRelationships{
			Territory: &models.ResourceIdentifier{
				Data: models.ResourceData{Type: "territories", ID: territory},
			},
		},
	}
}

func TestPricePointIDsByTerritory(t *testing.T) {
	tests := []struct {
		name   string
		points []models.SubscriptionPricePoint
		want   map[string]string
	}{
		{
			name:   "no points is an empty map, not nil",
			points: nil,
			want:   map[string]string{},
		},
		{
			name: "one entry per territory",
			points: []models.SubscriptionPricePoint{
				territoryPoint("eq-gbr", "GBR"),
				territoryPoint("eq-egy", "EGY"),
			},
			want: map[string]string{"GBR": "eq-gbr", "EGY": "eq-egy"},
		},
		{
			name:   "a point with no territory linkage is skipped",
			points: append(pricePoints(), territoryPoint("eq-gbr", "GBR")),
			want:   map[string]string{"GBR": "eq-gbr"},
		},
		{
			name: "an empty territory ID is skipped rather than keyed on the empty string",
			points: []models.SubscriptionPricePoint{
				territoryPoint("eq-blank", ""),
				territoryPoint("eq-gbr", "GBR"),
			},
			want: map[string]string{"GBR": "eq-gbr"},
		},
		{
			name: "a repeated territory keeps the first, so response order cannot change the result",
			points: []models.SubscriptionPricePoint{
				territoryPoint("eq-gbr", "GBR"),
				territoryPoint("eq-gbr-again", "GBR"),
			},
			want: map[string]string{"GBR": "eq-gbr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PricePointIDsByTerritory(tt.points)

			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for territory, id := range tt.want {
				if got[territory] != id {
					t.Errorf("territory %s = %q, want %q", territory, got[territory], id)
				}
			}
		})
	}
}

// reportedPrice builds one price as Apple returns it from
// GET /v1/subscriptions/{id}/prices with include=territory,subscriptionPricePoint.
func reportedPrice(id, pointID, territory string, startDate *string, planType *models.SubscriptionPlanType) models.SubscriptionPrice {
	return models.SubscriptionPrice{
		Type: "subscriptionPrices",
		ID:   id,
		Attributes: models.SubscriptionPriceAttributes{
			StartDate: startDate,
			PlanType:  planType,
		},
		Relationships: &models.SubscriptionPriceRelationships{
			Territory: &models.ResourceIdentifier{
				Data: models.ResourceData{Type: "territories", ID: territory},
			},
			SubscriptionPricePoint: &models.ResourceIdentifier{
				Data: models.ResourceData{Type: "subscriptionPricePoints", ID: pointID},
			},
		},
	}
}

// TestReconcileSchedulePricesKeepsConfiguredValues covers the reason this
// function exists: Apple reports a territory and a plan type on every price
// whether or not one was sent, so adopting what it returns would show as a
// permanent diff against a configuration that set neither.
func TestReconcileSchedulePricesKeepsConfiguredValues(t *testing.T) {
	configured := []subscriptionSchedulePriceModel{
		{PricePointID: types.StringValue("point-usa")},
	}

	monthly := models.SubscriptionPlanType("MONTHLY")
	reconciled := reconcileSchedulePrices(configured, []models.SubscriptionPrice{
		reportedPrice("price-1", "point-usa", "USA", nil, &monthly),
	})

	if len(reconciled) != 1 {
		t.Fatalf("got %d prices, want 1", len(reconciled))
	}
	if !reconciled[0].TerritoryID.IsNull() {
		t.Errorf("territory_id = %v, want null: the configuration named none", reconciled[0].TerritoryID)
	}
	if !reconciled[0].PlanType.IsNull() {
		t.Errorf("plan_type = %v, want null: the configuration named none", reconciled[0].PlanType)
	}
}

// TestReconcileSchedulePricesDropsPricesAppleNoLongerReports checks that a
// price deleted outside Terraform surfaces as drift rather than persisting.
func TestReconcileSchedulePricesDropsPricesAppleNoLongerReports(t *testing.T) {
	configured := []subscriptionSchedulePriceModel{
		{PricePointID: types.StringValue("point-usa")},
		{PricePointID: types.StringValue("point-gbr")},
	}

	reconciled := reconcileSchedulePrices(configured, []models.SubscriptionPrice{
		reportedPrice("price-1", "point-usa", "USA", nil, nil),
	})

	if len(reconciled) != 1 {
		t.Fatalf("got %d prices, want 1", len(reconciled))
	}
	if got := reconciled[0].PricePointID.ValueString(); got != "point-usa" {
		t.Errorf("surviving price point = %q, want point-usa", got)
	}
}

// TestReconcileSchedulePricesAdoptsUnknownPrices checks that a price added in
// App Store Connect appears with Apple's own values, so it shows up as drift
// instead of being silently discarded on the next plan.
func TestReconcileSchedulePricesAdoptsUnknownPrices(t *testing.T) {
	startDate := "2027-01-01"
	monthly := models.SubscriptionPlanType("MONTHLY")

	reconciled := reconcileSchedulePrices(nil, []models.SubscriptionPrice{
		reportedPrice("price-2", "point-gbr", "GBR", &startDate, &monthly),
	})

	if len(reconciled) != 1 {
		t.Fatalf("got %d prices, want 1", len(reconciled))
	}
	adopted := reconciled[0]
	if got := adopted.PricePointID.ValueString(); got != "point-gbr" {
		t.Errorf("price_point_id = %q, want point-gbr", got)
	}
	if got := adopted.TerritoryID.ValueString(); got != "GBR" {
		t.Errorf("territory_id = %q, want GBR", got)
	}
	if got := adopted.StartDate.ValueString(); got != startDate {
		t.Errorf("start_date = %q, want %q", got, startDate)
	}
	if got := adopted.PlanType.ValueString(); got != "MONTHLY" {
		t.Errorf("plan_type = %q, want MONTHLY", got)
	}
	// preserveCurrentPrice is an instruction Apple never reports back.
	if !adopted.PreserveCurrentPrice.IsNull() {
		t.Errorf("preserve_current_price = %v, want null", adopted.PreserveCurrentPrice)
	}
}

// TestReconcileSchedulePricesSkipsPricesWithoutAPricePoint checks the read that
// forgot include=subscriptionPricePoint: a price whose point is unknown cannot
// be matched against configuration, and must not be keyed under the empty
// string.
func TestReconcileSchedulePricesSkipsPricesWithoutAPricePoint(t *testing.T) {
	reconciled := reconcileSchedulePrices(nil, []models.SubscriptionPrice{
		{Type: "subscriptionPrices", ID: "price-3"},
		{
			Type:          "subscriptionPrices",
			ID:            "price-4",
			Relationships: &models.SubscriptionPriceRelationships{},
		},
	})

	if len(reconciled) != 0 {
		t.Fatalf("got %d prices, want 0: neither reported a price point", len(reconciled))
	}
}

func strPtr(s string) *string { return &s }

// apiOffer builds an offer as Apple reports it.
func apiOffer(id, territory string, mode models.SubscriptionOfferMode, duration models.SubscriptionOfferDuration, periods int, pricePoint string) models.SubscriptionIntroductoryOffer {
	offer := models.SubscriptionIntroductoryOffer{
		ID: id,
		Attributes: models.SubscriptionIntroductoryOfferAttributes{
			OfferMode:       &mode,
			Duration:        &duration,
			NumberOfPeriods: &periods,
		},
		Relationships: &models.SubscriptionIntroductoryOfferRelationships{
			Territory: &models.ResourceIdentifier{Data: models.ResourceData{Type: "territories", ID: territory}},
		},
	}
	if pricePoint != "" {
		offer.Relationships.SubscriptionPricePoint = &models.ResourceIdentifier{
			Data: models.ResourceData{Type: "subscriptionPricePoints", ID: pricePoint},
		}
	}
	return offer
}

// offerEntry builds a configured schedule entry with nothing optional set.
func offerEntry(territory string, mode models.SubscriptionOfferMode, duration models.SubscriptionOfferDuration, periods int64, pricePoint string) introductoryOfferValues {
	entry := introductoryOfferValues{
		TerritoryID:                types.StringValue(territory),
		OfferMode:                  types.StringValue(string(mode)),
		Duration:                   types.StringValue(string(duration)),
		NumberOfPeriods:            types.Int64Value(periods),
		PricePointID:               types.StringNull(),
		StartDate:                  types.StringNull(),
		EndDate:                    types.StringNull(),
		TargetSubscriptionPlanType: types.StringNull(),
	}
	if pricePoint != "" {
		entry.PricePointID = types.StringValue(pricePoint)
	}
	return entry
}

func TestIntroductoryOfferPricePointProblem(t *testing.T) {
	tests := []struct {
		mode          string
		hasPricePoint bool
		wantProblem   bool
	}{
		{"FREE_TRIAL", false, false},
		{"FREE_TRIAL", true, true},
		{"PAY_AS_YOU_GO", true, false},
		{"PAY_AS_YOU_GO", false, true},
		{"PAY_UP_FRONT", true, false},
		{"PAY_UP_FRONT", false, true},
	}
	for _, tt := range tests {
		got := introductoryOfferPricePointProblem(tt.mode, tt.hasPricePoint) != ""
		if got != tt.wantProblem {
			t.Errorf("%s with price point %v: problem = %v, want %v", tt.mode, tt.hasPricePoint, got, tt.wantProblem)
		}
	}
}

func TestPlanIntroductoryOffers(t *testing.T) {
	const (
		free  = models.SubscriptionOfferModeFreeTrial
		payGo = models.SubscriptionOfferModePayAsYouGo
		week  = models.SubscriptionOfferDurationOneWeek
		month = models.SubscriptionOfferDurationOneMonth
	)

	t.Run("matching offers are kept, the rest removed and created", func(t *testing.T) {
		existing := []models.SubscriptionIntroductoryOffer{
			apiOffer("usa", "USA", free, week, 1, ""),
			apiOffer("gbr", "GBR", payGo, month, 3, "point-gbr"),
		}
		planned := []introductoryOfferValues{
			offerEntry("USA", free, week, 1, ""),
			offerEntry("GBR", payGo, month, 2, "point-gbr"),
		}

		plan := planIntroductoryOffers(planned, existing, nil)
		if len(plan.Keep) != 1 || plan.Keep[0].ID != "usa" || plan.Keep[0].EndDateChanged {
			t.Errorf("keep = %+v, want usa unchanged", plan.Keep)
		}
		if len(plan.Remove) != 1 || plan.Remove[0] != "gbr" {
			t.Errorf("remove = %v, want [gbr]", plan.Remove)
		}
		if len(plan.Create) != 1 || plan.Create[0].NumberOfPeriods.ValueInt64() != 2 {
			t.Errorf("create = %+v, want the two-period GBR offer", plan.Create)
		}
	})

	t.Run("a dated entry claims its offer before an undated one", func(t *testing.T) {
		later := apiOffer("later", "USA", free, week, 1, "")
		later.Attributes.StartDate = strPtr("2027-06-01")
		current := apiOffer("current", "USA", free, week, 1, "")
		current.Attributes.StartDate = strPtr("2026-10-01")
		existing := []models.SubscriptionIntroductoryOffer{later, current}

		dated := offerEntry("USA", free, week, 1, "")
		dated.StartDate = types.StringValue("2027-06-01")
		planned := []introductoryOfferValues{offerEntry("USA", free, week, 1, ""), dated}

		plan := planIntroductoryOffers(planned, existing, nil)
		if len(plan.Keep) != 2 || len(plan.Remove) != 0 || len(plan.Create) != 0 {
			t.Errorf("plan = %+v, want both offers kept", plan)
		}
	})

	t.Run("a different end date is an update, not a new offer", func(t *testing.T) {
		existing := []models.SubscriptionIntroductoryOffer{apiOffer("usa", "USA", free, week, 1, "")}
		existing[0].Attributes.EndDate = strPtr("2027-01-31")

		entry := offerEntry("USA", free, week, 1, "")
		entry.EndDate = types.StringValue("2027-02-28")

		plan := planIntroductoryOffers([]introductoryOfferValues{entry}, existing, nil)
		if len(plan.Keep) != 1 || !plan.Keep[0].EndDateChanged {
			t.Errorf("keep = %+v, want usa with a changed end date", plan.Keep)
		}
		if len(plan.Remove) != 0 || len(plan.Create) != 0 {
			t.Errorf("remove = %v, create = %v, want neither", plan.Remove, plan.Create)
		}
	})

	t.Run("an end date set outside Terraform is cleared only if state managed it", func(t *testing.T) {
		existing := []models.SubscriptionIntroductoryOffer{apiOffer("usa", "USA", free, week, 1, "")}
		existing[0].Attributes.EndDate = strPtr("2027-01-31")
		planned := []introductoryOfferValues{offerEntry("USA", free, week, 1, "")}

		// Never configured: Read never showed it, so a write must not clear it.
		unmanaged := managedEndDateOffers([]introductoryOfferValues{offerEntry("USA", free, week, 1, "")}, existing)
		if plan := planIntroductoryOffers(planned, existing, unmanaged); plan.Keep[0].EndDateChanged {
			t.Error("an end date state never held was cleared")
		}

		// Configured before and removed from the configuration now: clear it.
		prior := offerEntry("USA", free, week, 1, "")
		prior.EndDate = types.StringValue("2027-01-31")
		managed := managedEndDateOffers([]introductoryOfferValues{prior}, existing)
		if plan := planIntroductoryOffers(planned, existing, managed); !plan.Keep[0].EndDateChanged {
			t.Error("an end date removed from the configuration was not cleared")
		}
	})

	t.Run("an unset start date matches whatever Apple reports", func(t *testing.T) {
		existing := []models.SubscriptionIntroductoryOffer{apiOffer("usa", "USA", free, week, 1, "")}
		existing[0].Attributes.StartDate = strPtr("2026-10-29")

		plan := planIntroductoryOffers([]introductoryOfferValues{offerEntry("USA", free, week, 1, "")}, existing, nil)
		if len(plan.Keep) != 1 {
			t.Errorf("keep = %+v, want the offer matched", plan.Keep)
		}

		dated := offerEntry("USA", free, week, 1, "")
		dated.StartDate = types.StringValue("2026-11-01")
		plan = planIntroductoryOffers([]introductoryOfferValues{dated}, existing, nil)
		if len(plan.Keep) != 0 || len(plan.Create) != 1 || len(plan.Remove) != 1 {
			t.Errorf("plan = %+v, want a configured start date to replace the offer", plan)
		}
	})

	t.Run("an undated entry takes the earliest offer it fits", func(t *testing.T) {
		live := apiOffer("live", "USA", free, week, 1, "")
		live.Attributes.StartDate = strPtr("2026-10-29")
		scheduled := apiOffer("scheduled", "USA", free, week, 1, "")
		scheduled.Attributes.StartDate = strPtr("2027-02-01")

		plan := planIntroductoryOffers(
			[]introductoryOfferValues{offerEntry("USA", free, week, 1, "")},
			[]models.SubscriptionIntroductoryOffer{scheduled, live},
			nil,
		)
		if len(plan.Keep) != 1 || plan.Keep[0].ID != "live" {
			t.Errorf("keep = %+v, want the live offer", plan.Keep)
		}
		if len(plan.Remove) != 1 || plan.Remove[0] != "scheduled" {
			t.Errorf("remove = %v, want the scheduled offer", plan.Remove)
		}
	})

	t.Run("a start date is matched before a plan type", func(t *testing.T) {
		monthly := models.SubscriptionPlanType("MONTHLY")
		first := apiOffer("first", "USA", free, week, 1, "")
		first.Attributes.StartDate = strPtr("2026-10-29")
		first.Attributes.TargetSubscriptionPlanType = &monthly
		second := apiOffer("second", "USA", free, week, 1, "")
		second.Attributes.StartDate = strPtr("2027-02-01")
		second.Attributes.TargetSubscriptionPlanType = &monthly

		typed := offerEntry("USA", free, week, 1, "")
		typed.TargetSubscriptionPlanType = types.StringValue("MONTHLY")
		dated := offerEntry("USA", free, week, 1, "")
		dated.StartDate = types.StringValue("2026-10-29")

		plan := planIntroductoryOffers(
			[]introductoryOfferValues{typed, dated},
			[]models.SubscriptionIntroductoryOffer{first, second},
			nil,
		)
		if len(plan.Keep) != 2 || len(plan.Create) != 0 || len(plan.Remove) != 0 {
			t.Errorf("plan = %+v, want both offers kept", plan)
		}
	})

	t.Run("each offer satisfies one entry", func(t *testing.T) {
		existing := []models.SubscriptionIntroductoryOffer{apiOffer("usa", "USA", free, week, 1, "")}
		planned := []introductoryOfferValues{
			offerEntry("USA", free, week, 1, ""),
			offerEntry("USA", free, week, 1, ""),
		}

		plan := planIntroductoryOffers(planned, existing, nil)
		if len(plan.Keep) != 1 || len(plan.Create) != 1 {
			t.Errorf("keep = %d, create = %d, want 1 and 1", len(plan.Keep), len(plan.Create))
		}
	})
}

func TestReconcileIntroductoryOffers(t *testing.T) {
	free := models.SubscriptionOfferModeFreeTrial
	week := models.SubscriptionOfferDurationOneWeek

	usa := apiOffer("usa", "USA", free, week, 1, "")
	usa.Attributes.StartDate = strPtr("2026-10-29")
	usa.Attributes.EndDate = strPtr("2027-03-31")
	manual := apiOffer("can", "CAN", models.SubscriptionOfferModePayUpFront, models.SubscriptionOfferDurationThreeMonths, 1, "point-can")

	configured := offerEntry("USA", free, week, 1, "")
	configured.EndDate = types.StringValue("2027-01-31")
	gone := offerEntry("GBR", free, week, 1, "")

	got := reconcileIntroductoryOffers([]introductoryOfferValues{configured, gone}, []models.SubscriptionIntroductoryOffer{usa, manual})
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}

	kept := got[0]
	if !kept.StartDate.IsNull() {
		t.Errorf("start_date = %s, want the configured null kept", kept.StartDate)
	}
	if kept.EndDate.ValueString() != "2027-03-31" {
		t.Errorf("end_date = %s, want Apple's 2027-03-31 refreshed", kept.EndDate)
	}

	adopted := got[1]
	if adopted.TerritoryID.ValueString() != "CAN" || adopted.PricePointID.ValueString() != "point-can" {
		t.Errorf("adopted = %+v, want the CAN offer with its price point", adopted)
	}

	imported := reconcileIntroductoryOffers(nil, []models.SubscriptionIntroductoryOffer{usa})
	if !imported[0].PricePointID.IsNull() {
		t.Errorf("free trial price_point_id = %s, want null", imported[0].PricePointID)
	}
	if imported[0].StartDate.ValueString() != "2026-10-29" {
		t.Errorf("imported start_date = %s, want Apple's", imported[0].StartDate)
	}
}

func TestIntroductoryOfferGone(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{errors.New("API error (status 404): Not Found - no such resource"), true},
		{errors.New("HTTP error: status 404, body: "), true},
		{errors.New("API error (status 500): Unexpected"), false},
		{errors.New(`Get "https://api.appstoreconnect.apple.com/v1/subscriptions/6740404123/introductoryOffers": context deadline exceeded`), false},
	}
	for _, tt := range tests {
		if got := introductoryOfferGone(tt.err); got != tt.want {
			t.Errorf("introductoryOfferGone(%q) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

// TestPendingEndDates covers recording state after a write that failed before
// it cleared an end date: the date Apple still holds is kept, so the next plan
// shows it and the next write retries the clear.
func TestPendingEndDates(t *testing.T) {
	free := models.SubscriptionOfferModeFreeTrial
	week := models.SubscriptionOfferDurationOneWeek

	usa := apiOffer("usa", "USA", free, week, 1, "")
	usa.Attributes.EndDate = strPtr("2027-01-31")
	gbr := apiOffer("gbr", "GBR", free, week, 1, "")
	gbr.Attributes.EndDate = strPtr("2027-02-28")
	offers := []models.SubscriptionIntroductoryOffer{usa, gbr}

	// State managed USA's end date and the plan removes it. GBR's was set by
	// hand and never in state.
	managedPrior := offerEntry("USA", free, week, 1, "")
	managedPrior.EndDate = types.StringValue("2027-01-31")
	prior := []introductoryOfferValues{managedPrior, offerEntry("GBR", free, week, 1, "")}
	planned := []introductoryOfferValues{offerEntry("USA", free, week, 1, ""), offerEntry("GBR", free, week, 1, "")}

	got := reconcileIntroductoryOffers(pendingEndDates(planned, prior, offers), offers)
	ends := map[string]types.String{}
	for _, entry := range got {
		ends[entry.TerritoryID.ValueString()] = entry.EndDate
	}

	if ends["USA"].ValueString() != "2027-01-31" {
		t.Errorf("USA end_date = %s, want the uncleared 2027-01-31 recorded", ends["USA"])
	}
	if !ends["GBR"].IsNull() {
		t.Errorf("GBR end_date = %s, want null: state never held it", ends["GBR"])
	}
}

// TestIntroductoryOfferEntryFromAPI covers what an import records: Apple's own
// values, with a free trial's price point null rather than "", so that it
// matches a configuration that leaves price_point_id unset.
func TestIntroductoryOfferEntryFromAPI(t *testing.T) {
	trial := apiOffer("usa", "USA", models.SubscriptionOfferModeFreeTrial, models.SubscriptionOfferDurationOneWeek, 1, "")
	trial.Attributes.StartDate = strPtr("2026-10-29")
	planType := models.SubscriptionPlanTypeUpfront
	trial.Attributes.TargetSubscriptionPlanType = &planType

	got := introductoryOfferEntryFromAPI(trial)
	if got.TerritoryID.ValueString() != "USA" || got.OfferMode.ValueString() != "FREE_TRIAL" ||
		got.Duration.ValueString() != "ONE_WEEK" || got.NumberOfPeriods.ValueInt64() != 1 {
		t.Errorf("identifying attributes = %+v", got)
	}
	if !got.PricePointID.IsNull() {
		t.Errorf("free trial price_point_id = %s, want null", got.PricePointID)
	}
	if got.StartDate.ValueString() != "2026-10-29" {
		t.Errorf("start_date = %s, want Apple's 2026-10-29", got.StartDate)
	}
	if !got.EndDate.IsNull() {
		t.Errorf("end_date = %s, want null", got.EndDate)
	}
	if got.TargetSubscriptionPlanType.ValueString() != "UPFRONT" {
		t.Errorf("target_subscription_plan_type = %s, want UPFRONT", got.TargetSubscriptionPlanType)
	}

	paid := apiOffer("gbr", "GBR", models.SubscriptionOfferModePayAsYouGo, models.SubscriptionOfferDurationOneMonth, 3, "point-gbr")
	if got := introductoryOfferEntryFromAPI(paid); got.PricePointID.ValueString() != "point-gbr" {
		t.Errorf("paid price_point_id = %s, want point-gbr", got.PricePointID)
	}
}

// preservedPrice marks a reported price as preserved: the old price Apple keeps
// for existing subscribers after a change that preserves it.
func preservedPrice(price models.SubscriptionPrice) models.SubscriptionPrice {
	preserved := true
	price.Attributes.Preserved = &preserved
	return price
}

// TestReconcileSchedulePricesAdoptsUpfrontAsUnset checks that the ordinary
// price, which Apple reports as UPFRONT, is adopted with plan_type unset, so a
// configuration that leaves plan_type out does not plan an update forever.
func TestReconcileSchedulePricesAdoptsUpfrontAsUnset(t *testing.T) {
	upfront := models.SubscriptionPlanTypeUpfront

	reconciled := reconcileSchedulePrices(nil, []models.SubscriptionPrice{
		reportedPrice("price-1", "point-usa", "USA", nil, &upfront),
	})

	if len(reconciled) != 1 {
		t.Fatalf("got %d prices, want 1", len(reconciled))
	}
	if !reconciled[0].PlanType.IsNull() {
		t.Errorf("plan_type = %v, want null for Apple's UPFRONT", reconciled[0].PlanType)
	}
}

// TestReconcileSchedulePricesIgnoresPriceHistory checks that the ended and
// preserved records Apple lists beside a price are not adopted as prices of
// their own once state holds the price that superseded them.
func TestReconcileSchedulePricesIgnoresPriceHistory(t *testing.T) {
	configured := []subscriptionSchedulePriceModel{
		{PricePointID: types.StringValue("point-usa-9")},
	}
	changed := "2026-12-01"

	reconciled := reconcileSchedulePrices(configured, []models.SubscriptionPrice{
		preservedPrice(reportedPrice("price-old", "point-usa-8", "USA", nil, nil)),
		reportedPrice("price-ended", "point-usa-7", "USA", nil, nil),
		reportedPrice("price-new", "point-usa-9", "USA", &changed, nil),
	})

	if len(reconciled) != 1 {
		t.Fatalf("got %d prices, want 1: only the price in effect is the price", len(reconciled))
	}
	if got := reconciled[0].PricePointID.ValueString(); got != "point-usa-9" {
		t.Errorf("price point = %q, want point-usa-9", got)
	}
}

// TestReconcileSchedulePricesAdoptsAChangeMadeByHand checks a price changed in
// App Store Connect: the configured price is superseded, so it leaves state,
// and the new one takes its form, so the plan shows only the price point.
func TestReconcileSchedulePricesAdoptsAChangeMadeByHand(t *testing.T) {
	configured := []subscriptionSchedulePriceModel{{
		PricePointID:         types.StringValue("point-usa-8"),
		TerritoryID:          types.StringNull(),
		StartDate:            types.StringNull(),
		PreserveCurrentPrice: types.BoolValue(true),
		PlanType:             types.StringNull(),
	}}
	changed := "2026-12-01"
	upfront := models.SubscriptionPlanTypeUpfront

	reconciled := reconcileSchedulePrices(configured, []models.SubscriptionPrice{
		reportedPrice("price-old", "point-usa-8", "USA", nil, &upfront),
		reportedPrice("price-new", "point-usa-9", "USA", &changed, &upfront),
	})

	if len(reconciled) != 1 {
		t.Fatalf("got %d prices, want 1", len(reconciled))
	}
	adopted := reconciled[0]
	if got := adopted.PricePointID.ValueString(); got != "point-usa-9" {
		t.Errorf("price point = %q, want point-usa-9", got)
	}
	if !adopted.StartDate.IsNull() {
		t.Errorf("start_date = %v, want null like the price it replaced", adopted.StartDate)
	}
	if !adopted.TerritoryID.IsNull() {
		t.Errorf("territory_id = %v, want null like the price it replaced", adopted.TerritoryID)
	}
	if !adopted.PreserveCurrentPrice.ValueBool() {
		t.Errorf("preserve_current_price = %v, want true from the price it replaced", adopted.PreserveCurrentPrice)
	}
	if !adopted.PlanType.IsNull() {
		t.Errorf("plan_type = %v, want null", adopted.PlanType)
	}
}

// TestReconcileSchedulePricesKeepsExplicitUpfront checks that a replaced price
// that said UPFRONT passes that form on.
func TestReconcileSchedulePricesKeepsExplicitUpfront(t *testing.T) {
	configured := []subscriptionSchedulePriceModel{{
		PricePointID: types.StringValue("point-usa-8"),
		TerritoryID:  types.StringValue("USA"),
		PlanType:     types.StringValue("UPFRONT"),
	}}
	upfront := models.SubscriptionPlanTypeUpfront

	reconciled := reconcileSchedulePrices(configured, []models.SubscriptionPrice{
		reportedPrice("price-new", "point-usa-9", "USA", nil, &upfront),
	})

	if len(reconciled) != 1 {
		t.Fatalf("got %d prices, want 1", len(reconciled))
	}
	if got := reconciled[0].PlanType.ValueString(); got != "UPFRONT" {
		t.Errorf("plan_type = %q, want UPFRONT", got)
	}
}

// TestReconcileSchedulePricesKeepsAScheduleConfiguredBeforeApproval checks
// that a price and a dated change of one territory, both in state, both stay.
func TestReconcileSchedulePricesKeepsAScheduleConfiguredBeforeApproval(t *testing.T) {
	later := "2027-01-01"
	configured := []subscriptionSchedulePriceModel{
		{PricePointID: types.StringValue("point-usa-8"), TerritoryID: types.StringValue("USA")},
		{PricePointID: types.StringValue("point-usa-9"), TerritoryID: types.StringValue("USA"), StartDate: types.StringValue(later)},
	}

	reconciled := reconcileSchedulePrices(configured, []models.SubscriptionPrice{
		reportedPrice("price-1", "point-usa-8", "USA", nil, nil),
		reportedPrice("price-2", "point-usa-9", "USA", &later, nil),
	})

	if len(reconciled) != 2 {
		t.Fatalf("got %d prices, want both configured prices", len(reconciled))
	}
}

var priceChangeToday = time.Date(2026, 11, 25, 22, 0, 0, 0, time.UTC)

func plannedPrice(point, territory string) subscriptionSchedulePriceModel {
	price := subscriptionSchedulePriceModel{
		PricePointID:         types.StringValue(point),
		TerritoryID:          types.StringNull(),
		StartDate:            types.StringNull(),
		PreserveCurrentPrice: types.BoolNull(),
		PlanType:             types.StringNull(),
	}
	if territory != "" {
		price.TerritoryID = types.StringValue(territory)
	}
	return price
}

func TestPlanPriceChanges(t *testing.T) {
	future := "2026-12-10"
	past := "2026-01-01"

	tests := []struct {
		name          string
		planned       []subscriptionSchedulePriceModel
		reported      []models.SubscriptionPrice
		wantCreations map[string]string // territory -> "point@startDate"
		wantDeletions []string
		wantNoTerr    []string
		wantUnmanaged []string
	}{
		{
			name:     "a price already in effect sends nothing",
			planned:  []subscriptionSchedulePriceModel{plannedPrice("usa-8", "")},
			reported: []models.SubscriptionPrice{reportedPrice("p1", "usa-8", "USA", nil, nil)},
		},
		{
			name:          "a changed price is dated three days out",
			planned:       []subscriptionSchedulePriceModel{plannedPrice("usa-9", "USA")},
			reported:      []models.SubscriptionPrice{reportedPrice("p1", "usa-8", "USA", nil, nil)},
			wantCreations: map[string]string{"USA": "usa-9@2026-11-28"},
		},
		{
			name:          "the latest dated price is the one in effect",
			planned:       []subscriptionSchedulePriceModel{plannedPrice("usa-8", "USA")},
			reported:      []models.SubscriptionPrice{reportedPrice("p1", "usa-8", "USA", nil, nil), reportedPrice("p2", "usa-7", "USA", &past, nil)},
			wantCreations: map[string]string{"USA": "usa-8@2026-11-28"},
		},
		{
			name:     "a change already scheduled is not sent again",
			planned:  []subscriptionSchedulePriceModel{plannedPrice("usa-9", "USA")},
			reported: []models.SubscriptionPrice{reportedPrice("p1", "usa-8", "USA", nil, nil), reportedPrice("p2", "usa-9", "USA", &future, nil)},
		},
		{
			name:          "an unwanted scheduled change is deleted",
			planned:       []subscriptionSchedulePriceModel{plannedPrice("usa-8", "USA")},
			reported:      []models.SubscriptionPrice{reportedPrice("p1", "usa-8", "USA", nil, nil), reportedPrice("p2", "usa-9", "USA", &future, nil)},
			wantDeletions: []string{"p2"},
		},
		{
			name:     "a preserved record is not the price in effect",
			planned:  []subscriptionSchedulePriceModel{plannedPrice("usa-9", "USA")},
			reported: []models.SubscriptionPrice{preservedPrice(reportedPrice("p1", "usa-8", "USA", nil, nil)), reportedPrice("p2", "usa-9", "USA", &past, nil)},
		},
		{
			name:          "the territory comes from Apple's prices when not configured",
			planned:       []subscriptionSchedulePriceModel{plannedPrice("usa-8", "")},
			reported:      []models.SubscriptionPrice{reportedPrice("p1", "usa-8", "USA", &past, nil), reportedPrice("p2", "usa-9", "USA", &future, nil)},
			wantDeletions: []string{"p2"},
		},
		{
			name:          "a new price point with no territory cannot be posted",
			planned:       []subscriptionSchedulePriceModel{plannedPrice("usa-9", "")},
			reported:      []models.SubscriptionPrice{reportedPrice("p1", "usa-8", "USA", nil, nil)},
			wantNoTerr:    []string{"usa-9"},
			wantUnmanaged: []string{"USA"},
		},
		{
			name:          "a territory the plan leaves out is reported and left alone",
			planned:       []subscriptionSchedulePriceModel{plannedPrice("usa-8", "USA")},
			reported:      []models.SubscriptionPrice{reportedPrice("p1", "usa-8", "USA", nil, nil), reportedPrice("p2", "gbr-8", "GBR", nil, nil)},
			wantUnmanaged: []string{"GBR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := planPriceChanges(tt.planned, tt.reported, priceChangeToday)

			got := make(map[string]string, len(changes.Creations))
			for _, c := range changes.Creations {
				got[c.TerritoryID] = c.PricePointID + "@" + *c.StartDate
			}
			if len(got) != len(tt.wantCreations) {
				t.Errorf("creations = %v, want %v", got, tt.wantCreations)
			}
			for territory, want := range tt.wantCreations {
				if got[territory] != want {
					t.Errorf("creation in %s = %q, want %q", territory, got[territory], want)
				}
			}
			if !slices.Equal(changes.Deletions, tt.wantDeletions) {
				t.Errorf("deletions = %v, want %v", changes.Deletions, tt.wantDeletions)
			}
			if !slices.Equal(changes.NoTerritory, tt.wantNoTerr) {
				t.Errorf("no territory = %v, want %v", changes.NoTerritory, tt.wantNoTerr)
			}
			if !slices.Equal(changes.Unmanaged, tt.wantUnmanaged) {
				t.Errorf("unmanaged = %v, want %v", changes.Unmanaged, tt.wantUnmanaged)
			}
		})
	}
}

// TestPlanPriceChangesSendsTheConfiguredInstruction checks that an explicit
// start date, preserve_current_price and plan type travel with the change.
func TestPlanPriceChangesSendsTheConfiguredInstruction(t *testing.T) {
	price := plannedPrice("usa-9", "USA")
	price.StartDate = types.StringValue("2027-02-01")
	price.PreserveCurrentPrice = types.BoolValue(true)
	price.PlanType = types.StringValue("MONTHLY")

	changes := planPriceChanges([]subscriptionSchedulePriceModel{price}, []models.SubscriptionPrice{
		reportedPrice("p1", "usa-8", "USA", nil, nil),
	}, priceChangeToday)

	if len(changes.Creations) != 1 {
		t.Fatalf("got %d creations, want 1", len(changes.Creations))
	}
	c := changes.Creations[0]
	if *c.StartDate != "2027-02-01" {
		t.Errorf("start date = %q, want the configured 2027-02-01", *c.StartDate)
	}
	if c.PreserveCurrentPrice == nil || !*c.PreserveCurrentPrice {
		t.Errorf("preserveCurrentPrice = %v, want true", c.PreserveCurrentPrice)
	}
	if c.PlanType == nil || *c.PlanType != models.SubscriptionPlanTypeMonthly {
		t.Errorf("planType = %v, want MONTHLY", c.PlanType)
	}
}
