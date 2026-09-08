// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package bundle contains Bundle ID Capability related resources, data sources, and shared functionality
// for the Apple Terraform provider.
package bundle

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

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

		// The inverse of the mapping SettingsToAPI applies: a setting with
		// exactly one option reports that option's key as its value, so a
		// configuration written with value round-trips unchanged. Anything with
		// several options is a genuine multi-option setting and has no scalar
		// value to report.
		value := types.StringNull()
		if len(setting.Options) == 1 {
			value = types.StringValue(setting.Options[0].Key)
		}

		settingObj, diags := types.ObjectValue(capabilitySettingType.AttrTypes, map[string]attr.Value{
			"key":       types.StringValue(setting.Key),
			"name":      types.StringValue(setting.Name),
			"value":     value,
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
// Note that value is shorthand: it is sent to Apple as a single-element options
// list, because Apple has no scalar value property on a capability setting.
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
			Key:  settingModel.Key.ValueString(),
			Name: settingModel.Name.ValueString(),
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

		// Apple takes the chosen value as a one-element options list keyed by
		// the value itself. Explicit options win when both are given; value is
		// the shorthand for the overwhelmingly common single-choice setting.
		if len(options) == 0 && !settingModel.Value.IsNull() && settingModel.Value.ValueString() != "" {
			options = []models.CapabilitySettingOption{{Key: settingModel.Value.ValueString()}}
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

// The resource and the data source describe capability settings differently,
// and deliberately so.
//
// Apple returns display metadata alongside a setting -- name, visible,
// minCount, and per-option labels -- none of which is configuration. Exposing
// them on the resource as Optional+Computed made every plan non-empty: nested
// computed attributes inside a non-computed list are marked unknown on each
// plan rather than taking their prior state, so Terraform proposed a change to
// a resource nobody had touched. The resource therefore models only what Apple
// accepts as input, and the data source keeps reporting everything.

// capabilitySettingInputOptionType is the option form the resource accepts.
var capabilitySettingInputOptionType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"key": types.StringType,
	},
}

// capabilitySettingInputType is the setting form the resource accepts.
var capabilitySettingInputType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"key":     types.StringType,
		"value":   types.StringType,
		"options": types.ListType{ElemType: capabilitySettingInputOptionType},
	},
}

type capabilitySettingInputModel struct {
	Key     types.String `tfsdk:"key"`
	Value   types.String `tfsdk:"value"`
	Options types.List   `tfsdk:"options"`
}

type capabilitySettingInputOptionModel struct {
	Key types.String `tfsdk:"key"`
}

// SettingsInputToAPI converts the resource's settings into Apple's form.
//
// value is shorthand for a single-choice setting and becomes a one-element
// options list; explicit options win when both are given.
func SettingsInputToAPI(ctx context.Context, tfSettings types.List) ([]models.CapabilitySetting, error) {
	if tfSettings.IsNull() || tfSettings.IsUnknown() {
		return nil, nil
	}

	var settingModels []capabilitySettingInputModel
	if diags := tfSettings.ElementsAs(ctx, &settingModels, false); diags.HasError() {
		return nil, fmt.Errorf("failed to read capability settings: %s", diagnosticsError(diags))
	}

	var apiSettings []models.CapabilitySetting
	for _, settingModel := range settingModels {
		setting := models.CapabilitySetting{Key: settingModel.Key.ValueString()}

		if !settingModel.Options.IsNull() && !settingModel.Options.IsUnknown() {
			var optionModels []capabilitySettingInputOptionModel
			if diags := settingModel.Options.ElementsAs(ctx, &optionModels, false); diags.HasError() {
				return nil, fmt.Errorf("setting %q: failed to read options: %s", setting.Key, diagnosticsError(diags))
			}

			for _, optionModel := range optionModels {
				setting.Options = append(setting.Options, models.CapabilitySettingOption{
					Key: optionModel.Key.ValueString(),
				})
			}
		}

		if len(setting.Options) == 0 && !settingModel.Value.IsNull() && settingModel.Value.ValueString() != "" {
			setting.Options = []models.CapabilitySettingOption{{Key: settingModel.Value.ValueString()}}
		}

		apiSettings = append(apiSettings, setting)
	}

	return apiSettings, nil
}

// SettingsInputFromAPI converts Apple's settings back into the resource's form.
//
// A setting with exactly one option is reported through value with options left
// null, which is the shape a configuration using the shorthand wrote; anything
// with several options is reported through options with value null. Keeping the
// two mutually exclusive is what makes the round trip produce an empty plan.
func SettingsInputFromAPI(apiSettings []models.CapabilitySetting) (types.List, error) {
	if len(apiSettings) == 0 {
		return types.ListNull(capabilitySettingInputType), nil
	}

	var settingsObjects []attr.Value
	for _, setting := range apiSettings {
		value := types.StringNull()
		options := types.ListNull(capabilitySettingInputOptionType)

		switch {
		case len(setting.Options) == 1:
			value = types.StringValue(setting.Options[0].Key)
		case len(setting.Options) > 1:
			var optionObjects []attr.Value
			for _, option := range setting.Options {
				optionObj, diags := types.ObjectValue(capabilitySettingInputOptionType.AttrTypes, map[string]attr.Value{
					"key": types.StringValue(option.Key),
				})
				if diags.HasError() {
					return types.ListNull(capabilitySettingInputType), fmt.Errorf("failed to create option object: %s", diagnosticsError(diags))
				}
				optionObjects = append(optionObjects, optionObj)
			}

			optionsList, diags := types.ListValue(capabilitySettingInputOptionType, optionObjects)
			if diags.HasError() {
				return types.ListNull(capabilitySettingInputType), fmt.Errorf("failed to create options list: %s", diagnosticsError(diags))
			}
			options = optionsList
		}

		settingObj, diags := types.ObjectValue(capabilitySettingInputType.AttrTypes, map[string]attr.Value{
			"key":     types.StringValue(setting.Key),
			"value":   value,
			"options": options,
		})
		if diags.HasError() {
			return types.ListNull(capabilitySettingInputType), fmt.Errorf("failed to create setting object: %s", diagnosticsError(diags))
		}
		settingsObjects = append(settingsObjects, settingObj)
	}

	settingsList, diags := types.ListValue(capabilitySettingInputType, settingsObjects)
	if diags.HasError() {
		return types.ListNull(capabilitySettingInputType), fmt.Errorf("failed to create settings list: %s", diagnosticsError(diags))
	}

	return settingsList, nil
}
