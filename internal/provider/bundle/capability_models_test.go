// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package bundle

import (
	"context"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

// TestSettingValueToString covers every JSON shape Apple can put in a
// capability setting's value. Each non-string case panicked before.
func TestSettingValueToString(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"string passes through", "TEN_GB", "TEN_GB"},
		{"empty string", "", ""},
		{"nil becomes empty", nil, ""},
		{"true", true, "true"},
		{"false", false, "false"},
		{"whole number has no decimal part", float64(25), "25"},
		{"fractional number", 1.5, "1.5"},
		{"negative whole number", float64(-3), "-3"},
		{"object encodes as JSON", map[string]interface{}{"a": "b"}, `{"a":"b"}`},
		{"array encodes as JSON", []interface{}{"a", "b"}, `["a","b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := settingValueToString(tt.value)
			if err != nil {
				t.Fatalf("settingValueToString(%#v) returned error: %v", tt.value, err)
			}
			if got != tt.want {
				t.Errorf("settingValueToString(%#v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestSettingsFromAPINonStringValues is the regression test for the panic: a
// capability whose settings omit "value" or return a non-string must convert.
func TestSettingsFromAPINonStringValues(t *testing.T) {
	apiSettings := []models.CapabilitySetting{
		{Key: "ICLOUD_VERSION", Value: "XCODE_6"},
		{Key: "OMITTED_VALUE", Value: nil},
		{Key: "BOOL_VALUE", Value: true},
		{Key: "NUMERIC_VALUE", Value: float64(10)},
		{Key: "OBJECT_VALUE", Value: map[string]interface{}{"limit": "10GB"}},
	}

	got, err := SettingsFromAPI(apiSettings)
	if err != nil {
		t.Fatalf("SettingsFromAPI() returned error: %v", err)
	}
	if got.IsNull() {
		t.Fatal("SettingsFromAPI() returned a null list, want 5 settings")
	}
	if n := len(got.Elements()); n != 5 {
		t.Fatalf("SettingsFromAPI() produced %d settings, want 5", n)
	}

	// Round-trip back and confirm the rendered values survive.
	back, err := SettingsToAPI(context.Background(), got)
	if err != nil {
		t.Fatalf("SettingsToAPI() returned error: %v", err)
	}

	want := map[string]string{
		"ICLOUD_VERSION": "XCODE_6",
		"OMITTED_VALUE":  "",
		"BOOL_VALUE":     "true",
		"NUMERIC_VALUE":  "10",
		"OBJECT_VALUE":   `{"limit":"10GB"}`,
	}
	if len(back) != len(want) {
		t.Fatalf("SettingsToAPI() produced %d settings, want %d", len(back), len(want))
	}
	for _, s := range back {
		w, ok := want[s.Key]
		if !ok {
			t.Errorf("unexpected setting key %q", s.Key)
			continue
		}
		if s.Value != w {
			t.Errorf("setting %q value = %#v, want %q", s.Key, s.Value, w)
		}
	}
}

// TestSettingsRoundTripPreservesOptionalFields checks visible/min_count/options
// survive both directions, including their null forms.
func TestSettingsRoundTripPreservesOptionalFields(t *testing.T) {
	apiSettings := []models.CapabilitySetting{
		{
			Key:      "ICLOUD_VERSION",
			Name:     "iCloud Version",
			Value:    "XCODE_6",
			Visible:  boolPtr(true),
			MinCount: intPtr(1),
			Options: []models.CapabilitySettingOption{
				{Key: "XCODE_5", Name: "Xcode 5", Description: "Legacy", Enabled: boolPtr(false)},
				{Key: "XCODE_6", Name: "Xcode 6", Enabled: boolPtr(true)},
			},
		},
		{Key: "NO_OPTIONALS", Value: "plain"},
	}

	tfList, err := SettingsFromAPI(apiSettings)
	if err != nil {
		t.Fatalf("SettingsFromAPI() returned error: %v", err)
	}

	back, err := SettingsToAPI(context.Background(), tfList)
	if err != nil {
		t.Fatalf("SettingsToAPI() returned error: %v", err)
	}
	if len(back) != 2 {
		t.Fatalf("SettingsToAPI() produced %d settings, want 2", len(back))
	}

	first := back[0]
	if first.Visible == nil || !*first.Visible {
		t.Errorf("visible = %v, want true", first.Visible)
	}
	if first.MinCount == nil || *first.MinCount != 1 {
		t.Errorf("min_count = %v, want 1", first.MinCount)
	}
	if len(first.Options) != 2 {
		t.Fatalf("options = %d, want 2", len(first.Options))
	}
	if first.Options[0].Key != "XCODE_5" || first.Options[0].Description != "Legacy" {
		t.Errorf("first option = %+v, want key XCODE_5 with description Legacy", first.Options[0])
	}
	if first.Options[0].Enabled == nil || *first.Options[0].Enabled {
		t.Errorf("first option enabled = %v, want false", first.Options[0].Enabled)
	}

	second := back[1]
	if second.Visible != nil {
		t.Errorf("visible = %v, want nil when Apple omitted it", second.Visible)
	}
	if second.MinCount != nil {
		t.Errorf("min_count = %v, want nil when Apple omitted it", second.MinCount)
	}
	if second.Options != nil {
		t.Errorf("options = %v, want nil when Apple omitted them", second.Options)
	}
}

// TestSettingsToAPIEmptyInputs covers the null and unknown lists that reach
// these helpers during plan.
func TestSettingsToAPIEmptyInputs(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		list types.List
	}{
		{"null", types.ListNull(capabilitySettingType)},
		{"unknown", types.ListUnknown(capabilitySettingType)},
		{"empty", types.ListValueMust(capabilitySettingType, nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SettingsToAPI(ctx, tc.list)
			if err != nil {
				t.Fatalf("SettingsToAPI() returned error: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("SettingsToAPI() = %v, want no settings", got)
			}
		})
	}

	if got, err := SettingsFromAPI(nil); err != nil {
		t.Errorf("SettingsFromAPI(nil) returned error: %v", err)
	} else if !got.IsNull() {
		t.Errorf("SettingsFromAPI(nil) = %v, want a null list", got)
	}
}
