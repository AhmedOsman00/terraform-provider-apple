// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package passtypeid

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &passTypeIDResource{}
	_ resource.ResourceWithConfigure   = &passTypeIDResource{}
	_ resource.ResourceWithImportState = &passTypeIDResource{}
)

func NewPassTypeIDResource() resource.Resource {
	return &passTypeIDResource{}
}

type passTypeIDResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *passTypeIDResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pass_type_id"
}

// Schema defines the schema for the resource.
func (r *passTypeIDResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Apple Pass Type ID.\n\n" +
			"Pass Type IDs are used to identify Wallet passes and must start with 'pass.' followed by a reverse domain format " +
			"(e.g., `pass.com.example.mypass`). They are required for creating and distributing passes for Apple Wallet.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the Pass Type ID. Used for API references.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"identifier": schema.StringAttribute{
				MarkdownDescription: "The Pass Type ID identifier string starting with 'pass.' followed by reverse domain format " +
					"(e.g., `pass.com.example.mypass`). This must be unique across your Apple Developer account and cannot be changed after creation.",
				Required:   true,
				Validators: GetIdentifierValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "A human-readable name for the Pass Type ID. This can be updated after creation.",
				Required:            true,
				Validators:          GetNameValidator(),
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *passTypeIDResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create a new resource.
func (r *passTypeIDResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating Pass Type ID resource")

	// Retrieve values from plan
	var plan passTypeIDModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add structured logging
	ctx = tflog.SetField(ctx, "pass_type_id_identifier", plan.Identifier.ValueString())
	ctx = tflog.SetField(ctx, "pass_type_id_name", plan.Name.ValueString())

	tflog.Debug(ctx, "Creating Pass Type ID with Apple API")

	// Create new Pass Type ID
	passTypeID, err := r.client.CreatePassTypeID(
		plan.Identifier.ValueString(),
		plan.Name.ValueString(),
	)
	if err != nil {
		// Provide specific error messages for common issues
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			resp.Diagnostics.AddError(
				"Pass Type ID Already Exists",
				fmt.Sprintf("A Pass Type ID with identifier '%s' already exists in your Apple Developer account. Pass Type ID identifiers must be unique.", plan.Identifier.ValueString()),
			)
		} else if strings.Contains(errMsg, "invalid identifier") {
			resp.Diagnostics.AddError(
				"Invalid Pass Type ID Identifier",
				fmt.Sprintf("The Pass Type ID identifier '%s' is invalid. Please ensure it starts with 'pass.' followed by reverse domain format (e.g., pass.com.example.mypass).", plan.Identifier.ValueString()),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error Creating Pass Type ID",
				fmt.Sprintf("Could not create Pass Type ID with identifier '%s': %s", plan.Identifier.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create Pass Type ID", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Map response body to schema and populate computed attribute values
	plan.ID = types.StringValue(passTypeID.ID)

	tflog.Info(ctx, "Pass Type ID created successfully", map[string]interface{}{
		"pass_type_id": passTypeID.ID,
	})

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *passTypeIDResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "Reading Pass Type ID resource")

	// Get current state
	var state passTypeIDModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add structured logging
	ctx = tflog.SetField(ctx, "pass_type_id_id", state.ID.ValueString())

	tflog.Debug(ctx, "Reading Pass Type ID from Apple API")

	// Get refreshed Pass Type ID value from Apple
	passTypeID, err := r.client.GetPassTypeID(state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			// Pass Type ID was deleted outside Terraform
			tflog.Warn(ctx, "Pass Type ID not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Pass Type ID",
			fmt.Sprintf("Could not read Pass Type ID ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Overwrite items with refreshed state
	state.ID = types.StringValue(passTypeID.ID)
	state.Identifier = types.StringValue(passTypeID.Attributes.Identifier)
	state.Name = types.StringValue(passTypeID.Attributes.Name)

	tflog.Debug(ctx, "Pass Type ID read successfully")

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *passTypeIDResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating Pass Type ID resource")

	// Retrieve values from plan
	var plan passTypeIDModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add structured logging
	ctx = tflog.SetField(ctx, "pass_type_id_id", plan.ID.ValueString())
	ctx = tflog.SetField(ctx, "pass_type_id_name", plan.Name.ValueString())

	tflog.Debug(ctx, "Updating Pass Type ID with Apple API")

	// Update Pass Type ID
	passTypeID, err := r.client.UpdatePassTypeID(plan.ID.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Pass Type ID",
			fmt.Sprintf("Could not update Pass Type ID %s: %s", plan.ID.ValueString(), err.Error()),
		)
		return
	}

	// Update plan with latest values
	plan.Name = types.StringValue(passTypeID.Attributes.Name)

	tflog.Info(ctx, "Pass Type ID updated successfully")

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *passTypeIDResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting Pass Type ID resource")

	// Retrieve values from state
	var state passTypeIDModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add structured logging
	ctx = tflog.SetField(ctx, "pass_type_id_id", state.ID.ValueString())

	tflog.Debug(ctx, "Deleting Pass Type ID with Apple API")

	// Delete existing Pass Type ID
	err := r.client.DeletePassTypeID(state.ID.ValueString())
	if err != nil {
		// If the Pass Type ID is already gone, that's OK
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, "Pass Type ID already deleted")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Pass Type ID",
			fmt.Sprintf("Could not delete Pass Type ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Pass Type ID deleted successfully")
}

// ImportState imports existing Pass Type IDs into Terraform state.
// Supports import by both Apple ID (e.g., "ABC123XYZ") and identifier (e.g., "pass.com.example.mypass").
func (r *passTypeIDResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing Pass Type ID resource", map[string]interface{}{
		"import_id": req.ID,
	})

	importID := strings.TrimSpace(req.ID)
	if importID == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID cannot be empty. Use either the Apple ID (e.g., ABC123XYZ) or the identifier (e.g., pass.com.example.mypass).",
		)
		return
	}

	// Check if it looks like an Apple ID (alphanumeric, typically 8-10 characters)
	appleIDPattern := regexp.MustCompile(`^[A-Z0-9]{8,10}$`)
	var passTypeID *models.PassTypeIDResource
	var err error

	if appleIDPattern.MatchString(importID) {
		// Import by Apple ID
		tflog.Debug(ctx, "Importing Pass Type ID by Apple ID")
		passTypeID, err = r.client.GetPassTypeID(importID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Importing Pass Type ID by Apple ID",
				fmt.Sprintf("Could not retrieve Pass Type ID with Apple ID '%s': %s", importID, err.Error()),
			)
			return
		}
	} else {
		// Import by identifier - validate it looks like a pass type identifier
		passTypeIDPattern := regexp.MustCompile(`^pass\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`)
		if passTypeIDPattern.MatchString(importID) {
			tflog.Debug(ctx, "Importing Pass Type ID by identifier")
			passTypeID, err = r.client.GetPassTypeIDByIdentifier(importID)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Importing Pass Type ID by Identifier",
					fmt.Sprintf("Could not retrieve Pass Type ID with identifier '%s': %s. Make sure the Pass Type ID exists in your Apple Developer account.", importID, err.Error()),
				)
				return
			}
		} else {
			resp.Diagnostics.AddError(
				"Invalid Import ID Format",
				fmt.Sprintf("Import ID '%s' is not valid. Use either:\n"+
					"- Apple ID: 8-10 character alphanumeric string (e.g., ABC123XYZ)\n"+
					"- Pass Type ID identifier: starts with 'pass.' followed by reverse domain (e.g., pass.com.example.mypass)", importID),
			)
			return
		}
	}

	// Set the imported values
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), passTypeID.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("identifier"), passTypeID.Attributes.Identifier)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), passTypeID.Attributes.Name)...)

	tflog.Info(ctx, "Pass Type ID imported successfully", map[string]interface{}{
		"apple_id":   passTypeID.ID,
		"identifier": passTypeID.Attributes.Identifier,
		"name":       passTypeID.Attributes.Name,
	})
}
