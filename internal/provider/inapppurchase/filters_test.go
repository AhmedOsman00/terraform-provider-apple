// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package inapppurchase

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

func state(s models.InAppPurchaseState) *models.InAppPurchaseState { return &s }
func kind(t models.InAppPurchaseType) *models.InAppPurchaseType    { return &t }

func purchases() []models.InAppPurchase {
	return []models.InAppPurchase{
		{ID: "1", Attributes: models.InAppPurchaseAttributes{
			Name:              "Pro Unlock",
			ProductID:         "com.example.app.prounlock",
			InAppPurchaseType: kind(models.InAppPurchaseTypeNonConsumable),
			State:             state(models.InAppPurchaseStateApproved),
		}},
		{ID: "2", Attributes: models.InAppPurchaseAttributes{
			Name:              "Coin Pack",
			ProductID:         "com.example.app.coins100",
			InAppPurchaseType: kind(models.InAppPurchaseTypeConsumable),
			State:             state(models.InAppPurchaseStateReadyToSubmit),
		}},
		{ID: "3", Attributes: models.InAppPurchaseAttributes{
			Name:              "Season Pass",
			ProductID:         "com.example.app.season",
			InAppPurchaseType: kind(models.InAppPurchaseTypeNonRenewingSubscription),
			State:             state(models.InAppPurchaseStateMissingMetadata),
		}},
		// Apple omits attributes it has no value for. A purchase without a
		// state or type must not match a filter that asks for one, and must not
		// panic the sort comparators either.
		{ID: "4", Attributes: models.InAppPurchaseAttributes{
			Name:      "Bare",
			ProductID: "com.example.app.bare",
		}},
	}
}

func productIDs(list []models.InAppPurchase) []string {
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, p.Attributes.ProductID)
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

func TestFilterInAppPurchases(t *testing.T) {
	tests := []struct {
		name    string
		filters InAppPurchaseFilters
		want    []string
	}{
		{
			name:    "no filters returns everything",
			filters: InAppPurchaseFilters{},
			want: []string{
				"com.example.app.prounlock",
				"com.example.app.coins100",
				"com.example.app.season",
				"com.example.app.bare",
			},
		},
		{
			name:    "name pattern matches as a substring",
			filters: InAppPurchaseFilters{NamePattern: "Pack"},
			want:    []string{"com.example.app.coins100"},
		},
		{
			name:    "anchored name pattern matches the whole name",
			filters: InAppPurchaseFilters{NamePattern: "^Bare$"},
			want:    []string{"com.example.app.bare"},
		},
		{
			name:    "product ID pattern",
			filters: InAppPurchaseFilters{ProductIDPattern: "coins"},
			want:    []string{"com.example.app.coins100"},
		},
		{
			name:    "type is an exact match",
			filters: InAppPurchaseFilters{InAppPurchaseType: string(models.InAppPurchaseTypeNonConsumable)},
			want:    []string{"com.example.app.prounlock"},
		},
		{
			name:    "state is an exact match",
			filters: InAppPurchaseFilters{State: string(models.InAppPurchaseStateReadyToSubmit)},
			want:    []string{"com.example.app.coins100"},
		},
		{
			name: "filters combine",
			filters: InAppPurchaseFilters{
				ProductIDPattern:  "^com.example.app",
				InAppPurchaseType: string(models.InAppPurchaseTypeConsumable),
			},
			want: []string{"com.example.app.coins100"},
		},
		{
			name:    "a purchase without a type never matches a type filter",
			filters: InAppPurchaseFilters{InAppPurchaseType: string(models.InAppPurchaseTypeConsumable), NamePattern: "Bare"},
			want:    []string{},
		},
		{
			name:    "a purchase without a state never matches a state filter",
			filters: InAppPurchaseFilters{State: string(models.InAppPurchaseStateApproved), NamePattern: "Bare"},
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterInAppPurchases(purchases(), tt.filters)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalStrings(productIDs(got), tt.want) {
				t.Errorf("got %v, want %v", productIDs(got), tt.want)
			}
		})
	}
}

func TestFilterInAppPurchasesRejectsMalformedPatterns(t *testing.T) {
	tests := []struct {
		name    string
		filters InAppPurchaseFilters
	}{
		{"name pattern", InAppPurchaseFilters{NamePattern: "["}},
		{"product ID pattern", InAppPurchaseFilters{ProductIDPattern: "(unclosed"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := FilterInAppPurchases(purchases(), tt.filters); err == nil {
				t.Fatal("expected an error for a malformed pattern, got none")
			}
		})
	}
}

