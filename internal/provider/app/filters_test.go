// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

func apps() []models.App {
	return []models.App{
		{ID: "100", Attributes: models.AppAttributes{Name: "Fenn", BundleID: "com.aostudio.fenn", SKU: "FENN-1"}},
		{ID: "200", Attributes: models.AppAttributes{Name: "fenn beta", BundleID: "com.aostudio.fenn.beta", SKU: "FENN-2"}},
		{ID: "300", Attributes: models.AppAttributes{Name: "Other", BundleID: "com.example.other", SKU: "OTHER-1"}},
	}
}

func names(list []models.App) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.Attributes.Name)
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

func TestFilterApps(t *testing.T) {
	tests := []struct {
		name    string
		filters AppFilters
		want    []string
	}{
		{
			name: "no filters returns everything",
			want: []string{"Fenn", "fenn beta", "Other"},
		},
		{
			name:    "bundle id is exact, not a prefix",
			filters: AppFilters{BundleID: "com.aostudio.fenn"},
			want:    []string{"Fenn"},
		},
		{
			name:    "sku is exact",
			filters: AppFilters{SKU: "FENN-2"},
			want:    []string{"fenn beta"},
		},
		{
			name:    "name pattern matches as substring and is case-sensitive",
			filters: AppFilters{NamePattern: "fenn"},
			want:    []string{"fenn beta"},
		},
		{
			name:    "name pattern accepts an inline case-insensitive flag",
			filters: AppFilters{NamePattern: "(?i)fenn"},
			want:    []string{"Fenn", "fenn beta"},
		},
		{
			name:    "filters combine",
			filters: AppFilters{NamePattern: "(?i)fenn", SKU: "FENN-1"},
			want:    []string{"Fenn"},
		},
		{
			name:    "no matches yields an empty result",
			filters: AppFilters{BundleID: "com.nope.missing"},
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterApps(apps(), tt.filters)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalStrings(names(got), tt.want) {
				t.Errorf("got %v, want %v", names(got), tt.want)
			}
		})
	}
}

func TestFilterAppsRejectsMalformedPattern(t *testing.T) {
	if _, err := FilterApps(apps(), AppFilters{NamePattern: "("}); err == nil {
		t.Fatal("expected an error for a malformed pattern, got nil")
	}
}

func TestSortApps(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{
			name: "defaults to name ascending, case-insensitively",
			want: []string{"Fenn", "fenn beta", "Other"},
		},
		{
			name:      "name descending",
			sortOrder: "desc",
			want:      []string{"Other", "fenn beta", "Fenn"},
		},
		{
			name:   "bundle id",
			sortBy: "bundle_id",
			want:   []string{"Fenn", "fenn beta", "Other"},
		},
		{
			name:   "sku",
			sortBy: "sku",
			want:   []string{"Fenn", "fenn beta", "Other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := apps()
			SortApps(list, tt.sortBy, tt.sortOrder)
			if !equalStrings(names(list), tt.want) {
				t.Errorf("got %v, want %v", names(list), tt.want)
			}
		})
	}
}
