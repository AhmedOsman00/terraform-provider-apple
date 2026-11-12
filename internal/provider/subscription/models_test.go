// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func passes(t *testing.T, validators []validator.String, value string) bool {
	t.Helper()

	req := validator.StringRequest{ConfigValue: types.StringValue(value)}
	resp := &validator.StringResponse{}
	for _, v := range validators {
		v.ValidateString(context.Background(), req, resp)
	}
	return !resp.Diagnostics.HasError()
}

// TestLocalizationLimitsMatchAppStoreConnect pins the subscription
// localization limits to App Store Connect's own form: 35 characters for the
// display name and 55 for the description. A 49-character description that
// Apple stores and the web UI accepts must not be rejected at plan time.
func TestLocalizationLimitsMatchAppStoreConnect(t *testing.T) {
	tests := []struct {
		name       string
		validators []validator.String
		value      string
		want       bool
	}{
		{"name at 35", GetLocalizationNameValidator(), strings.Repeat("a", 35), true},
		{"name at 36", GetLocalizationNameValidator(), strings.Repeat("a", 36), false},
		{"name empty", GetLocalizationNameValidator(), "", false},
		{"description at 49, as Apple stores it", GetLocalizationDescriptionValidator(), "Your own dedicated IP for banking and work logins", true},
		{"description at 55", GetLocalizationDescriptionValidator(), strings.Repeat("a", 55), true},
		{"description at 56", GetLocalizationDescriptionValidator(), strings.Repeat("a", 56), false},
		{"description of 55 multi-byte characters", GetLocalizationDescriptionValidator(), strings.Repeat("ü", 55), true},
		{"description empty", GetLocalizationDescriptionValidator(), "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := passes(t, tt.validators, tt.value); got != tt.want {
				t.Errorf("validation of %d characters = %v, want %v", len([]rune(tt.value)), got, tt.want)
			}
		})
	}
}
