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

// TestSettingsValueMapsToSingleOption pins the shorthand: Apple has no scalar
// value property on a capability setting, so settings.value is sent as a
// one-element options list keyed by the value and read back the same way.
func TestSettingsValueMapsToSingleOption(t *testing.T) {
	tfList, err := SettingsFromAPI([]models.CapabilitySetting{
		{
			Key:     "DATA_PROTECTION_PERMISSION_LEVEL",
			Options: []models.CapabilitySettingOption{{Key: "COMPLETE_PROTECTION"}},
		},
	})
	if err != nil {
		t.Fatalf("SettingsFromAPI() returned error: %v", err)
	}

	var settings []capabilitySettingModel
	if diags := tfList.ElementsAs(context.Background(), &settings, false); diags.HasError() {
		t.Fatalf("reading settings: %s", diagnosticsError(diags))
	}

	if len(settings) != 1 {
		t.Fatalf("got %d settings, want 1", len(settings))
	}

	if got := settings[0].Value.ValueString(); got != "COMPLETE_PROTECTION" {
		t.Errorf("value = %q, want COMPLETE_PROTECTION recovered from the single option", got)
	}

	// And back out again: the wire form must be options, never a value field.
	back, err := SettingsToAPI(context.Background(), tfList)
	if err != nil {
		t.Fatalf("SettingsToAPI() returned error: %v", err)
	}

	if len(back) != 1 || len(back[0].Options) != 1 {
		t.Fatalf("round trip produced %+v, want one setting with one option", back)
	}

	if got := back[0].Options[0].Key; got != "COMPLETE_PROTECTION" {
		t.Errorf("option key = %q, want COMPLETE_PROTECTION", got)
	}
}

// TestSettingsValueAbsentForMultiOption covers the other direction: a setting
// Apple reports with several options has no single value to report, so value is
// null rather than an arbitrary pick.
func TestSettingsValueAbsentForMultiOption(t *testing.T) {
	tfList, err := SettingsFromAPI([]models.CapabilitySetting{
		{
			Key: "ICLOUD_VERSION",
			Options: []models.CapabilitySettingOption{
				{Key: "XCODE_5"},
				{Key: "XCODE_6"},
			},
		},
		{Key: "NO_OPTIONS"},
	})
	if err != nil {
		t.Fatalf("SettingsFromAPI() returned error: %v", err)
	}

	var settings []capabilitySettingModel
	if diags := tfList.ElementsAs(context.Background(), &settings, false); diags.HasError() {
		t.Fatalf("reading settings: %s", diagnosticsError(diags))
	}

	for _, setting := range settings {
		if !setting.Value.IsNull() {
			t.Errorf("setting %q reported value %q, want null", setting.Key.ValueString(), setting.Value.ValueString())
		}
	}
}

// TestSettingsExplicitOptionsWinOverValue documents the precedence rule when a
// configuration supplies both forms.
func TestSettingsExplicitOptionsWinOverValue(t *testing.T) {
	tfList, err := SettingsFromAPI([]models.CapabilitySetting{
		{
			Key:     "ICLOUD_VERSION",
			Options: []models.CapabilitySettingOption{{Key: "XCODE_5"}, {Key: "XCODE_6"}},
		},
	})
	if err != nil {
		t.Fatalf("SettingsFromAPI() returned error: %v", err)
	}

	back, err := SettingsToAPI(context.Background(), tfList)
	if err != nil {
		t.Fatalf("SettingsToAPI() returned error: %v", err)
	}

	if len(back) != 1 || len(back[0].Options) != 2 {
		t.Fatalf("got %+v, want the two explicit options preserved", back)
	}
}

// TestSettingsRoundTripPreservesOptionalFields checks visible/min_count/options
// survive both directions, including their null forms.
func TestSettingsRoundTripPreservesOptionalFields(t *testing.T) {
	apiSettings := []models.CapabilitySetting{
		{
			Key:      "ICLOUD_VERSION",
			Name:     "iCloud Version",
			Visible:  boolPtr(true),
			MinCount: intPtr(1),
			Options: []models.CapabilitySettingOption{
				{Key: "XCODE_5", Name: "Xcode 5", Description: "Legacy", Enabled: boolPtr(false)},
				{Key: "XCODE_6", Name: "Xcode 6", Enabled: boolPtr(true)},
			},
		},
		{Key: "NO_OPTIONALS"},
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
