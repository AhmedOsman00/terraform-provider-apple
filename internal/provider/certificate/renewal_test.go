// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestReadyForRenewal(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	rfc := func(d time.Duration) types.String {
		return types.StringValue(now.Add(d).Format(time.RFC3339))
	}

	tests := []struct {
		name       string
		expiration types.String
		hours      types.Int64
		want       bool
		wantErr    bool
	}{
		{
			name:       "disabled when no window configured",
			expiration: rfc(time.Hour), // expires in an hour
			hours:      types.Int64Null(),
			want:       false,
		},
		{
			name:       "disabled when window is zero",
			expiration: rfc(time.Hour),
			hours:      types.Int64Value(0),
			want:       false,
		},
		{
			name:       "outside the window",
			expiration: rfc(90 * 24 * time.Hour), // 90 days out
			hours:      types.Int64Value(720),    // renew 30 days early
			want:       false,
		},
		{
			name:       "inside the window",
			expiration: rfc(10 * 24 * time.Hour), // 10 days out
			hours:      types.Int64Value(720),    // renew 30 days early
			want:       true,
		},
		{
			name:       "already expired",
			expiration: rfc(-24 * time.Hour),
			hours:      types.Int64Value(720),
			want:       true,
		},
		{
			name:       "exactly at the boundary is not yet due",
			expiration: rfc(720 * time.Hour),
			hours:      types.Int64Value(720),
			want:       false,
		},
		{
			name:       "no expiration reported by Apple",
			expiration: types.StringNull(),
			hours:      types.Int64Value(720),
			want:       false,
		},
		{
			name:       "unparseable expiration is an error",
			expiration: types.StringValue("not-a-date"),
			hours:      types.Int64Value(720),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readyForRenewal(tt.expiration, tt.hours, now)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("readyForRenewal() = %v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("readyForRenewal() returned unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("readyForRenewal() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestReadyForRenewalWindowLargerThanValidity documents the footgun called out
// in the schema: a window wider than the certificate's life keeps it
// permanently due for renewal.
func TestReadyForRenewalWindowLargerThanValidity(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	freshlyIssued := types.StringValue(now.Add(365 * 24 * time.Hour).Format(time.RFC3339))

	got, err := readyForRenewal(freshlyIssued, types.Int64Value(400*24), now)
	if err != nil {
		t.Fatalf("readyForRenewal() returned unexpected error: %v", err)
	}
	if !got {
		t.Error("readyForRenewal() = false, want true: a window wider than the validity period always triggers")
	}
}
