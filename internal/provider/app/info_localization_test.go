// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestInfoLocalizationUpdateClearsWhatTheConfigLeavesOut covers adopting a
// localization Apple added to the AppInfo by itself: anything it holds that the
// configuration does not set has to be sent as null, or Apple keeps it and the
// state read back disagrees with the plan.
func TestInfoLocalizationUpdateClearsWhatTheConfigLeavesOut(t *testing.T) {
	t.Parallel()

	plan := appInfoLocalizationModel{
		Name:              types.StringValue("Fenn: Haushaltsbuch"),
		Subtitle:          types.StringValue("Ausgaben privat im Blick"),
		PrivacyPolicyURL:  types.StringNull(),
		PrivacyChoicesURL: types.StringNull(),
		PrivacyPolicyText: types.StringNull(),
	}
	current := appInfoLocalizationModel{
		Name:              types.StringValue("Fenn"),
		Subtitle:          types.StringNull(),
		PrivacyPolicyURL:  types.StringValue("https://example.com/privacy"),
		PrivacyChoicesURL: types.StringNull(),
		PrivacyPolicyText: types.StringNull(),
	}

	body, err := json.Marshal(infoLocalizationUpdate(plan, current))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", body, err)
	}

	want := map[string]any{
		"name":             "Fenn: Haushaltsbuch",
		"subtitle":         "Ausgaben privat im Blick",
		"privacyPolicyUrl": nil,
	}
	if len(got) != len(want) {
		t.Fatalf("body = %s, want exactly %v", body, want)
	}
	for name, value := range want {
		sent, ok := got[name]
		if !ok {
			t.Errorf("%s missing from %s", name, body)
			continue
		}
		if sent != value {
			t.Errorf("%s = %v, want %v", name, sent, value)
		}
	}
}

// TestInfoLocalizationUpdateOmitsWhatIsAlreadyEmpty checks that nothing is
// cleared when Apple holds nothing there either, so an ordinary update sends
// only what the configuration sets.
func TestInfoLocalizationUpdateOmitsWhatIsAlreadyEmpty(t *testing.T) {
	t.Parallel()

	empty := appInfoLocalizationModel{
		Name:              types.StringValue("Fenn"),
		Subtitle:          types.StringNull(),
		PrivacyPolicyURL:  types.StringNull(),
		PrivacyChoicesURL: types.StringNull(),
		PrivacyPolicyText: types.StringNull(),
	}

	body, err := json.Marshal(infoLocalizationUpdate(empty, empty))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(body) != `{"name":"Fenn"}` {
		t.Errorf("body = %s, want only the name", body)
	}
}
