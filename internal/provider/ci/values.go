// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// These conversions keep Apple's wire names separate from Terraform's typed attributes.
func toJSON(value attr.Value) (any, error) {
	if value.IsUnknown() {
		return nil, fmt.Errorf("workflow configuration contains an unknown value")
	}
	if value.IsNull() {
		return nil, nil
	}
	switch v := value.(type) {
	case types.String:
		return v.ValueString(), nil
	case types.Bool:
		return v.ValueBool(), nil
	case types.Int64:
		return v.ValueInt64(), nil
	case types.List:
		result := make([]any, 0, len(v.Elements()))
		for _, item := range v.Elements() {
			converted, err := toJSON(item)
			if err != nil {
				return nil, err
			}
			result = append(result, converted)
		}
		return result, nil
	case types.Object:
		result := make(map[string]any)
		for name, item := range v.Attributes() {
			converted, err := toJSON(item)
			if err != nil {
				return nil, err
			}
			key := jsonNames[name]
			if key == "" {
				key = name
			}
			result[key] = converted
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported workflow value %T", value)
	}
}
func fromJSON(ctx context.Context, typ attr.Type, value any) (attr.Value, error) {
	if value == nil {
		return typ.ValueFromTerraform(ctx, tftypes.NewValue(typ.TerraformType(ctx), nil))
	}
	switch t := typ.(type) {
	case basetypes.StringType:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", value)
		}
		return types.StringValue(s), nil
	case basetypes.BoolType:
		b, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", value)
		}
		return types.BoolValue(b), nil
	case basetypes.Int64Type:
		n, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("expected integer, got %T", value)
		}
		return types.Int64Value(int64(n)), nil
	case basetypes.ListType:
		items, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("expected list, got %T", value)
		}
		elements := make([]attr.Value, 0, len(items))
		for _, item := range items {
			v, err := fromJSON(ctx, t.ElemType, item)
			if err != nil {
				return nil, err
			}
			elements = append(elements, v)
		}
		return types.ListValueMust(t.ElemType, elements), nil
	case basetypes.ObjectType:
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object, got %T", value)
		}
		values := make(map[string]attr.Value, len(t.AttrTypes))
		for name, fieldType := range t.AttrTypes {
			key := jsonNames[name]
			if key == "" {
				key = name
			}
			v, err := fromJSON(ctx, fieldType, object[key])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			values[name] = v
		}
		return types.ObjectValueMust(t.AttrTypes, values), nil
	default:
		return nil, fmt.Errorf("unsupported workflow type %T", typ)
	}
}
