// Package bundle contains Bundle ID Capability related resources, data sources, and shared functionality
// for the Apple Terraform provider.
package bundle

import (
	"fmt"
	"regexp"
	"terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	TotalCount    types.Int64  `tfsdk:"total_count"`
	FilteredCount types.Int64  `tfsdk:"filtered_count"`
	LastUpdated   types.String `tfsdk:"last_updated"`
}

// capabilitySettingModel maps capability setting data
type capabilitySettingModel struct {
	Key      types.String `tfsdk:"key"`
	Name     types.String `tfsdk:"name"`
	Value    types.String `tfsdk:"value"`
	Visible  types.Bool   `tfsdk:"visible"`
	MinCount types.Int64  `tfsdk:"min_count"`
	Options  types.List   `tfsdk:"options"`
}

// capabilitySettingOptionModel maps capability setting option data
type capabilitySettingOptionModel struct {
	Key         types.String `tfsdk:"key"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Enabled     types.Bool   `tfsdk:"enabled"`
}

// Bundle ID Capability validators
var (
	// CapabilityTypeValidator validates capability type values
	CapabilityTypeValidator = stringvalidator.OneOf(models.ValidCapabilityTypes...)

	// CapabilitySortByValidator validates sort field options
	CapabilitySortByValidator = stringvalidator.OneOf("capability_type", "bundle_id")

	// CapabilitySortOrderValidator validates sort order options
	CapabilitySortOrderValidator = stringvalidator.OneOf("asc", "desc")

	// BundleIDValidator validates Bundle ID format (Apple's internal ID format)
	BundleIDValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[A-Z0-9]{10}$|^bundle-[a-zA-Z0-9-]+$`),
		"Bundle ID must be either an Apple-generated ID (10 characters) or Bundle identifier format",
	)
)

// GetCapabilityTypeValidator returns validators for capability type fields
func GetCapabilityTypeValidator() []validator.String {
	return []validator.String{
		CapabilityTypeValidator,
		stringvalidator.LengthBetween(1, 50),
	}
}

// GetBundleIDValidator returns validators for bundle ID fields
func GetBundleIDValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 255),
	}
}

// capabilitySettingType defines the object type for capability settings
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

// capabilitySettingOptionType defines the object type for capability setting options
var capabilitySettingOptionType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"key":         types.StringType,
		"name":        types.StringType,
		"description": types.StringType,
		"enabled":     types.BoolType,
	},
}

// Helper functions to convert between API models and Terraform models

// SettingsFromAPI converts Apple API settings to Terraform settings
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

		settingObj, diags := types.ObjectValue(capabilitySettingType.AttrTypes, map[string]attr.Value{
			"key":       types.StringValue(setting.Key),
			"name":      types.StringValue(setting.Name),
			"value":     types.StringValue(setting.Value.(string)),
			"visible":   visible,
			"min_count": minCount,
			"options":   options,
		})
		if diags.HasError() {
			return types.ListNull(capabilitySettingType), fmt.Errorf("failed to create setting object: %s", diags)
		}
		settingsObjects = append(settingsObjects, settingObj)
	}

	settingsList, diags := types.ListValue(capabilitySettingType, settingsObjects)
	if diags.HasError() {
		return types.ListNull(capabilitySettingType), fmt.Errorf("failed to create settings list: %s", diags)
	}
	return settingsList, nil
}

// OptionsFromAPI converts Apple API setting options to Terraform options
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
			return types.ListNull(capabilitySettingOptionType), fmt.Errorf("failed to create option object: %s", diags)
		}
		optionObjects = append(optionObjects, optionObj)
	}

	optionsList, diags := types.ListValue(capabilitySettingOptionType, optionObjects)
	if diags.HasError() {
		return types.ListNull(capabilitySettingOptionType), fmt.Errorf("failed to create options list: %s", diags)
	}
	return optionsList, nil
}

// SettingsToAPI converts Terraform settings to Apple API settings
func SettingsToAPI(tfSettings types.List) ([]models.CapabilitySetting, error) {
	if tfSettings.IsNull() || tfSettings.IsUnknown() {
		return nil, nil
	}

	var apiSettings []models.CapabilitySetting
	settingsValues := tfSettings.Elements()

	for _, settingValue := range settingsValues {
		settingObj := settingValue.(types.Object)
		settingAttrs := settingObj.Attributes()

		setting := models.CapabilitySetting{
			Key:   settingAttrs["key"].(types.String).ValueString(),
			Name:  settingAttrs["name"].(types.String).ValueString(),
			Value: settingAttrs["value"].(types.String).ValueString(),
		}

		if !settingAttrs["visible"].(types.Bool).IsNull() {
			visible := settingAttrs["visible"].(types.Bool).ValueBool()
			setting.Visible = &visible
		}

		if !settingAttrs["min_count"].(types.Int64).IsNull() {
			minCount := int(settingAttrs["min_count"].(types.Int64).ValueInt64())
			setting.MinCount = &minCount
		}

		// Convert options
		optionsList := settingAttrs["options"].(types.List)
		if !optionsList.IsNull() && !optionsList.IsUnknown() {
			options, err := OptionsToAPI(optionsList)
			if err != nil {
				return nil, err
			}
			setting.Options = options
		}

		apiSettings = append(apiSettings, setting)
	}

	return apiSettings, nil
}

// OptionsToAPI converts Terraform options to Apple API options
func OptionsToAPI(tfOptions types.List) ([]models.CapabilitySettingOption, error) {
	if tfOptions.IsNull() || tfOptions.IsUnknown() {
		return nil, nil
	}

	var apiOptions []models.CapabilitySettingOption
	optionsValues := tfOptions.Elements()

	for _, optionValue := range optionsValues {
		optionObj := optionValue.(types.Object)
		optionAttrs := optionObj.Attributes()

		option := models.CapabilitySettingOption{
			Key:         optionAttrs["key"].(types.String).ValueString(),
			Name:        optionAttrs["name"].(types.String).ValueString(),
			Description: optionAttrs["description"].(types.String).ValueString(),
		}

		if !optionAttrs["enabled"].(types.Bool).IsNull() {
			enabled := optionAttrs["enabled"].(types.Bool).ValueBool()
			option.Enabled = &enabled
		}

		apiOptions = append(apiOptions, option)
	}

	return apiOptions, nil
}
