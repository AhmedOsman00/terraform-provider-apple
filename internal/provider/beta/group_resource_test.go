// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package beta

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestAllBuildsReplacesOnlyAnInternalGroup pins the case an import hits:
// Apple reports has_access_to_all_builds as null on an external group, so an
// imported one holds null against a configuration saying true. Replacing it
// would delete the group and its public link for a value Apple ignores.
func TestAllBuildsReplacesOnlyAnInternalGroup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	NewBetaGroupResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)

	for name, test := range map[string]struct {
		internal types.Bool
		want     bool
	}{
		"external group":        {types.BoolValue(false), false},
		"internal group":        {types.BoolValue(true), true},
		"kind unknown to state": {types.BoolNull(), true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
			}
			model := betaGroupModel{
				ID:              types.StringValue("GROUP"),
				AppID:           types.StringValue("APP"),
				Name:            types.StringValue("Public beta"),
				IsInternalGroup: test.internal,
			}
			if diags := state.Set(ctx, &model); diags.HasError() {
				t.Fatalf("setting state: %v", diags)
			}

			resp := &boolplanmodifier.RequiresReplaceIfFuncResponse{}
			allBuildsRequiresReplace(ctx, planmodifier.BoolRequest{
				State:       state,
				StateValue:  types.BoolNull(),
				PlanValue:   types.BoolValue(true),
				ConfigValue: types.BoolValue(true),
			}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if resp.RequiresReplace != test.want {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, test.want)
			}
		})
	}
}
