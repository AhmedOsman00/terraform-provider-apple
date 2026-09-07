// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package device

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// validateUDID runs the full set a device resource applies to `udid`, not just
// the regex, so the length bounds are covered too.
func validateUDID(t *testing.T, udid string) bool {
	t.Helper()

	req := validator.StringRequest{ConfigValue: types.StringValue(udid)}
	resp := &validator.StringResponse{}
	for _, v := range GetUDIDValidator() {
		v.ValidateString(context.Background(), req, resp)
	}
	return !resp.Diagnostics.HasError()
}

// TestUDIDValidatorAcceptsEveryShapeAppleIssues covers the three formats in
// circulation. The 8-16 hex form is the one every A12 and later device reports,
// so rejecting it would turn away most of the devices a team registers.
func TestUDIDValidatorAcceptsEveryShapeAppleIssues(t *testing.T) {
	tests := []struct {
		name string
		udid string
	}{
		{"40 hex, iPhone X and earlier", "00008030000a4d8e0ab8802e1234567890abcdef"},
		{"40 hex uppercase", "00008030000A4D8E0AB8802E1234567890ABCDEF"},
		{"8-16 hex, iPhone XS and later", "00008030-000A4D8E0AB8802E"},
		{"8-16 hex lowercase", "00008130-000a1b2c3d4e5f60"},
		{"UUID, Mac and Apple TV", "550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !validateUDID(t, tt.udid) {
				t.Errorf("UDIDValidator rejected %q, want it accepted", tt.udid)
			}
		})
	}
}

// TestUDIDValidatorAcceptsUnknownShapes is the point of the loose rule: Apple
// owns this format and has changed it before, so a shape we have never seen has
// to reach the API rather than be turned away by a stale guess here.
func TestUDIDValidatorAcceptsUnknownShapes(t *testing.T) {
	tests := []struct {
		name string
		udid string
	}{
		{"an unfamiliar split", "0000803-0000A4D8E0AB8802E"},
		{"more groups than a UUID", "00008030-000a-4d8e-0ab8-802e-1234"},
		{"no dashes at an unfamiliar length", "00008030000a4d8e0ab8802e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !validateUDID(t, tt.udid) {
				t.Errorf("UDIDValidator rejected %q; unrecognized shapes belong to Apple to reject", tt.udid)
			}
		})
	}
}

// TestUDIDValidatorRejectsNonUDIDs covers what the rule is actually for:
// values that cannot be a UDID under any format Apple might ship.
func TestUDIDValidatorRejectsNonUDIDs(t *testing.T) {
	tests := []struct {
		name string
		udid string
	}{
		{"empty", ""},
		{"placeholder text", "existing-device-udid-here"},
		{"a file path", "/Users/me/devices/udid.txt"},
		{"non-hex letter in an otherwise valid 40 hex", "00008030000a4d8e0ab8802e1234567890abcdeg"},
		{"leading whitespace", " 550e8400-e29b-41d4-a716-446655440000"},
		{"too short for any format", "00008030"},
		{"too long for any format", "00008030000a4d8e0ab8802e1234567890abcdef00008030000a4d8e0ab8802e12"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if validateUDID(t, tt.udid) {
				t.Errorf("UDIDValidator accepted %q, want it rejected", tt.udid)
			}
		})
	}
}
