// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package territory

import (
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

func territories() []models.Territory {
	return []models.Territory{
		{ID: "USA", Attributes: models.TerritoryAttributes{Currency: "USD"}},
		{ID: "GBR", Attributes: models.TerritoryAttributes{Currency: "GBP"}},
		{ID: "EGY", Attributes: models.TerritoryAttributes{Currency: "EGP"}},
		{ID: "DEU", Attributes: models.TerritoryAttributes{Currency: "EUR"}},
		{ID: "ARE", Attributes: models.TerritoryAttributes{Currency: "AED"}},
		{ID: "AUT", Attributes: models.TerritoryAttributes{Currency: "EUR"}},
	}
}

func ids(list []models.Territory) []string {
	out := make([]string, 0, len(list))
	for _, t := range list {
		out = append(out, t.ID)
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

func TestFilterTerritories(t *testing.T) {
	tests := []struct {
		name    string
		filters TerritoryFilters
		want    []string
	}{
		{
			name: "no filters returns everything",
			want: []string{"USA", "GBR", "EGY", "DEU", "ARE", "AUT"},
		},
		{
			name:    "currency is exact",
			filters: TerritoryFilters{Currency: "EUR"},
			want:    []string{"DEU", "AUT"},
		},
		{
			name:    "currency is case-insensitive",
			filters: TerritoryFilters{Currency: "gbp"},
			want:    []string{"GBR"},
		},
		{
			name:    "id pattern matches as a substring",
			filters: TerritoryFilters{IDPattern: "A"},
			want:    []string{"USA", "ARE", "AUT"},
		},
		{
			name:    "id pattern can be anchored",
			filters: TerritoryFilters{IDPattern: "^A"},
			want:    []string{"ARE", "AUT"},
		},
		{
			name:    "filters combine",
			filters: TerritoryFilters{IDPattern: "^A", Currency: "EUR"},
			want:    []string{"AUT"},
		},
		{
			name:    "no matches yields an empty result",
			filters: TerritoryFilters{Currency: "XYZ"},
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterTerritories(territories(), tt.filters)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalStrings(ids(got), tt.want) {
				t.Errorf("got %v, want %v", ids(got), tt.want)
			}
		})
	}
}

func TestFilterTerritoriesRejectsMalformedPattern(t *testing.T) {
	if _, err := FilterTerritories(territories(), TerritoryFilters{IDPattern: "("}); err == nil {
		t.Fatal("expected an error for a malformed pattern, got nil")
	}
}

func TestSortTerritories(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{
			name: "defaults to id ascending",
			want: []string{"ARE", "AUT", "DEU", "EGY", "GBR", "USA"},
		},
		{
			name:      "id descending",
			sortOrder: "desc",
			want:      []string{"USA", "GBR", "EGY", "DEU", "AUT", "ARE"},
		},
		{
			name:   "currency, with a shared currency broken by id",
			sortBy: "currency",
			want:   []string{"ARE", "EGY", "AUT", "DEU", "GBR", "USA"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := territories()
			SortTerritories(list, tt.sortBy, tt.sortOrder)
			if !equalStrings(ids(list), tt.want) {
				t.Errorf("got %v, want %v", ids(list), tt.want)
			}
		})
	}
}
