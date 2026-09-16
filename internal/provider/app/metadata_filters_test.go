// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ptr[T any](v T) *T {
	return &v
}

func categories() []models.AppCategory {
	return []models.AppCategory{
		{
			ID:         "PRODUCTIVITY",
			Attributes: models.AppCategoryAttributes{Platforms: []string{"IOS", "MAC_OS"}},
		},
		{
			ID:         "FINANCE",
			Attributes: models.AppCategoryAttributes{Platforms: []string{"IOS"}},
		},
		{
			ID:         "GAMES",
			Attributes: models.AppCategoryAttributes{Platforms: []string{"IOS", "TV_OS"}},
			Relationships: &models.AppCategoryRelationships{
				Subcategories: &models.ResourceIdentifiers{
					Data: []models.ResourceData{{Type: "appCategories", ID: "GAMES_PUZZLE"}},
				},
			},
		},
		{
			ID:         "GAMES_PUZZLE",
			Attributes: models.AppCategoryAttributes{Platforms: []string{"IOS"}},
			Relationships: &models.AppCategoryRelationships{
				Parent: &models.ResourceIdentifier{
					Data: models.ResourceData{Type: "appCategories", ID: "GAMES"},
				},
			},
		},
	}
}

func categoryIDs(list []models.AppCategory) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		out = append(out, c.ID)
	}

	return out
}

func TestFilterAppCategories(t *testing.T) {
	tests := []struct {
		name    string
		filters AppCategoryFilters
		want    []string
		wantErr bool
	}{
		{
			name:    "no filters returns everything",
			filters: AppCategoryFilters{},
			want:    []string{"PRODUCTIVITY", "FINANCE", "GAMES", "GAMES_PUZZLE"},
		},
		{
			name:    "top level only drops the subcategory",
			filters: AppCategoryFilters{TopLevelOnly: true},
			want:    []string{"PRODUCTIVITY", "FINANCE", "GAMES"},
		},
		{
			name:    "id pattern matches as a substring",
			filters: AppCategoryFilters{IDPattern: "GAMES"},
			want:    []string{"GAMES", "GAMES_PUZZLE"},
		},
		{
			name:    "anchored id pattern",
			filters: AppCategoryFilters{IDPattern: "^GAMES$"},
			want:    []string{"GAMES"},
		},
		{
			name:    "combined filters",
			filters: AppCategoryFilters{TopLevelOnly: true, IDPattern: "GAMES"},
			want:    []string{"GAMES"},
		},
		{
			name:    "no match",
			filters: AppCategoryFilters{IDPattern: "NOPE"},
			want:    []string{},
		},
		{
			name:    "malformed pattern errors",
			filters: AppCategoryFilters{IDPattern: "[unclosed"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FilterAppCategories(categories(), tc.filters)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !equalStrings(categoryIDs(got), tc.want) {
				t.Errorf("got %v, want %v", categoryIDs(got), tc.want)
			}
		})
	}
}

func TestSortAppCategories(t *testing.T) {
	list := categories()
	SortAppCategories(list)

	want := []string{"FINANCE", "GAMES", "GAMES_PUZZLE", "PRODUCTIVITY"}
	if !equalStrings(categoryIDs(list), want) {
		t.Errorf("got %v, want %v", categoryIDs(list), want)
	}
}

func pricePoints() []models.AppPricePoint {
	return []models.AppPricePoint{
		{ID: "a", Attributes: models.AppPricePointAttributes{CustomerPrice: "0", Proceeds: "0"}},
		{ID: "b", Attributes: models.AppPricePointAttributes{CustomerPrice: "9.99", Proceeds: "6.99"}},
		{ID: "c", Attributes: models.AppPricePointAttributes{CustomerPrice: "9.99", Proceeds: "7.00"}},
	}
}

func pricePointIDs(list []models.AppPricePoint) []string {
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, p.ID)
	}

	return out
}

func TestFilterAppPricePoints(t *testing.T) {
	tests := []struct {
		name    string
		filters AppPricePointFilters
		want    []string
	}{
		{
			name:    "no filter returns everything",
			filters: AppPricePointFilters{},
			want:    []string{"a", "b", "c"},
		},
		{
			name:    "exact customer price",
			filters: AppPricePointFilters{CustomerPrice: "9.99"},
			want:    []string{"b", "c"},
		},
		{
			name:    "free price point",
			filters: AppPricePointFilters{CustomerPrice: "0"},
			want:    []string{"a"},
		},
		{
			// The comparison is a string match, not a numeric one: Apple reports
			// prices as decimal strings and "9.990" is simply not a price it
			// published.
			name:    "trailing zero does not match",
			filters: AppPricePointFilters{CustomerPrice: "9.990"},
			want:    []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterAppPricePoints(pricePoints(), tc.filters)
			if !equalStrings(pricePointIDs(got), tc.want) {
				t.Errorf("got %v, want %v", pricePointIDs(got), tc.want)
			}
		})
	}
}

