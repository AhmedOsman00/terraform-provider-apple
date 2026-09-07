// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package bundle contains Bundle ID Capability related resources, data sources, and shared functionality
// for the Apple Terraform provider.
package bundle

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// bundleIDCapabilityModel maps the Bundle ID Capability schema data for resources and data sources.
type bundleIDCapabilityModel struct {
	ID             types.String `tfsdk:"id"`
	BundleID       types.String `tfsdk:"bundle_id"`
	CapabilityType types.String `tfsdk:"capability_type"`
	Settings       types.List   `tfsdk:"settings"`
}

// bundleIDCapabilitiesDataSourceModel maps the data source schema for listing Bundle ID Capabilities.
type bundleIDCapabilitiesDataSourceModel struct {
	// Filter configuration
	BundleID       types.String `tfsdk:"bundle_id"`
	CapabilityType types.String `tfsdk:"capability_type"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	Capabilities []bundleIDCapabilityModel `tfsdk:"capabilities"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// capabilitySettingModel maps a single capability setting. It is the target
// type for ElementsAs when converting Terraform settings back to API models.
type capabilitySettingModel struct {
	Key      types.String `tfsdk:"key"`
	Name     types.String `tfsdk:"name"`
	Value    types.String `tfsdk:"value"`
	Visible  types.Bool   `tfsdk:"visible"`
	MinCount types.Int64  `tfsdk:"min_count"`
	Options  types.List   `tfsdk:"options"`
}

// capabilitySettingOptionModel maps a single capability setting option.
type capabilitySettingOptionModel struct {
	Key         types.String `tfsdk:"key"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Enabled     types.Bool   `tfsdk:"enabled"`
}

// Bundle ID Capability validators.
var (
	// CapabilityTypeValidator validates capability type values.
	CapabilityTypeValidator = stringvalidator.OneOf(models.ValidCapabilityTypes...)

	// CapabilitySortByValidator validates sort field options.
	CapabilitySortByValidator = stringvalidator.OneOf("capability_type", "bundle_id")

	// CapabilitySortOrderValidator validates sort order options.
	CapabilitySortOrderValidator = stringvalidator.OneOf("asc", "desc")

	// BundleIDValidator validates Bundle ID format (Apple's internal ID format).
	BundleIDValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[A-Z0-9]{10}$|^bundle-[a-zA-Z0-9-]+$`),
		"Bundle ID must be either an Apple-generated ID (10 characters) or Bundle identifier format",
	)
)

// GetCapabilityTypeValidator returns validators for capability type fields.
func GetCapabilityTypeValidator() []validator.String {
	return []validator.String{
		CapabilityTypeValidator,
		stringvalidator.LengthBetween(1, 50),
	}
}

// GetBundleIDValidator returns validators for bundle ID fields.
func GetBundleIDValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 255),
	}
}

// capabilitySettingType defines the object type for capability settings.
var capabilitySettingType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"key":       types.StringType,
		"name":      types.StringType,
		"value":     types.StringType,
		"visible":   types.BoolType,
		"min_count": types.Int64Type,
		"options":   types.ListType{ElemType: capabilitySettingOptionType},
	},
}

// capabilitySettingOptionType defines the object type for capability setting options.
var capabilitySettingOptionType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"key":         types.StringType,
		"name":        types.StringType,
		"description": types.StringType,
		"enabled":     types.BoolType,
	},
}

// diagnosticsError renders framework diagnostics as a single error string.
func diagnosticsError(diags diag.Diagnostics) string {
	msgs := make([]string, 0, len(diags))
	for _, d := range diags {
		msgs = append(msgs, fmt.Sprintf("%s: %s", d.Summary(), d.Detail()))
	}
	return strings.Join(msgs, "; ")
}

// settingValueToString renders a capability setting value as the string the
// Terraform schema declares.
//
// Apple types this field as an arbitrary JSON value: capabilities such as
// ICLOUD return objects, others return booleans or numbers, and a setting that
// omits "value" entirely decodes to nil. Asserting it to a string panics the
// provider on every one of those, so each shape is converted explicitly and
// complex values round-trip as JSON, matching the schema's description.
func settingValueToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case float64:
		// encoding/json decodes every JSON number into a float64; render whole
		// numbers without a misleading decimal component.
		if v == math.Trunc(v) && math.Abs(v) < 1<<53 {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("could not represent capability setting value of type %T as a string: %w", value, err)
		}
		return string(encoded), nil
	}
}

// Helper functions to convert between API models and Terraform models