func TestSortInAppPurchases(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{
			name:   "default sorts by product ID ascending",
			sortBy: "",
			want: []string{
				"com.example.app.bare",
				"com.example.app.coins100",
				"com.example.app.prounlock",
				"com.example.app.season",
			},
		},
		{
			name:      "product ID descending",
			sortBy:    "product_id",
			sortOrder: "desc",
			want: []string{
				"com.example.app.season",
				"com.example.app.prounlock",
				"com.example.app.coins100",
				"com.example.app.bare",
			},
		},
		{
			name:   "name is case-insensitive",
			sortBy: "name",
			want: []string{
				"com.example.app.bare",
				"com.example.app.coins100",
				"com.example.app.prounlock",
				"com.example.app.season",
			},
		},
		{
			// The purchase Apple reported without a type sorts as the empty
			// string, which is what keeps the comparator total.
			name:   "type sorts missing values first",
			sortBy: "in_app_purchase_type",
			want: []string{
				"com.example.app.bare",
				"com.example.app.coins100",
				"com.example.app.prounlock",
				"com.example.app.season",
			},
		},
		{
			name:   "state sorts missing values first",
			sortBy: "state",
			want: []string{
				"com.example.app.bare",
				"com.example.app.prounlock",
				"com.example.app.season",
				"com.example.app.coins100",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := purchases()
			SortInAppPurchases(list, tt.sortBy, tt.sortOrder)
			if !equalStrings(productIDs(list), tt.want) {
				t.Errorf("got %v, want %v", productIDs(list), tt.want)
			}
		})
	}
}

func TestFilterInAppPurchasePricePoints(t *testing.T) {
	points := []models.InAppPurchasePricePoint{
		{ID: "a", Attributes: models.InAppPurchasePricePointAttributes{CustomerPrice: "0.99", Proceeds: "0.70"}},
		{ID: "b", Attributes: models.InAppPurchasePricePointAttributes{CustomerPrice: "9.99", Proceeds: "6.99"}},
		{ID: "c", Attributes: models.InAppPurchasePricePointAttributes{CustomerPrice: "9.99", Proceeds: "7.00"}},
	}

	t.Run("no filter returns everything", func(t *testing.T) {
		if got := FilterInAppPurchasePricePoints(points, InAppPurchasePricePointFilters{}); len(got) != 3 {
			t.Errorf("got %d price points, want 3", len(got))
		}
	})

	t.Run("customer price is an exact string match", func(t *testing.T) {
		got := FilterInAppPurchasePricePoints(points, InAppPurchasePricePointFilters{CustomerPrice: "9.99"})
		if len(got) != 2 || got[0].ID != "b" || got[1].ID != "c" {
			t.Errorf("got %v, want price points b and c", got)
		}
	})

	t.Run("a price that formats differently does not match", func(t *testing.T) {
		if got := FilterInAppPurchasePricePoints(points, InAppPurchasePricePointFilters{CustomerPrice: "9.990"}); len(got) != 0 {
			t.Errorf("got %d price points, want 0", len(got))
		}
	})
}

func TestReconcilePricesKeepsConfiguredDates(t *testing.T) {
	apple := func(pointID string, start, end *string) models.InAppPurchasePrice {
		return models.InAppPurchasePrice{
			ID: "price-" + pointID,
			Attributes: models.InAppPurchasePriceAttributes{
				StartDate: start,
				EndDate:   end,
			},
			Relationships: &models.InAppPurchasePriceRelationships{
				InAppPurchasePricePoint: &models.ResourceIdentifier{
					Data: models.ResourceData{Type: "inAppPurchasePricePoints", ID: pointID},
				},
			},
		}
	}
	derived := "2026-06-01"

	t.Run("a price already in state keeps its configured dates", func(t *testing.T) {
		state := []inAppPurchasePriceModel{
			{PricePointID: types.StringValue("point-a"), StartDate: types.StringNull(), EndDate: types.StringNull()},
		}

		// Apple derives an end date once a later price supersedes this one.
		// Adopting it would show as a permanent diff against a configuration
		// that never set one.
		got := reconcilePrices(state, []models.InAppPurchasePrice{apple("point-a", nil, &derived)})

		if len(got) != 1 {
			t.Fatalf("got %d prices, want 1", len(got))
		}
		if !got[0].EndDate.IsNull() {
			t.Errorf("end date was adopted from Apple as %q, want it left null", got[0].EndDate.ValueString())
		}
	})

	t.Run("a price Apple no longer reports is dropped", func(t *testing.T) {
		state := []inAppPurchasePriceModel{
			{PricePointID: types.StringValue("point-a")},
			{PricePointID: types.StringValue("point-gone")},
		}

		got := reconcilePrices(state, []models.InAppPurchasePrice{apple("point-a", nil, nil)})

		if len(got) != 1 || got[0].PricePointID.ValueString() != "point-a" {
			t.Errorf("got %v, want only point-a", got)
		}
	})

	t.Run("a price added outside Terraform surfaces with Apple's dates", func(t *testing.T) {
		start := "2026-01-01"

		got := reconcilePrices(nil, []models.InAppPurchasePrice{apple("point-b", &start, nil)})

		if len(got) != 1 {
			t.Fatalf("got %d prices, want 1", len(got))
		}
		if got[0].PricePointID.ValueString() != "point-b" || got[0].StartDate.ValueString() != start {
			t.Errorf("got %+v, want point-b starting %s", got[0], start)
		}
	})

	t.Run("a price without a price point linkage is ignored", func(t *testing.T) {
		unlinked := models.InAppPurchasePrice{ID: "price-unlinked"}

		if got := reconcilePrices(nil, []models.InAppPurchasePrice{unlinked}); len(got) != 0 {
			t.Errorf("got %d prices, want 0", len(got))
		}
	})
}
