// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package bundle

import (
	"context"
	"fmt"
	"strings"
	"terraform-provider-apple/internal/apple"
	"terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &bundleIDResource{}
	_ resource.ResourceWithConfigure   = &bundleIDResource{}
	_ resource.ResourceWithImportState = &bundleIDResource{}
)

func NewBundleIDResource() resource.Resource {
	return &bundleIDResource{}
}

type bundleIDResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *bundleIDResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bundle_id"
}

// Schema defines the schema for the resource.
func (r *bundleIDResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Apple App Store Connect Bundle ID.\n\n" +
			"Bundle IDs are used to identify apps and enable app services in the Apple ecosystem. " +
			"They follow a reverse domain name format (e.g., `com.example.myapp`).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the Bundle ID. Used for API references.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"identifier": schema.StringAttribute{
				MarkdownDescription: "The Bundle ID identifier string in reverse domain format (e.g., `com.example.myapp`). " +
					"This must be unique across your Apple Developer account and cannot be changed after creation.",
				Required:   true,
				Validators: GetIdentifierValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "A human-readable name for the Bundle ID. This can be updated after creation.",
				Required:            true,
				Validators:          GetNameValidator(),
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "The platform for the Bundle ID. Valid values are:\n" +
					"- `IOS` - iOS platform\n" +
					"- `MAC_OS` - macOS platform\n" +
					"- `TV_OS` - tvOS platform\n" +
					"- `WATCH_OS` - watchOS platform\n\n" +
					"This cannot be changed after creation.",
				Required:   true,
				Validators: []validator.String{PlatformValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"seed_id": schema.StringAttribute{
				MarkdownDescription: "The seed ID for the Bundle ID, automatically assigned by Apple. " +
					"This represents the team or organization identifier prefix.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create a new resource.
func (r *bundleIDResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating Bundle ID resource")

	// Retrieve values from plan
	var plan bundleIDModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add structured logging
	ctx = tflog.SetField(ctx, "bundle_identifier", plan.Identifier.ValueString())
	ctx = tflog.SetField(ctx, "bundle_name", plan.Name.ValueString())
	ctx = tflog.SetField(ctx, "bundle_platform", plan.Platform.ValueString())

	tflog.Debug(ctx, "Creating Bundle ID with Apple API")

	// Create new Bundle ID
	bundleID, err := r.client.CreateBundleID(
		plan.Identifier.ValueString(),
		plan.Name.ValueString(),
		models.BundleIDPlatform(plan.Platform.ValueString()),
		nil,
	)
	if err != nil {
		// Provide specific error messages for common issues
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			resp.Diagnostics.AddError(
				"Bundle ID Already Exists",
				fmt.Sprintf("A Bundle ID with identifier '%s' already exists in your Apple Developer account. Bundle ID identifiers must be unique.", plan.Identifier.ValueString()),
			)
		} else if strings.Contains(errMsg, "invalid identifier") {
			resp.Diagnostics.AddError(
				"Invalid Bundle ID Identifier",
				fmt.Sprintf("The Bundle ID identifier '%s' is invalid. Please ensure it follows the reverse domain format (e.g., com.example.myapp).", plan.Identifier.ValueString()),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error Creating Bundle ID",
				fmt.Sprintf("Could not create Bundle ID with identifier '%s': %s", plan.Identifier.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create Bundle ID", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Map response body to schema and populate computed attribute values
	plan.ID = types.StringValue(bundleID.ID)
	plan.SeedID = types.StringValue(bundleID.Attributes.SeedID)

	tflog.Info(ctx, "Bundle ID created successfully", map[string]interface{}{
		"bundle_id": bundleID.ID,
		"seed_id":   bundleID.Attributes.SeedID,
	})

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *bundleIDResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading Bundle ID resource")

	// Get current state
	var state bundleIDModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "bundle_id", state.ID.ValueString())

	// Get refreshed Bundle ID value from Apple
	bundleID, err := r.client.GetBundleID(state.ID.ValueString())
	if err != nil {
		// Handle 404 - resource no longer exists
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, "Bundle ID not found, removing from state")
			resp.Diagnostics.AddWarning(
				"Bundle ID Not Found",
				fmt.Sprintf("Bundle ID %s was not found and will be removed from Terraform state. It may have been deleted outside of Terraform.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}

		tflog.Error(ctx, "Failed to read Bundle ID", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Reading Bundle ID",
			fmt.Sprintf("Could not read Bundle ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Overwrite Bundle ID with refreshed state
	state.ID = types.StringValue(bundleID.ID)
	state.Identifier = types.StringValue(bundleID.Attributes.Identifier)
	state.Name = types.StringValue(bundleID.Attributes.Name)
	state.Platform = types.StringValue(string(bundleID.Attributes.Platform))
	state.SeedID = types.StringValue(bundleID.Attributes.SeedID)

	tflog.Debug(ctx, "Bundle ID read successfully")

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *bundleIDResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating Bundle ID resource")

	// Retrieve values from plan
	var plan bundleIDModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state bundleIDModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "bundle_id", state.ID.ValueString())
	ctx = tflog.SetField(ctx, "new_name", plan.Name.ValueString())

	tflog.Debug(ctx, "Updating Bundle ID with Apple API")

	// Update Bundle ID (only name can be updated)
	bundleID, err := r.client.UpdateBundleID(state.ID.ValueString(), plan.Name.ValueString(), nil)
	if err != nil {
		// Handle specific update errors
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			resp.Diagnostics.AddError(
				"Bundle ID Not Found",
				fmt.Sprintf("Bundle ID %s was not found during update. It may have been deleted outside of Terraform.", state.ID.ValueString()),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error Updating Bundle ID",
				fmt.Sprintf("Could not update Bundle ID %s: %s", state.ID.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to update Bundle ID", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Update state with updated values
	plan.ID = types.StringValue(bundleID.ID)
	plan.SeedID = types.StringValue(bundleID.Attributes.SeedID)

	tflog.Info(ctx, "Bundle ID updated successfully")

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *bundleIDResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting Bundle ID resource")

	// Retrieve values from state
	var state bundleIDModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "bundle_id", state.ID.ValueString())
	ctx = tflog.SetField(ctx, "bundle_identifier", state.Identifier.ValueString())

	tflog.Debug(ctx, "Deleting Bundle ID with Apple API")

	// Delete Bundle ID
	err := r.client.DeleteBundleID(state.ID.ValueString(), nil)
	if err != nil {
		// Handle specific delete errors
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			// Resource already deleted, this is not an error
			tflog.Warn(ctx, "Bundle ID already deleted outside of Terraform")
			resp.Diagnostics.AddWarning(
				"Bundle ID Already Deleted",
				fmt.Sprintf("Bundle ID %s was already deleted outside of Terraform.", state.ID.ValueString()),
			)
			return
		}

		tflog.Error(ctx, "Failed to delete Bundle ID", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Deleting Bundle ID",
			fmt.Sprintf("Could not delete Bundle ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Bundle ID deleted successfully")
}

// ImportState imports the resource into Terraform state.
func (r *bundleIDResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing Bundle ID resource", map[string]interface{}{
		"import_id": req.ID,
	})

	// Import supports both Bundle ID (Apple's internal ID) and Bundle identifier (user-friendly identifier)
	importID := strings.TrimSpace(req.ID)

	if importID == "" {
		resp.Diagnostics.AddError(
			"Empty Import Identifier",
			"The import identifier cannot be empty. Provide either the Bundle ID (Apple's internal ID) or the Bundle identifier (e.g., com.example.myapp).",
		)
		return
	}

	var bundleID *models.BundleID
	var err error

	// Try to import by Apple ID first (if it looks like an Apple ID)
	if strings.HasPrefix(importID, "bundle-") || len(importID) > 50 {
		tflog.Debug(ctx, "Attempting import by Apple Bundle ID")
		bundleID, err = r.client.GetBundleID(importID)
	} else {
		// Try to import by Bundle identifier (reverse domain format)
		tflog.Debug(ctx, "Attempting import by Bundle identifier")
		bundleID, err = r.client.GetBundleIDByIdentifier(importID)
	}

	if err != nil {
		tflog.Error(ctx, "Failed to import Bundle ID", map[string]interface{}{
			"import_id": importID,
			"error":     err.Error(),
		})
		resp.Diagnostics.AddError(
			"Bundle ID Not Found",
			fmt.Sprintf("Could not find Bundle ID with identifier '%s'. Please verify the identifier exists in your Apple Developer account. Error: %s", importID, err.Error()),
		)
		return
	}

	// Populate the state with the found Bundle ID
	state := bundleIDModel{
		ID:         types.StringValue(bundleID.ID),
		Identifier: types.StringValue(bundleID.Attributes.Identifier),
		Name:       types.StringValue(bundleID.Attributes.Name),
		Platform:   types.StringValue(string(bundleID.Attributes.Platform)),
		SeedID:     types.StringValue(bundleID.Attributes.SeedID),
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Bundle ID imported successfully", map[string]interface{}{
		"bundle_id":         bundleID.ID,
		"bundle_identifier": bundleID.Attributes.Identifier,
	})

	resp.Diagnostics.AddWarning(
		"Bundle ID Imported Successfully",
		fmt.Sprintf("Bundle ID '%s' (identifier: %s) has been imported. Please review the configuration and run 'terraform plan' to see any differences.", bundleID.ID, bundleID.Attributes.Identifier),
	)
}

// Configure adds the provider configured client to the resource.
func (r *bundleIDResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
