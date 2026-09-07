// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package bundle

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &bundleIDCapabilityResource{}
	_ resource.ResourceWithConfigure   = &bundleIDCapabilityResource{}
	_ resource.ResourceWithImportState = &bundleIDCapabilityResource{}
)

func NewBundleIDCapabilityResource() resource.Resource {
	return &bundleIDCapabilityResource{}
}

type bundleIDCapabilityResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *bundleIDCapabilityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bundle_id_capability"
}

// Schema defines the schema for the resource.
func (r *bundleIDCapabilityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Apple App Store Connect Bundle ID Capability.\n\n" +
			"Bundle ID Capabilities define what services and features your app can use, such as " +
			"push notifications, in-app purchases, HealthKit, and more. Each capability is associated " +
			"with a specific Bundle ID and can have configurable settings.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the Bundle ID Capability. Used for API references.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bundle_id": schema.StringAttribute{
				MarkdownDescription: "The Bundle ID that this capability is associated with. " +
					"This can be either the Apple-generated Bundle ID or a reference to a bundle_id resource.",
				Required:   true,
				Validators: GetBundleIDValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"capability_type": schema.StringAttribute{
				MarkdownDescription: "The type of capability to enable. Valid values include:\n" +
					"- `ICLOUD` - iCloud services\n" +
					"- `PUSH_NOTIFICATIONS` - Push notifications\n" +
					"- `IN_APP_PURCHASE` - In-app purchases\n" +
					"- `GAME_CENTER` - Game Center integration\n" +
					"- `APP_GROUPS` - App Groups for data sharing\n" +
					"- `APPLE_PAY` - Apple Pay payments\n" +
					"- `ASSOCIATED_DOMAINS` - Associated domains\n" +
					"- `HEALTH_KIT` - HealthKit data access\n" +
					"- `HOME_KIT` - HomeKit device control\n" +
					"- `SIRI` - SiriKit integration\n" +
					"- `WALLET_PASSES` - Wallet passes\n" +
					"And many more. This cannot be changed after creation.",
				Required:   true,
				Validators: GetCapabilityTypeValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"settings": schema.ListNestedAttribute{
				MarkdownDescription: "Configuration settings for the capability. The available settings depend on the capability type. " +
					"Some capabilities like PUSH_NOTIFICATIONS don't require settings, while others like ICLOUD have multiple configuration options.",
				Optional: true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "The setting key identifier.",
							Required:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Human-readable name for the setting.",
							Optional:            true,
							Computed:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "The setting value. This can be a string, boolean (as string), or JSON for complex values.",
							Required:            true,
						},
						"visible": schema.BoolAttribute{
							MarkdownDescription: "Whether this setting is visible in the Apple Developer portal.",
							Optional:            true,
							Computed:            true,
						},
						"min_count": schema.Int64Attribute{
							MarkdownDescription: "Minimum number of values required for this setting.",
							Optional:            true,
							Computed:            true,
						},
						"options": schema.ListNestedAttribute{
							MarkdownDescription: "Available options for this setting.",
							Optional:            true,
							Computed:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"key": schema.StringAttribute{
										MarkdownDescription: "The option key identifier.",
										Required:            true,
									},
									"name": schema.StringAttribute{
										MarkdownDescription: "Human-readable name for the option.",
										Optional:            true,
										Computed:            true,
									},
									"description": schema.StringAttribute{
										MarkdownDescription: "Description of what this option enables.",
										Optional:            true,
										Computed:            true,
									},
									"enabled": schema.BoolAttribute{
										MarkdownDescription: "Whether this option is enabled.",
										Optional:            true,
										Computed:            true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Create a new resource.
func (r *bundleIDCapabilityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating Bundle ID Capability resource")

	// Retrieve values from plan
	var plan bundleIDCapabilityModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add structured logging
	ctx = tflog.SetField(ctx, "bundle_id", plan.BundleID.ValueString())
	ctx = tflog.SetField(ctx, "capability_type", plan.CapabilityType.ValueString())

	tflog.Debug(ctx, "Creating Bundle ID Capability with Apple API")

	// Convert settings from Terraform to API format
	settings, err := SettingsToAPI(ctx, plan.Settings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Settings",
			fmt.Sprintf("Could not convert capability settings: %s", err.Error()),
		)
		return
	}

	// Create new Bundle ID Capability
	capability, err := r.client.CreateBundleIDCapability(
		plan.BundleID.ValueString(),
		models.CapabilityType(plan.CapabilityType.ValueString()),
		settings,
		nil,
	)
	if err != nil {
		// Provide specific error messages for common issues
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			resp.Diagnostics.AddError(
				"Bundle ID Capability Already Exists",
				fmt.Sprintf("A capability of type '%s' already exists for Bundle ID '%s'. Each capability type can only be added once per Bundle ID.", plan.CapabilityType.ValueString(), plan.BundleID.ValueString()),
			)
		} else if strings.Contains(errMsg, "invalid capability") {
			resp.Diagnostics.AddError(
				"Invalid Capability Type",
				fmt.Sprintf("The capability type '%s' is not valid or not supported for this Bundle ID.", plan.CapabilityType.ValueString()),
			)
		} else if strings.Contains(errMsg, "bundle ID not found") {
			resp.Diagnostics.AddError(
				"Bundle ID Not Found",
				fmt.Sprintf("Bundle ID '%s' was not found. Please verify the Bundle ID exists in your Apple Developer account.", plan.BundleID.ValueString()),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error Creating Bundle ID Capability",
				fmt.Sprintf("Could not create Bundle ID Capability: %s", err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create Bundle ID Capability", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Map response body to schema and populate computed attribute values
	plan.ID = types.StringValue(capability.ID)

	// Convert settings from API to Terraform format
	tfSettings, err := SettingsFromAPI(capability.Attributes.Settings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Settings",
			fmt.Sprintf("Could not convert capability settings from API response: %s", err.Error()),
		)
		return
	}
	plan.Settings = tfSettings

	tflog.Info(ctx, "Bundle ID Capability created successfully", map[string]interface{}{
		"capability_id": capability.ID,
	})

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *bundleIDCapabilityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading Bundle ID Capability resource")

	// Get current state
	var state bundleIDCapabilityModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "capability_id", state.ID.ValueString())

	// Get refreshed Bundle ID Capability value from Apple
	capability, err := r.client.GetBundleIDCapability(state.ID.ValueString())
	if err != nil {
		// Handle 404 - resource no longer exists
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, "Bundle ID Capability not found, removing from state")
			resp.Diagnostics.AddWarning(
				"Bundle ID Capability Not Found",
				fmt.Sprintf("Bundle ID Capability %s was not found and will be removed from Terraform state. It may have been deleted outside of Terraform.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}

		tflog.Error(ctx, "Failed to read Bundle ID Capability", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Reading Bundle ID Capability",
			fmt.Sprintf("Could not read Bundle ID Capability %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Convert settings from API to Terraform format
	tfSettings, err := SettingsFromAPI(capability.Attributes.Settings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Settings",
			fmt.Sprintf("Could not convert capability settings from API response: %s", err.Error()),
		)
		return
	}

	// Overwrite Bundle ID Capability with refreshed state
	state.ID = types.StringValue(capability.ID)
	state.CapabilityType = types.StringValue(string(capability.Attributes.CapabilityType))
	state.Settings = tfSettings

	tflog.Debug(ctx, "Bundle ID Capability read successfully")

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *bundleIDCapabilityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating Bundle ID Capability resource")

	// Retrieve values from plan
	var plan bundleIDCapabilityModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state bundleIDCapabilityModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "capability_id", state.ID.ValueString())

	tflog.Debug(ctx, "Updating Bundle ID Capability with Apple API")

	// Convert settings from Terraform to API format
	settings, err := SettingsToAPI(ctx, plan.Settings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Settings",
			fmt.Sprintf("Could not convert capability settings: %s", err.Error()),
		)
		return
	}

	// Update Bundle ID Capability (only settings can be updated)
	capability, err := r.client.UpdateBundleIDCapability(state.ID.ValueString(), nil, settings, nil)
	if err != nil {
		// Handle specific update errors
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			resp.Diagnostics.AddError(
				"Bundle ID Capability Not Found",
				fmt.Sprintf("Bundle ID Capability %s was not found during update. It may have been deleted outside of Terraform.", state.ID.ValueString()),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error Updating Bundle ID Capability",
				fmt.Sprintf("Could not update Bundle ID Capability %s: %s", state.ID.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to update Bundle ID Capability", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Convert settings from API to Terraform format
	tfSettings, err := SettingsFromAPI(capability.Attributes.Settings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Settings",
			fmt.Sprintf("Could not convert capability settings from API response: %s", err.Error()),
		)
		return
	}

	// Update state with updated values
	plan.ID = types.StringValue(capability.ID)
	plan.Settings = tfSettings

	tflog.Info(ctx, "Bundle ID Capability updated successfully")

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *bundleIDCapabilityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting Bundle ID Capability resource")

	// Retrieve values from state
	var state bundleIDCapabilityModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "capability_id", state.ID.ValueString())
	ctx = tflog.SetField(ctx, "capability_type", state.CapabilityType.ValueString())

	tflog.Debug(ctx, "Deleting Bundle ID Capability with Apple API")

	// Delete Bundle ID Capability
	err := r.client.DeleteBundleIDCapability(state.ID.ValueString(), nil)
	if err != nil {
		// Handle specific delete errors
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			// Resource already deleted, this is not an error
			tflog.Warn(ctx, "Bundle ID Capability already deleted outside of Terraform")
			resp.Diagnostics.AddWarning(
				"Bundle ID Capability Already Deleted",
				fmt.Sprintf("Bundle ID Capability %s was already deleted outside of Terraform.", state.ID.ValueString()),
			)
			return
		}

		tflog.Error(ctx, "Failed to delete Bundle ID Capability", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Deleting Bundle ID Capability",
			fmt.Sprintf("Could not delete Bundle ID Capability %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Bundle ID Capability deleted successfully")
}

// ImportState imports the resource into Terraform state.
func (r *bundleIDCapabilityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing Bundle ID Capability resource", map[string]interface{}{
		"import_id": req.ID,
	})

	// Import accepts "<bundle_id>/<capability_id>" or a bare capability ID.
	// Apple's capability response does not carry its parent Bundle ID, so the
	// composite form is the only one that yields complete state.
	importID := strings.TrimSpace(req.ID)

	if importID == "" {
		resp.Diagnostics.AddError(
			"Empty Import Identifier",
			"The import identifier cannot be empty. Provide \"<bundle_id>/<capability_id>\" (preferred) or the Bundle ID Capability ID on its own.",
		)
		return
	}

	var bundleID, capabilityID string
	if parts := strings.SplitN(importID, "/", 2); len(parts) == 2 {
		bundleID = strings.TrimSpace(parts[0])
		capabilityID = strings.TrimSpace(parts[1])
		if bundleID == "" || capabilityID == "" {
			resp.Diagnostics.AddError(
				"Invalid Import Identifier",
				fmt.Sprintf("Could not parse import ID %q. Use \"<bundle_id>/<capability_id>\", for example \"ABC123DEF4/XYZ789GHI0\".", importID),
			)
			return
		}
	} else {
		capabilityID = importID
	}

	// Get the capability from Apple API
	capability, err := r.client.GetBundleIDCapability(capabilityID)
	if err != nil {
		tflog.Error(ctx, "Failed to import Bundle ID Capability", map[string]interface{}{
			"import_id": importID,
			"error":     err.Error(),
		})
		resp.Diagnostics.AddError(
			"Bundle ID Capability Not Found",
			fmt.Sprintf("Could not find Bundle ID Capability with ID '%s'. Please verify the capability ID exists in your Apple Developer account. Error: %s", capabilityID, err.Error()),
		)
		return
	}

	// When a Bundle ID was supplied, confirm it actually owns this capability.
	// Importing a mismatched pair would write state that a later apply resolves
	// by destroying and recreating the capability.
	if bundleID != "" {
		capabilities, err := r.client.GetBundleIDCapabilities(bundleID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Verify Bundle ID",
				fmt.Sprintf("Could not list capabilities for Bundle ID '%s' to confirm it owns capability '%s': %s", bundleID, capabilityID, err.Error()),
			)
			return
		}

		owned := false
		for _, c := range capabilities {
			if c.ID == capabilityID {
				owned = true
				break
			}
		}

		if !owned {
			resp.Diagnostics.AddError(
				"Bundle ID Does Not Own This Capability",
				fmt.Sprintf("Capability '%s' is not a capability of Bundle ID '%s'. Check the import ID, which must be \"<bundle_id>/<capability_id>\".", capabilityID, bundleID),
			)
			return
		}
	}

	// Convert settings from API to Terraform format
	tfSettings, err := SettingsFromAPI(capability.Attributes.Settings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Settings",
			fmt.Sprintf("Could not convert capability settings from API response: %s", err.Error()),
		)
		return
	}

	// State may never hold an unknown value, so an unresolved bundle_id is
	// recorded as null and flagged below rather than left unknown.
	state := bundleIDCapabilityModel{
		ID:             types.StringValue(capability.ID),
		BundleID:       types.StringNull(),
		CapabilityType: types.StringValue(string(capability.Attributes.CapabilityType)),
		Settings:       tfSettings,
	}
	if bundleID != "" {
		state.BundleID = types.StringValue(bundleID)
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Bundle ID Capability imported successfully", map[string]interface{}{
		"capability_id":   capability.ID,
		"capability_type": string(capability.Attributes.CapabilityType),
	})

	if bundleID == "" {
		resp.Diagnostics.AddWarning(
			"Bundle ID Not Recorded During Import",
			fmt.Sprintf("Bundle ID Capability '%s' (type: %s) was imported without a bundle_id, because Apple's capability response does not include its parent Bundle ID. "+
				"The next plan will show bundle_id changing from null, which forces replacement. "+
				"Re-run the import as \"terraform import <address> <bundle_id>/%s\" to record it directly.", capability.ID, capability.Attributes.CapabilityType, capability.ID),
		)
	}
}

// Configure adds the provider configured client to the resource.
func (r *bundleIDCapabilityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apple.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *apple.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}
