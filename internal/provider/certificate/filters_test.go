// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"testing"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

func platformPtr(p models.BundleIDPlatform) *models.BundleIDPlatform { return &p }
func timePtr(t time.Time) *time.Time                                 { return &t }

func certificates() []models.Certificate {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return []models.Certificate{
		{ID: "1", Attributes: models.CertificateAttributes{
			SerialNumber: "AAA1", DisplayName: "Dev Cert", Name: "iPhone Developer",
			CertificateType: "IOS_DEVELOPMENT", Platform: platformPtr(models.IOS),
			ExpirationDate: timePtr(base.AddDate(0, 6, 0)),
		}},
		{ID: "2", Attributes: models.CertificateAttributes{
			SerialNumber: "BBB2", DisplayName: "Dist Cert", Name: "iPhone Distribution",
			CertificateType: "IOS_DISTRIBUTION", Platform: platformPtr(models.IOS),
			ExpirationDate: timePtr(base.AddDate(1, 0, 0)),
		}},
		{ID: "3", Attributes: models.CertificateAttributes{
			SerialNumber: "CCC3", DisplayName: "Mac Cert", Name: "Mac Developer",
			CertificateType: "MAC_APP_DEVELOPMENT", Platform: platformPtr(models.MACOS),
			// No expiration date: Apple does not always report one.
		}},
	}
}

func serials(list []models.Certificate) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		out = append(out, c.Attributes.SerialNumber)
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

func TestFilterCertificates(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		options FilterOptions
		want    []string
	}{
		{name: "no filters", want: []string{"AAA1", "BBB2", "CCC3"}},
		{name: "single type", options: FilterOptions{CertificateType: "IOS_DISTRIBUTION"}, want: []string{"BBB2"}},
		{name: "multiple types", options: FilterOptions{CertificateTypes: []string{"IOS_DEVELOPMENT", "MAC_APP_DEVELOPMENT"}}, want: []string{"AAA1", "CCC3"}},
		{name: "platform", options: FilterOptions{Platform: "MAC_OS"}, want: []string{"CCC3"}},
		{name: "exact serial number", options: FilterOptions{SerialNumber: "BBB2"}, want: []string{"BBB2"}},
		// The glob is matched against display name OR name, so this hits via name.
		{name: "name glob matches either name field", options: FilterOptions{NamePattern: "iPhone*"}, want: []string{"AAA1", "BBB2"}},
		{name: "name glob matches display name", options: FilterOptions{NamePattern: "Mac Cert"}, want: []string{"CCC3"}},
		{name: "limit", options: FilterOptions{Limit: 2}, want: []string{"AAA1", "BBB2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterCertificates(ctx, certificates(), tt.options)
			if err != nil {
				t.Fatalf("FilterCertificates() returned error: %v", err)
			}
			if !equalStrings(serials(got), tt.want) {
				t.Errorf("FilterCertificates() = %v, want %v", serials(got), tt.want)
			}
		})
	}
}

func TestFilterCertificatesRejectsBadNamePattern(t *testing.T) {
	got, err := FilterCertificates(context.Background(), certificates(), FilterOptions{NamePattern: "[unclosed"})
	if err == nil {
		t.Fatalf("FilterCertificates() = %v with nil error, want an error naming the bad glob", serials(got))
	}
	if got != nil {
		t.Errorf("FilterCertificates() returned %v alongside an error, want nil", serials(got))
	}
}

// TestSortCertificatesByExpiration pins the nil handling: Apple omits
// expirationDate for some certificates and those must not sort first.
func TestSortCertificatesByExpiration(t *testing.T) {
	ctx := context.Background()

	got, err := FilterCertificates(ctx, certificates(), FilterOptions{SortBy: "expiration_date", SortOrder: "asc"})
	if err != nil {
		t.Fatalf("FilterCertificates() returned error: %v", err)
	}
	if want := []string{"AAA1", "BBB2", "CCC3"}; !equalStrings(serials(got), want) {
		t.Errorf("ascending by expiration = %v, want %v (undated last)", serials(got), want)
	}
}

func TestSortCertificates(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{"display name ascending", "display_name", "asc", []string{"AAA1", "BBB2", "CCC3"}},
		{"display name descending", "display_name", "desc", []string{"CCC3", "BBB2", "AAA1"}},
		{"serial ascending", "serial_number", "asc", []string{"AAA1", "BBB2", "CCC3"}},
		{"certificate type ascending", "certificate_type", "asc", []string{"AAA1", "BBB2", "CCC3"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterCertificates(ctx, certificates(), FilterOptions{SortBy: tt.sortBy, SortOrder: tt.sortOrder})
			if err != nil {
				t.Fatalf("FilterCertificates() returned error: %v", err)
			}
			if !equalStrings(serials(got), tt.want) {
				t.Errorf("sort by %q %q = %v, want %v", tt.sortBy, tt.sortOrder, serials(got), tt.want)
			}
		})
	}
}