func versions() []models.AppStoreVersion {
	return []models.AppStoreVersion{
		{
			ID: "1",
			Attributes: models.AppStoreVersionAttributes{
				Platform:        "IOS",
				VersionString:   "1.0",
				AppVersionState: ptr("READY_FOR_DISTRIBUTION"),
				CreatedDate:     ptr("2026-01-01T00:00:00Z"),
			},
		},
		{
			ID: "2",
			Attributes: models.AppStoreVersionAttributes{
				Platform:        "IOS",
				VersionString:   "2.0",
				AppVersionState: ptr("PREPARE_FOR_SUBMISSION"),
				CreatedDate:     ptr("2026-06-01T00:00:00Z"),
			},
		},
		{
			ID: "3",
			Attributes: models.AppStoreVersionAttributes{
				Platform:      "MAC_OS",
				VersionString: "1.5",
				CreatedDate:   ptr("2026-03-01T00:00:00Z"),
			},
		},
	}
}

func versionStrings(list []models.AppStoreVersion) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		out = append(out, v.Attributes.VersionString)
	}

	return out
}

func TestFilterAppStoreVersions(t *testing.T) {
	tests := []struct {
		name    string
		filters AppStoreVersionFilters
		want    []string
		wantErr bool
	}{
		{
			name:    "no filters returns everything",
			filters: AppStoreVersionFilters{},
			want:    []string{"1.0", "2.0", "1.5"},
		},
		{
			name:    "state matches case-insensitively",
			filters: AppStoreVersionFilters{AppVersionState: "prepare_for_submission"},
			want:    []string{"2.0"},
		},
		{
			// A version Apple reported no state for must not match a state
			// filter, rather than being treated as an empty string that matches
			// everything.
			name:    "missing state never matches",
			filters: AppStoreVersionFilters{AppVersionState: "READY_FOR_DISTRIBUTION"},
			want:    []string{"1.0"},
		},
		{
			name:    "version pattern matches as a substring",
			filters: AppStoreVersionFilters{VersionStringPattern: "1"},
			want:    []string{"1.0", "1.5"},
		},
		{
			name:    "anchored version pattern",
			filters: AppStoreVersionFilters{VersionStringPattern: `^2\.`},
			want:    []string{"2.0"},
		},
		{
			name:    "malformed pattern errors",
			filters: AppStoreVersionFilters{VersionStringPattern: "[unclosed"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FilterAppStoreVersions(versions(), tc.filters)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !equalStrings(versionStrings(got), tc.want) {
				t.Errorf("got %v, want %v", versionStrings(got), tc.want)
			}
		})
	}
}

func TestSortAppStoreVersions(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{
			name:   "unset leaves the order alone",
			sortBy: "",
			want:   []string{"1.0", "2.0", "1.5"},
		},
		{
			name:   "version string ascending",
			sortBy: "version_string",
			want:   []string{"1.0", "1.5", "2.0"},
		},
		{
			name:      "created date descending is newest first",
			sortBy:    "created_date",
			sortOrder: "desc",
			want:      []string{"2.0", "1.5", "1.0"},
		},
		{
			name:   "platform ascending",
			sortBy: "platform",
			want:   []string{"1.0", "2.0", "1.5"},
		},
		{
			name:   "unknown field leaves the order alone",
			sortBy: "nonsense",
			want:   []string{"1.0", "2.0", "1.5"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			list := versions()
			SortAppStoreVersions(list, tc.sortBy, tc.sortOrder)

			if !equalStrings(versionStrings(list), tc.want) {
				t.Errorf("got %v, want %v", versionStrings(list), tc.want)
			}
		})
	}
}

func TestLimitAppStoreVersions(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "zero means no limit", limit: 0, want: 3},
		{name: "negative means no limit", limit: -1, want: 3},
		{name: "limit below the count truncates", limit: 2, want: 2},
		{name: "limit above the count is a no-op", limit: 10, want: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(LimitAppStoreVersions(versions(), tc.limit)); got != tc.want {
				t.Errorf("got %d versions, want %d", got, tc.want)
			}
		})
	}
}

