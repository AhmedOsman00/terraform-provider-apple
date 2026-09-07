// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package bundle

import (
	"context"
	"testing"

	"terraform-provider-apple/internal/apple/models"
)

func bundleIDs() []models.BundleID {
	return []models.BundleID{
		{ID: "1", Attributes: models.BundleIDAttributes{Identifier: "com.example.app", Name: "Example App", Platform: models.IOS}},
		{ID: "2", Attributes: models.BundleIDAttributes{Identifier: "com.example.watch", Name: "Example Watch", Platform: models.WATCHOS}},
		{ID: "3", Attributes: models.BundleIDAttributes{Identifier: "com.other.tool", Name: "Other Tool", Platform: models.MACOS}},
	}
}

func bundleIdents(list []models.BundleID) []string {
	out := make([]string, 0, len(list))
	for _, b := range list {
		out = append(out, b.Attributes.Identifier)
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

func TestFilterBundleIDs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		options FilterOptions
		want    []string
	}{
		{name: "no filters", want: []string{"com.example.app", "com.example.watch", "com.other.tool"}},
		{name: "single platform", options: FilterOptions{Platform: "IOS"}, want: []string{"com.example.app"}},
		{name: "multiple platforms", options: FilterOptions{Platforms: []string{"IOS", "MAC_OS"}}, want: []string{"com.example.app", "com.other.tool"}},
		{name: "identifier prefix", options: FilterOptions{IdentifierPrefix: "com.example"}, want: []string{"com.example.app", "com.example.watch"}},
		{name: "identifier regex", options: FilterOptions{IdentifierPattern: `\.watch$`}, want: []string{"com.example.watch"}},
		// name_pattern is glob-matched here, not regex -- a distinction worth pinning.
		{name: "name glob", options: FilterOptions{NamePattern: "Example *"}, want: []string{"com.example.app", "com.example.watch"}},
		{name: "name glob must match the whole string", options: FilterOptions{NamePattern: "Example"}, want: []string{}},
		{name: "limit truncates after filtering", options: FilterOptions{IdentifierPrefix: "com.example", Limit: 1}, want: []string{"com.example.app"}},
		{name: "sort descending by identifier", options: FilterOptions{SortBy: "identifier", SortOrder: "desc"}, want: []string{"com.other.tool", "com.example.watch", "com.example.app"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterBundleIDs(ctx, bundleIDs(), tt.options)
			if err != nil {
				t.Fatalf("FilterBundleIDs() returned error: %v", err)
			}
			if !equalStrings(bundleIdents(got), tt.want) {
				t.Errorf("FilterBundleIDs() = %v, want %v", bundleIdents(got), tt.want)
			}
		})
	}
}

func TestFilterBundleIDsRejectsBadPatterns(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name    string
		options FilterOptions
	}{
		{"identifier_pattern is a regex", FilterOptions{IdentifierPattern: "(unclosed"}},
		{"name_pattern is a glob", FilterOptions{NamePattern: "[unclosed"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterBundleIDs(ctx, bundleIDs(), tt.options)
			if err == nil {
				t.Fatalf("FilterBundleIDs() = %v with nil error, want an error naming the bad pattern", bundleIdents(got))
			}
			if got != nil {
				t.Errorf("FilterBundleIDs() returned %v alongside an error, want nil", bundleIdents(got))
			}
		})
	}
}
