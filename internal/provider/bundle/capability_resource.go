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
					"- `HEALTHKIT` - HealthKit data access\n" +
					"- `HOMEKIT` - HomeKit device control\n" +
					"- `SIRIKIT` - SiriKit integration\n" +
					"- `WALLET` - Wallet passes\n\n" +
					"Note the spelling: Apple writes HEALTHKIT, HOMEKIT, CLASSKIT and SIRIKIT without an " +
					"underscore, and Wallet as WALLET. Capabilities that Xcode configures rather than the " +
					"App Store Connect API -- APP_ATTEST, WEATHER_KIT, GROUP_ACTIVITIES and similar -- are " +
					"not accepted here. This cannot be changed after creation.",
				Required:   true,
				Validators: GetCapabilityTypeValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"settings": schema.ListNestedAttribute{
				MarkdownDescription: "Configuration settings for the capability. The available settings depend on the capability type: " +
					"`PUSH_NOTIFICATIONS` takes none, while `DATA_PROTECTION` and `ICLOUD` take one each.\n\n" +
					"Set `value` for a single-choice setting, which is the common case, or `options` for a setting that " +
					"takes several. The two are mutually exclusive -- Apple models both as an options list, and this " +
					"resource reports a setting with one option through `value`.\n\n" +
					"Apple's display metadata for a setting (its name, visibility and available choices) is not " +
					"configuration and is not recorded here; read it from the `apple_bundle_id_capabilities` data source.",
				Optional: true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "The setting key identifier, for example `DATA_PROTECTION_PERMISSION_LEVEL`.",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "The chosen value for a single-choice setting, for example `COMPLETE_PROTECTION`. " +
								"Mutually exclusive with `options`.",
							Optional: true,
						},
						"options": schema.ListNestedAttribute{
							MarkdownDescription: "The chosen values for a setting that takes more than one. " +
								"Mutually exclusive with `value`.",
							Optional: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"key": schema.StringAttribute{
										MarkdownDescription: "The option key identifier.",
										Required:            true,
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
	settings, err := SettingsInputToAPI(ctx, plan.Settings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Settings",
			fmt.Sprintf("Could not convert capability settings: %s", err.Error()),
		)
		return
	}

	// Serialized per Bundle ID: Apple loses one of two concurrent capability
	// writes to the same bundle. See capability_serialize.go.
	unlock := lockBundleCapabilities(plan.BundleID.ValueString())
	defer unlock()

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
	tfSettings, err := SettingsInputFromAPI(capability.Attributes.Settings)
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

	// A capability can only be read through its parent Bundle ID's collection.
	// State carries the parent for anything this provider created; a resource
	// imported by bare ID before that was recorded falls back to the ID's own
	// "<bundleID>_<TYPE>" prefix.
	bundleID := state.BundleID.ValueString()
	if bundleID == "" {
		bundleID = apple.BundleIDFromCapabilityID(state.ID.ValueString())
	}

	if bundleID == "" {
		resp.Diagnostics.AddError(
			"Unknown Parent Bundle ID",
			fmt.Sprintf("Bundle ID Capability %s has no bundle_id in state and none could be recovered from its ID. "+
				"Re-import it as \"<bundle_id>/<capability_id>\".", state.ID.ValueString()),
		)
		return
	}

	// Get refreshed Bundle ID Capability value from Apple
	capability, err := r.client.GetBundleIDCapability(bundleID, state.ID.ValueString())
	if err != nil {
		// Handle 404 - resource no longer exists. A capability absent from its
		// parent's collection reports "not found" the same way, and a deleted
		// parent Bundle ID 404s the list call, so both land here.
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
	tfSettings, err := SettingsInputFromAPI(capability.Attributes.Settings)
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
	settings, err := SettingsInputToAPI(ctx, plan.Settings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Settings",
			fmt.Sprintf("Could not convert capability settings: %s", err.Error()),
		)
		return
	}

	// Update Bundle ID Capability (only settings can be updated), serialized
	// per Bundle ID for the reason capability_serialize.go documents.
	unlock := lockBundleCapabilities(plan.BundleID.ValueString())
	defer unlock()

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
	tfSettings, err := SettingsInputFromAPI(capability.Attributes.Settings)
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

	// Delete Bundle ID Capability, serialized per Bundle ID: a delete is the
	// same read-modify-write over the bundle's capability set as a create.
	unlock := lockBundleCapabilities(state.BundleID.ValueString())
	defer unlock()

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

	// A bare capability ID still has to name its parent, because reading a
	// capability means listing the parent's collection. Apple embeds the parent
	// in the ID, so recover it from there -- and let the lookup below prove the
	// guess, since the format is undocumented.
	derivedBundleID := false
	if bundleID == "" {
		bundleID = apple.BundleIDFromCapabilityID(capabilityID)
		derivedBundleID = bundleID != ""
	}

	if bundleID == "" {
		resp.Diagnostics.AddError(
			"Invalid Import Identifier",
			fmt.Sprintf("Could not determine which Bundle ID owns capability '%s'. Apple does not allow reading a capability "+
				"on its own, so the parent is required: import as \"<bundle_id>/<capability_id>\".", capabilityID),
		)
		return
	}

	// Fetching through the parent's collection is itself the ownership check:
	// a capability the Bundle ID does not own is simply absent from the list.
	capability, err := r.client.GetBundleIDCapability(bundleID, capabilityID)
	if err != nil {
		tflog.Error(ctx, "Failed to import Bundle ID Capability", map[string]interface{}{
			"import_id": importID,
			"bundle_id": bundleID,
			"error":     err.Error(),
		})

		if derivedBundleID {
			resp.Diagnostics.AddError(
				"Bundle ID Capability Not Found",
				fmt.Sprintf("Could not find capability '%s' on Bundle ID '%s', which was inferred from the capability ID. "+
					"Import as \"<bundle_id>/<capability_id>\" to name the parent explicitly. Error: %s", capabilityID, bundleID, err.Error()),
			)
			return
		}

		resp.Diagnostics.AddError(
			"Bundle ID Capability Not Found",
			fmt.Sprintf("Could not find capability '%s' on Bundle ID '%s'. Verify both exist in your Apple Developer account "+
				"and that the Bundle ID owns this capability. Error: %s", capabilityID, bundleID, err.Error()),
		)
		return
	}

	// Convert settings from API to Terraform format
	tfSettings, err := SettingsInputFromAPI(capability.Attributes.Settings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Settings",
			fmt.Sprintf("Could not convert capability settings from API response: %s", err.Error()),
		)
		return
	}

	// bundle_id is always known by this point: it was either given in the
	// composite import ID or recovered from the capability ID and confirmed by
	// the lookup above. Import therefore never writes partial state, and the
	// next plan is empty rather than a forced replacement.
	state := bundleIDCapabilityModel{
		ID:             types.StringValue(capability.ID),
		BundleID:       types.StringValue(bundleID),
		CapabilityType: types.StringValue(string(capability.Attributes.CapabilityType)),
		Settings:       tfSettings,
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Bundle ID Capability imported successfully", map[string]interface{}{
		"capability_id":   capability.ID,
		"bundle_id":       bundleID,
		"capability_type": string(capability.Attributes.CapabilityType),
	})
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
