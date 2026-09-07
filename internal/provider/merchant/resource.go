// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package merchant

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
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &merchantIDResource{}
	_ resource.ResourceWithConfigure   = &merchantIDResource{}
	_ resource.ResourceWithImportState = &merchantIDResource{}
)

func NewMerchantIDResource() resource.Resource {
	return &merchantIDResource{}
}

type merchantIDResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *merchantIDResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_merchant_id"
}

// Schema defines the schema for the resource.
func (r *merchantIDResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Apple App Store Connect Merchant ID.\n\n" +
			"Merchant IDs are used to identify merchants for Apple Pay transactions. " +
			"They follow a specific format starting with 'merchant.' (e.g., `merchant.com.example.myapp`).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the Merchant ID. Used for API references.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"identifier": schema.StringAttribute{
				MarkdownDescription: "The Merchant ID identifier string in merchant format (e.g., `merchant.com.example.myapp`). " +
					"This must be unique across your Apple Developer account and cannot be changed after creation.",
				Required:   true,
				Validators: GetIdentifierValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "A human-readable display name for the Merchant ID. This can be updated after creation.",
				Required:            true,
				Validators:          GetDisplayNameValidator(),
			},
		},
	}
}

// Create a new resource.
func (r *merchantIDResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating Merchant ID resource")

	// Retrieve values from plan
	var plan merchantIDModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add structured logging
	ctx = tflog.SetField(ctx, "merchant_identifier", plan.Identifier.ValueString())
	ctx = tflog.SetField(ctx, "merchant_display_name", plan.DisplayName.ValueString())

	tflog.Debug(ctx, "Creating Merchant ID with Apple API")

	// Create new Merchant ID
	merchantID, err := r.client.CreateMerchantID(
		plan.Identifier.ValueString(),
		plan.DisplayName.ValueString(),
		nil,
	)
	if err != nil {
		// Provide specific error messages for common issues
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			resp.Diagnostics.AddError(
				"Merchant ID Already Exists",
				fmt.Sprintf("A Merchant ID with identifier '%s' already exists in your Apple Developer account. Merchant ID identifiers must be unique.", plan.Identifier.ValueString()),
			)
		} else if strings.Contains(errMsg, "invalid identifier") {
			resp.Diagnostics.AddError(
				"Invalid Merchant ID Identifier",
				fmt.Sprintf("The Merchant ID identifier '%s' is invalid. Please ensure it follows the merchant format (e.g., merchant.com.example.myapp).", plan.Identifier.ValueString()),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error Creating Merchant ID",
				fmt.Sprintf("Could not create Merchant ID with identifier '%s': %s", plan.Identifier.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create Merchant ID", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Map response body to schema and populate computed attribute values
	plan.ID = types.StringValue(merchantID.ID)

	tflog.Info(ctx, "Merchant ID created successfully", map[string]interface{}{
		"merchant_id": merchantID.ID,
	})

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to set state after creating Merchant ID")
		return
	}

	tflog.Debug(ctx, "Merchant ID resource creation completed")
}

// Read refreshes the Terraform state with the latest data.
func (r *merchantIDResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading Merchant ID resource")

	// Get current state
	var state merchantIDModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "merchant_id", state.ID.ValueString())
	ctx = tflog.SetField(ctx, "merchant_identifier", state.Identifier.ValueString())

	// Get refreshed Merchant ID from Apple API
	merchantID, err := r.client.GetMerchantID(state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			// Merchant ID was deleted outside of Terraform
			tflog.Info(ctx, "Merchant ID not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Merchant ID",
			fmt.Sprintf("Could not read Merchant ID with ID '%s': %s", state.ID.ValueString(), err.Error()),
		)
		tflog.Error(ctx, "Failed to read Merchant ID", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Overwrite state with refreshed data
	state.Identifier = types.StringValue(merchantID.Attributes.Identifier)
	state.DisplayName = types.StringValue(merchantID.Attributes.DisplayName)

	tflog.Debug(ctx, "Read Merchant ID from API", map[string]interface{}{
		"identifier":   merchantID.Attributes.Identifier,
		"display_name": merchantID.Attributes.DisplayName,
	})

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to set state after reading Merchant ID")
		return
	}

	tflog.Debug(ctx, "Merchant ID resource read completed")
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *merchantIDResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating Merchant ID resource")

	// Retrieve values from plan
	var plan merchantIDModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "merchant_id", plan.ID.ValueString())
	ctx = tflog.SetField(ctx, "merchant_identifier", plan.Identifier.ValueString())
	ctx = tflog.SetField(ctx, "new_display_name", plan.DisplayName.ValueString())

	tflog.Debug(ctx, "Updating Merchant ID with Apple API")

	// Update Merchant ID
	merchantID, err := r.client.UpdateMerchantID(
		plan.ID.ValueString(),
		plan.DisplayName.ValueString(),
		nil,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Merchant ID",
			fmt.Sprintf("Could not update Merchant ID with ID '%s': %s", plan.ID.ValueString(), err.Error()),
		)
		tflog.Error(ctx, "Failed to update Merchant ID", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Update plan with refreshed data from API
	plan.DisplayName = types.StringValue(merchantID.Attributes.DisplayName)

	tflog.Info(ctx, "Merchant ID updated successfully", map[string]interface{}{
		"merchant_id":  merchantID.ID,
		"display_name": merchantID.Attributes.DisplayName,
	})

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to set state after updating Merchant ID")
		return
	}

	tflog.Debug(ctx, "Merchant ID resource update completed")
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *merchantIDResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting Merchant ID resource")

	// Retrieve values from state
	var state merchantIDModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "merchant_id", state.ID.ValueString())
	ctx = tflog.SetField(ctx, "merchant_identifier", state.Identifier.ValueString())

	tflog.Debug(ctx, "Deleting Merchant ID with Apple API")

	// Delete existing Merchant ID
	err := r.client.DeleteMerchantID(state.ID.ValueString(), nil)
	if err != nil {
		// If the Merchant ID is already gone, that's fine
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Merchant ID already deleted")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Merchant ID",
			fmt.Sprintf("Could not delete Merchant ID with ID '%s': %s", state.ID.ValueString(), err.Error()),
		)
		tflog.Error(ctx, "Failed to delete Merchant ID", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	tflog.Info(ctx, "Merchant ID deleted successfully")
	tflog.Debug(ctx, "Merchant ID resource deletion completed")
}

// Configure adds the provider configured client to the resource.
func (r *merchantIDResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

// ImportState imports an existing resource by ID or identifier.
func (r *merchantIDResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing Merchant ID resource", map[string]interface{}{
		"import_id": req.ID,
	})

	// The ID can be either the Apple ID or the merchant identifier
	importID := req.ID

	var merchantID *models.MerchantID
	var err error

	// Try to get by Apple ID first (format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	if strings.Contains(importID, "-") && len(importID) == 36 {
		tflog.Debug(ctx, "Attempting import by Apple ID")
		merchantID, err = r.client.GetMerchantID(importID)
	} else {
		tflog.Debug(ctx, "Attempting import by identifier")
		merchantID, err = r.client.GetMerchantIDByIdentifier(importID)
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Merchant ID",
			fmt.Sprintf("Could not import Merchant ID '%s': %s\n\n"+
				"Import ID should be either:\n"+
				"- The Apple-generated Merchant ID (e.g., 12345678-1234-1234-1234-123456789012)\n"+
				"- The merchant identifier (e.g., merchant.com.example.myapp)",
				importID, err.Error()),
		)
		return
	}

	// Set the resource state
	state := merchantIDModel{
		ID:          types.StringValue(merchantID.ID),
		Identifier:  types.StringValue(merchantID.Attributes.Identifier),
		DisplayName: types.StringValue(merchantID.Attributes.DisplayName),
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to set state during import")
		return
	}

	tflog.Info(ctx, "Merchant ID resource imported successfully", map[string]interface{}{
		"merchant_id":  merchantID.ID,
		"identifier":   merchantID.Attributes.Identifier,
		"display_name": merchantID.Attributes.DisplayName,
	})
}
