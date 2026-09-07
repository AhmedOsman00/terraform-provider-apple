package passtypeid

import (
	"testing"

	"terraform-provider-apple/internal/apple/models"
)

func passTypeIDs() []models.PassTypeIDResource {
	return []models.PassTypeIDResource{
		{ID: "1", Attributes: models.PassTypeIDAttributes{Identifier: "pass.com.example.boarding", Name: "Boarding Pass"}},
		{ID: "2", Attributes: models.PassTypeIDAttributes{Identifier: "pass.com.example.loyalty", Name: "loyalty card"}},
		{ID: "3", Attributes: models.PassTypeIDAttributes{Identifier: "pass.com.other.ticket", Name: "Ticket"}},
	}
}

func idents(list []models.PassTypeIDResource) []string {
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, p.Attributes.Identifier)
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

func TestFilterPassTypeIDs(t *testing.T) {
	tests := []struct {
		name              string
		identifierPattern string
		identifierPrefix  string
		namePattern       string
		want              []string
	}{
		{name: "no filters", want: []string{"pass.com.example.boarding", "pass.com.example.loyalty", "pass.com.other.ticket"}},
		{name: "identifier prefix", identifierPrefix: "pass.com.example", want: []string{"pass.com.example.boarding", "pass.com.example.loyalty"}},
		{name: "identifier regex", identifierPattern: `ticket$`, want: []string{"pass.com.other.ticket"}},
		{name: "name regex is case-sensitive", namePattern: "^Boarding", want: []string{"pass.com.example.boarding"}},
		{name: "name regex misses on case", namePattern: "^Loyalty", want: []string{}},
		{name: "filters combine", identifierPrefix: "pass.com.example", namePattern: "card", want: []string{"pass.com.example.loyalty"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterPassTypeIDs(passTypeIDs(), tt.identifierPattern, tt.identifierPrefix, tt.namePattern)
			if err != nil {
				t.Fatalf("FilterPassTypeIDs() returned error: %v", err)
			}
			if !equal(idents(got), tt.want) {
				t.Errorf("FilterPassTypeIDs() = %v, want %v", idents(got), tt.want)
			}
		})
	}
}

func TestFilterPassTypeIDsRejectsBadPatterns(t *testing.T) {
	for _, tt := range []struct {
		name              string
		identifierPattern string
		namePattern       string
	}{
		{name: "identifier_pattern", identifierPattern: "*invalid("},
		{name: "name_pattern", namePattern: "[a-"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterPassTypeIDs(passTypeIDs(), tt.identifierPattern, "", tt.namePattern)
			if err == nil {
				t.Fatalf("FilterPassTypeIDs() = %v with nil error, want an error naming the bad pattern", idents(got))
			}
			if got != nil {
				t.Errorf("FilterPassTypeIDs() returned %v alongside an error, want nil", idents(got))
			}
		})
	}
}

func TestSortPassTypeIDs(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{"identifier ascending", "identifier", "asc", []string{"pass.com.example.boarding", "pass.com.example.loyalty", "pass.com.other.ticket"}},
		{"identifier descending", "identifier", "desc", []string{"pass.com.other.ticket", "pass.com.example.loyalty", "pass.com.example.boarding"}},
		{"name ascending is case-insensitive", "name", "asc", []string{"pass.com.example.boarding", "pass.com.example.loyalty", "pass.com.other.ticket"}},
		{"empty sortBy defaults to identifier", "", "", []string{"pass.com.example.boarding", "pass.com.example.loyalty", "pass.com.other.ticket"}},
		{"unknown field falls back to identifier", "nonsense", "desc", []string{"pass.com.other.ticket", "pass.com.example.loyalty", "pass.com.example.boarding"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SortPassTypeIDs(passTypeIDs(), tt.sortBy, tt.sortOrder)
			if !equal(idents(got), tt.want) {
				t.Errorf("SortPassTypeIDs(%q, %q) = %v, want %v", tt.sortBy, tt.sortOrder, idents(got), tt.want)
			}
		})
	}
}

func TestLimitPassTypeIDs(t *testing.T) {
	for _, tt := range []struct {
		name  string
		limit int
		want  int
	}{
		{"zero means no limit", 0, 3},
		{"negative means no limit", -1, 3},
		{"limit below length truncates", 2, 2},
		{"limit equal to length", 3, 3},
		{"limit above length", 10, 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := LimitPassTypeIDs(passTypeIDs(), tt.limit); len(got) != tt.want {
				t.Errorf("LimitPassTypeIDs(%d) returned %d items, want %d", tt.limit, len(got), tt.want)
			}
		})
	}
}