// keywordList builds the list attribute the keyword helpers take.
func keywordList(values ...string) types.List {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.StringValue(value))
	}

	list, diags := types.ListValue(types.StringType, elements)
	if diags.HasError() {
		panic(diags)
	}

	return list
}

// TestJoinKeywords covers the list-to-string conversion Apple's keyword
// attribute forces.
func TestJoinKeywords(t *testing.T) {
	mixed, diags := types.ListValue(types.StringType, []attr.Value{
		types.StringValue("budget"),
		types.StringNull(),
		types.StringUnknown(),
		types.StringValue("   "),
		types.StringValue("money"),
	})
	if diags.HasError() {
		t.Fatalf("building the mixed list: %+v", diags)
	}

	tests := []struct {
		name     string
		keywords types.List
		want     string
	}{
		{
			// The whole list is unknown when keywords come from a variable or a
			// for_each. A Go slice cannot represent that, which is why the
			// helper takes a types.List -- decoding into []types.String failed
			// with "Received unknown value, however the target type cannot
			// handle unknown values".
			name:     "unknown list",
			keywords: types.ListUnknown(types.StringType),
			want:     "",
		},
		{name: "null list", keywords: types.ListNull(types.StringType), want: ""},
		{name: "empty list", keywords: keywordList(), want: ""},
		{
			name:     "joined with no space",
			keywords: keywordList("budget", "expenses"),
			want:     "budget,expenses",
		},
		{
			// A space after the comma is a character, and every one of them
			// comes out of the hundred a locale gets.
			name:     "surrounding whitespace is trimmed",
			keywords: keywordList("  budget ", "expenses"),
			want:     "budget,expenses",
		},
		{
			name:     "null, unknown and blank elements are dropped",
			keywords: mixed,
			want:     "budget,money",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinKeywords(tc.keywords); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSplitKeywords(t *testing.T) {
	tests := []struct {
		name     string
		keywords *string
		wantNull bool
		want     []string
	}{
		{name: "nil is a null list", keywords: nil, wantNull: true},
		{name: "empty string is a null list", keywords: ptr(""), wantNull: true},
		{name: "whitespace only is a null list", keywords: ptr("   "), wantNull: true},
		{name: "commas only is a null list", keywords: ptr(",,,"), wantNull: true},
		{name: "plain list", keywords: ptr("budget,expenses"), want: []string{"budget", "expenses"}},
		{
			// App Store Connect's own editor writes them back with spaces.
			name:     "spaces after commas are trimmed",
			keywords: ptr("budget, expenses, money"),
			want:     []string{"budget", "expenses", "money"},
		},
		{
			name:     "empty elements are dropped",
			keywords: ptr("budget,,expenses,"),
			want:     []string{"budget", "expenses"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitKeywords(tc.keywords)

			if tc.wantNull {
				if !got.IsNull() {
					t.Errorf("got %v, want a null list", got)
				}

				return
			}

			elements := got.Elements()
			if len(elements) != len(tc.want) {
				t.Fatalf("got %d keywords, want %d: %v", len(elements), len(tc.want), elements)
			}
			for i := range elements {
				keyword, ok := elements[i].(types.String)
				if !ok {
					t.Fatalf("element %d is %T, want types.String", i, elements[i])
				}
				if value := keyword.ValueString(); value != tc.want[i] {
					t.Errorf("keyword %d = %q, want %q", i, value, tc.want[i])
				}
			}
		})
	}
}

// TestKeywordsRoundTrip checks that splitting what joinKeywords produced gives
// the list back, since the two run against each other on every plan.
func TestKeywordsRoundTrip(t *testing.T) {
	original := keywordList("budget", "expenses", "ميزانية")

	joined := joinKeywords(original)
	got := splitKeywords(&joined)

	if !got.Equal(original) {
		t.Errorf("round trip produced %v, want %v", got, original)
	}
}

// TestStringListValues covers the filter-attribute reader the data sources use.
func TestStringListValues(t *testing.T) {
	tests := []struct {
		name string
		list types.List
		want []string
	}{
		{name: "null list", list: types.ListNull(types.StringType), want: nil},
		{name: "unknown list", list: types.ListUnknown(types.StringType), want: nil},
		{name: "empty list", list: keywordList(), want: []string{}},
		{name: "values", list: keywordList("USA", "GBR"), want: []string{"USA", "GBR"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stringListValues(tc.list)

			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("element %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestReconcileAppPrices covers the rule that a price already in state keeps
// its configured dates.
func TestReconcileAppPrices(t *testing.T) {
	reported := []models.AppPrice{
		{
			ID:         "p1",
			Attributes: models.AppPriceAttributes{StartDate: ptr("2026-01-01"), EndDate: ptr("2026-06-30")},
			Relationships: &models.AppPriceRelationships{
				AppPricePoint: &models.ResourceIdentifier{Data: models.ResourceData{ID: "point-a"}},
			},
		},
		{
			ID:         "p2",
			Attributes: models.AppPriceAttributes{StartDate: ptr("2026-07-01")},
			Relationships: &models.AppPriceRelationships{
				AppPricePoint: &models.ResourceIdentifier{Data: models.ResourceData{ID: "point-b"}},
			},
		},
	}

	t.Run("configured dates survive Apple's derived end date", func(t *testing.T) {
		state := []appPriceModel{{
			PricePointID: types.StringValue("point-a"),
			StartDate:    types.StringValue("2026-01-01"),
			EndDate:      types.StringNull(),
		}}

		got := reconcileAppPrices(state, reported)

		if len(got) != 2 {
			t.Fatalf("got %d prices, want 2", len(got))
		}
		if !got[0].EndDate.IsNull() {
			t.Errorf("end date = %q, want null: Apple's derived date must not be adopted", got[0].EndDate.ValueString())
		}
	})

	t.Run("a price gone from Apple is dropped", func(t *testing.T) {
		state := []appPriceModel{{PricePointID: types.StringValue("point-gone")}}

		got := reconcileAppPrices(state, reported)

		for _, price := range got {
			if price.PricePointID.ValueString() == "point-gone" {
				t.Error("a price Apple no longer reports should not survive the read")
			}
		}
	})

	t.Run("an import adopts Apple's dates", func(t *testing.T) {
		got := reconcileAppPrices(nil, reported)

		if len(got) != 2 {
			t.Fatalf("got %d prices, want 2", len(got))
		}
		for _, price := range got {
			if price.PricePointID.ValueString() == "point-a" && price.EndDate.ValueString() != "2026-06-30" {
				t.Errorf("end date = %q, want 2026-06-30", price.EndDate.ValueString())
			}
		}
	})
}

// TestReconcileTerritories covers the availability equivalent, including the
// rule that a storefront Apple reports as unavailable is not in the list.
func TestReconcileTerritories(t *testing.T) {
	territory := func(code string, available bool, releaseDate *string) models.TerritoryAvailability {
		return models.TerritoryAvailability{
			Attributes: models.TerritoryAvailabilityAttributes{
				Available:   &available,
				ReleaseDate: releaseDate,
			},
			Relationships: &models.TerritoryAvailabilityRelationships{
				Territory: &models.ResourceIdentifier{Data: models.ResourceData{ID: code}},
			},
		}
	}

	reported := []models.TerritoryAvailability{
		territory("USA", true, ptr("2026-03-01")),
		territory("GBR", true, nil),
		territory("EGY", false, nil),
	}

	t.Run("unavailable storefronts are dropped", func(t *testing.T) {
		got := reconcileTerritories(nil, reported)

		for _, entry := range got {
			if entry.Territory.ValueString() == "EGY" {
				t.Error("a storefront Apple reports as unavailable should not be in the list")
			}
		}
		if len(got) != 2 {
			t.Errorf("got %d territories, want 2", len(got))
		}
	})

	t.Run("configured release date survives", func(t *testing.T) {
		state := []appTerritoryAvailabilityModel{{
			Territory:   types.StringValue("USA"),
			ReleaseDate: types.StringNull(),
		}}

		got := reconcileTerritories(state, reported)

		if got[0].Territory.ValueString() != "USA" {
			t.Fatalf("first entry = %q, want USA", got[0].Territory.ValueString())
		}
		if !got[0].ReleaseDate.IsNull() {
			t.Errorf("release date = %q, want null: Apple's own date must not be adopted",
				got[0].ReleaseDate.ValueString())
		}
	})

	t.Run("additions land in a stable order", func(t *testing.T) {
		first := reconcileTerritories(nil, reported)
		for i := 0; i < 10; i++ {
			again := reconcileTerritories(nil, reported)
			for j := range first {
				if first[j].Territory.ValueString() != again[j].Territory.ValueString() {
					t.Fatal("territory order is not stable across reads")
				}
			}
		}
	})
}