// SettingsFromAPI converts Apple API settings to Terraform settings.
func SettingsFromAPI(apiSettings []models.CapabilitySetting) (types.List, error) {
	if len(apiSettings) == 0 {
		return types.ListNull(capabilitySettingType), nil
	}

	var settingsObjects []attr.Value
	for _, setting := range apiSettings {
		options, err := OptionsFromAPI(setting.Options)
		if err != nil {
			return types.ListNull(capabilitySettingType), err
		}

		visible := types.BoolNull()
		if setting.Visible != nil {
			visible = types.BoolValue(*setting.Visible)
		}

		minCount := types.Int64Null()
		if setting.MinCount != nil {
			minCount = types.Int64Value(int64(*setting.MinCount))
		}

		value, err := settingValueToString(setting.Value)
		if err != nil {
			return types.ListNull(capabilitySettingType), fmt.Errorf("setting %q: %w", setting.Key, err)
		}

		settingObj, diags := types.ObjectValue(capabilitySettingType.AttrTypes, map[string]attr.Value{
			"key":       types.StringValue(setting.Key),
			"name":      types.StringValue(setting.Name),
			"value":     types.StringValue(value),
			"visible":   visible,
			"min_count": minCount,
			"options":   options,
		})
		if diags.HasError() {
			return types.ListNull(capabilitySettingType), fmt.Errorf("failed to create setting object: %s", diagnosticsError(diags))
		}
		settingsObjects = append(settingsObjects, settingObj)
	}

	settingsList, diags := types.ListValue(capabilitySettingType, settingsObjects)
	if diags.HasError() {
		return types.ListNull(capabilitySettingType), fmt.Errorf("failed to create settings list: %s", diagnosticsError(diags))
	}
	return settingsList, nil
}

// OptionsFromAPI converts Apple API setting options to Terraform options.
func OptionsFromAPI(apiOptions []models.CapabilitySettingOption) (types.List, error) {
	if len(apiOptions) == 0 {
		return types.ListNull(capabilitySettingOptionType), nil
	}

	var optionObjects []attr.Value
	for _, option := range apiOptions {
		enabled := types.BoolNull()
		if option.Enabled != nil {
			enabled = types.BoolValue(*option.Enabled)
		}

		optionObj, diags := types.ObjectValue(capabilitySettingOptionType.AttrTypes, map[string]attr.Value{
			"key":         types.StringValue(option.Key),
			"name":        types.StringValue(option.Name),
			"description": types.StringValue(option.Description),
			"enabled":     enabled,
		})
		if diags.HasError() {
			return types.ListNull(capabilitySettingOptionType), fmt.Errorf("failed to create option object: %s", diagnosticsError(diags))
		}
		optionObjects = append(optionObjects, optionObj)
	}

	optionsList, diags := types.ListValue(capabilitySettingOptionType, optionObjects)
	if diags.HasError() {
		return types.ListNull(capabilitySettingOptionType), fmt.Errorf("failed to create options list: %s", diagnosticsError(diags))
	}
	return optionsList, nil
}

// SettingsToAPI converts Terraform settings to Apple API settings.
//
// Conversion goes through ElementsAs rather than walking the list and asserting
// each attribute's type. A missing or differently typed attribute returns a
// diagnostic here, where the old assertions panicked the provider.
//
// Note that value is sent back as the string the schema declares, even when it
// originally arrived from Apple as an object or number. See settingValueToString.
func SettingsToAPI(ctx context.Context, tfSettings types.List) ([]models.CapabilitySetting, error) {
	if tfSettings.IsNull() || tfSettings.IsUnknown() {
		return nil, nil
	}

	var settingModels []capabilitySettingModel
	if diags := tfSettings.ElementsAs(ctx, &settingModels, false); diags.HasError() {
		return nil, fmt.Errorf("failed to read capability settings: %s", diagnosticsError(diags))
	}

	var apiSettings []models.CapabilitySetting
	for _, settingModel := range settingModels {
		setting := models.CapabilitySetting{
			Key:   settingModel.Key.ValueString(),
			Name:  settingModel.Name.ValueString(),
			Value: settingModel.Value.ValueString(),
		}

		if !settingModel.Visible.IsNull() && !settingModel.Visible.IsUnknown() {
			visible := settingModel.Visible.ValueBool()
			setting.Visible = &visible
		}

		if !settingModel.MinCount.IsNull() && !settingModel.MinCount.IsUnknown() {
			minCount := int(settingModel.MinCount.ValueInt64())
			setting.MinCount = &minCount
		}

		options, err := OptionsToAPI(ctx, settingModel.Options)
		if err != nil {
			return nil, fmt.Errorf("setting %q: %w", setting.Key, err)
		}
		setting.Options = options

		apiSettings = append(apiSettings, setting)
	}

	return apiSettings, nil
}

// OptionsToAPI converts Terraform options to Apple API options.
func OptionsToAPI(ctx context.Context, tfOptions types.List) ([]models.CapabilitySettingOption, error) {
	if tfOptions.IsNull() || tfOptions.IsUnknown() {
		return nil, nil
	}

	var optionModels []capabilitySettingOptionModel
	if diags := tfOptions.ElementsAs(ctx, &optionModels, false); diags.HasError() {
		return nil, fmt.Errorf("failed to read capability setting options: %s", diagnosticsError(diags))
	}

	var apiOptions []models.CapabilitySettingOption
	for _, optionModel := range optionModels {
		option := models.CapabilitySettingOption{
			Key:         optionModel.Key.ValueString(),
			Name:        optionModel.Name.ValueString(),
			Description: optionModel.Description.ValueString(),
		}

		if !optionModel.Enabled.IsNull() && !optionModel.Enabled.IsUnknown() {
			enabled := optionModel.Enabled.ValueBool()
			option.Enabled = &enabled
		}

		apiOptions = append(apiOptions, option)
	}

	return apiOptions, nil
}
