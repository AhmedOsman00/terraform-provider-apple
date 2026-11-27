// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package bundle

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// validateListAttribute runs one list attribute's validators against a
// configuration holding the given values and null everywhere else.
func validateListAttribute(t *testing.T, ds datasource.DataSource, attribute string, values map[string]tftypes.Value) diag.Diagnostics {
	t.Helper()
	ctx := context.Background()

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	objectType, ok := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, want tftypes.Object", schemaResp.Schema.Type().TerraformType(ctx))
	}
	raw := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		raw[name] = tftypes.NewValue(attributeType, nil)
	}
	for name, value := range values {
		raw[name] = value
	}
	config := tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, raw)}

	var value types.List
	if diags := config.GetAttribute(ctx, path.Root(attribute), &value); diags.HasError() {
		t.Fatalf("reading %s: %v", attribute, diags)
	}

	list, ok := schemaResp.Schema.Attributes[attribute].(schema.ListAttribute)
	if !ok {
		t.Fatalf("%s is %T, want a list attribute", attribute, schemaResp.Schema.Attributes[attribute])
	}

	var diags diag.Diagnostics
	for _, v := range list.Validators {
		resp := &validator.ListResponse{}
		v.ValidateList(ctx, validator.ListRequest{
			Config:         config,
			ConfigValue:    value,
			Path:           path.Root(attribute),
			PathExpression: path.MatchRoot(attribute),
		}, resp)
		diags.Append(resp.Diagnostics...)
	}
	return diags
}

func stringList(values ...string) tftypes.Value {
	elements := make([]tftypes.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, tftypes.NewValue(tftypes.String, value))
	}
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, elements)
}

// TestBundleIDsDataSourcePlatformFilters checks that platform and platforms
// are refused together, as their descriptions say, and that an unknown value
// is not mistaken for a conflict before it is known.
func TestBundleIDsDataSourcePlatformFilters(t *testing.T) {
	for name, tc := range map[string]struct {
		platform tftypes.Value
		conflict bool
	}{
		"both set":         {platform: tftypes.NewValue(tftypes.String, "IOS"), conflict: true},
		"platforms only":   {platform: tftypes.NewValue(tftypes.String, nil)},
		"platform unknown": {platform: tftypes.NewValue(tftypes.String, tftypes.UnknownValue)},
	} {
		t.Run(name, func(t *testing.T) {
			diags := validateListAttribute(t, NewBundleIDsDataSource(), "platforms", map[string]tftypes.Value{
				"platform":  tc.platform,
				"platforms": stringList("IOS", "MAC_OS"),
			})
			if diags.HasError() != tc.conflict {
				t.Errorf("diagnostics = %v, want an error: %v", diags, tc.conflict)
			}
		})
	}
}
