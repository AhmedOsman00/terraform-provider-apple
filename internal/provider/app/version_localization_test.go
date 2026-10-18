// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestVersionLocalizationUpdateClearsWhatTheConfigLeavesOut covers adopting
// the copy Apple starts a new version with: anything it carried over that the
// configuration does not set has to be sent as null, or Apple keeps it and the
// state read back disagrees with the plan.
func TestVersionLocalizationUpdateClearsWhatTheConfigLeavesOut(t *testing.T) {
	t.Parallel()

	plan := appStoreVersionLocalizationModel{
		Description:     types.StringValue("New description"),
		Keywords:        types.ListNull(types.StringType),
		PromotionalText: types.StringNull(),
		MarketingURL:    types.StringNull(),
		SupportURL:      types.StringValue("https://example.com/support"),
		WhatsNew:        types.StringNull(),
	}
	current := appStoreVersionLocalizationModel{
		Description:     types.StringValue("Old description"),
		Keywords:        types.ListValueMust(types.StringType, nil),
		PromotionalText: types.StringValue("Carried over"),
		MarketingURL:    types.StringNull(),
		SupportURL:      types.StringValue("https://example.com/support"),
		WhatsNew:        types.StringValue("Last release"),
	}

	body, err := json.Marshal(versionLocalizationUpdate(plan, current))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", body, err)
	}

	want := map[string]any{
		"description":     "New description",
		"keywords":        nil,
		"promotionalText": nil,
		"supportUrl":      "https://example.com/support",
		"whatsNew":        nil,
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

// TestVersionLocalizationUpdateOmitsWhatIsAlreadyEmpty checks that nothing is
// cleared when Apple holds nothing there either, so an ordinary update sends
// only what the configuration sets.
func TestVersionLocalizationUpdateOmitsWhatIsAlreadyEmpty(t *testing.T) {
	t.Parallel()

	empty := appStoreVersionLocalizationModel{
		Description:     types.StringNull(),
		Keywords:        types.ListNull(types.StringType),
		PromotionalText: types.StringNull(),
		MarketingURL:    types.StringNull(),
		SupportURL:      types.StringNull(),
		WhatsNew:        types.StringNull(),
	}
	plan := empty
	plan.Description = types.StringValue("Only this")

	body, err := json.Marshal(versionLocalizationUpdate(plan, empty))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(body) != `{"description":"Only this"}` {
		t.Errorf("body = %s, want only the description", body)
	}
}
