package merchant

import (
	"testing"

	"terraform-provider-apple/internal/apple/models"
)

func merchantIDs() []models.MerchantID {
	return []models.MerchantID{
		{ID: "1", Attributes: models.MerchantIDAttributes{Identifier: "merchant.com.example.store", DisplayName: "Example Store"}},
		{ID: "2", Attributes: models.MerchantIDAttributes{Identifier: "merchant.com.example.premium", DisplayName: "Premium Tier"}},
		{ID: "3", Attributes: models.MerchantIDAttributes{Identifier: "merchant.com.other.shop", DisplayName: "other shop"}},
	}
}

func identifiers(list []models.MerchantID) []string {
	out := make([]string, 0, len(list))
	for _, m := range list {
		out = append(out, m.Attributes.Identifier)
	}
	return out
}

func equal(got, want []string) bool {
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

func TestFilterMerchantIDs(t *testing.T) {
	tests := []struct {
		name    string
		filters MerchantIDFilters
		want    []string
	}{
		{
			name: "no filters returns everything",
			want: []string{"merchant.com.example.store", "merchant.com.example.premium", "merchant.com.other.shop"},
		},
		{
			name:    "identifier prefix",
			filters: MerchantIDFilters{IdentifierPrefix: "merchant.com.example"},
			want:    []string{"merchant.com.example.store", "merchant.com.example.premium"},
		},
		{
			name:    "identifier regex",
			filters: MerchantIDFilters{IdentifierPattern: `\.store$`},
			want:    []string{"merchant.com.example.store"},
		},
		{
			name:    "display name pattern is case-insensitive",
			filters: MerchantIDFilters{DisplayNamePattern: "OTHER"},
			want:    []string{"merchant.com.other.shop"},
		},
		{
			name:    "filters combine",
			filters: MerchantIDFilters{IdentifierPrefix: "merchant.com.example", DisplayNamePattern: "premium"},
			want:    []string{"merchant.com.example.premium"},
		},
		{
			name:    "no matches yields an empty result, not an error",
			filters: MerchantIDFilters{IdentifierPrefix: "merchant.com.nothing"},
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterMerchantIDs(merchantIDs(), tt.filters)
			if err != nil {
				t.Fatalf("FilterMerchantIDs() returned error: %v", err)
			}
			if !equal(identifiers(got), tt.want) {
				t.Errorf("FilterMerchantIDs() = %v, want %v", identifiers(got), tt.want)
			}
		})
	}
}

// TestFilterMerchantIDsRejectsBadPatterns is the regression test for filters
// that used to swallow a compile error and silently return nothing.
func TestFilterMerchantIDsRejectsBadPatterns(t *testing.T) {
	for _, tt := range []struct {
		name    string
		filters MerchantIDFilters
	}{
		{"identifier_pattern", MerchantIDFilters{IdentifierPattern: "["}},
		{"display_name_pattern", MerchantIDFilters{DisplayNamePattern: "("}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterMerchantIDs(merchantIDs(), tt.filters)
			if err == nil {
				t.Fatalf("FilterMerchantIDs() = %v with nil error, want an error naming the bad pattern", identifiers(got))
			}
			if got != nil {
				t.Errorf("FilterMerchantIDs() returned %v alongside an error, want nil", identifiers(got))
			}
		})
	}
}

func TestSortMerchantIDs(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{"identifier ascending", "identifier", "asc", []string{"merchant.com.example.premium", "merchant.com.example.store", "merchant.com.other.shop"}},
		{"identifier descending", "identifier", "desc", []string{"merchant.com.other.shop", "merchant.com.example.store", "merchant.com.example.premium"}},
		{"display name ascending is case-insensitive", "display_name", "asc", []string{"merchant.com.example.store", "merchant.com.other.shop", "merchant.com.example.premium"}},
		{"unknown field falls back to identifier", "nonsense", "asc", []string{"merchant.com.example.premium", "merchant.com.example.store", "merchant.com.other.shop"}},
		{"empty order defaults to ascending", "identifier", "", []string{"merchant.com.example.premium", "merchant.com.example.store", "merchant.com.other.shop"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := merchantIDs()
			SortMerchantIDs(list, tt.sortBy, tt.sortOrder)
			if !equal(identifiers(list), tt.want) {
				t.Errorf("SortMerchantIDs(%q, %q) = %v, want %v", tt.sortBy, tt.sortOrder, identifiers(list), tt.want)
			}
		})
	}
}
