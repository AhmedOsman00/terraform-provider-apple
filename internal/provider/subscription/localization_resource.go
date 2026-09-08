// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &subscriptionLocalizationResource{}
	_ resource.ResourceWithConfigure   = &subscriptionLocalizationResource{}
	_ resource.ResourceWithImportState = &subscriptionLocalizationResource{}
)

func NewSubscriptionLocalizationResource() resource.Resource {
	return &subscriptionLocalizationResource{}
}

type subscriptionLocalizationResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *subscriptionLocalizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_localization"
}

// Schema defines the schema for the resource.
func (r *subscriptionLocalizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the customer-facing name and description of a subscription in one locale.\n\n" +
			"This is what a customer actually reads on the App Store and in the system purchase sheet — " +
			"the `name` on `apple_subscription` is an internal reference name that customers never see. " +
			"A subscription stays in `MISSING_METADATA` until it has at least one localization.\n\n" +
			"The locale must be one the app itself supports; Apple rejects a localization for a locale " +
			"the app has not been localized into.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the localization.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_subscription` this localization describes. " +
					"Cannot be changed after creation.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"locale": schema.StringAttribute{
				MarkdownDescription: "The App Store locale this localization applies to, for example `en-US`, " +
					"`ar-SA` or `es-MX`. Apple permits only one localization per locale per subscription. " +
					"Cannot be changed after creation — the locale identifies the record, and Apple's update " +
					"request has no locale member.",
				Required:   true,
				Validators: []validator.String{LocaleValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The subscription name customers see in this locale. Limited to 30 characters. " +
					"Can be updated in place, though a change on an approved subscription sends it back for review.",
				Required:   true,
				Validators: GetNameValidator(),
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The subscription description customers see in this locale. Limited to " +
					"45 characters. Optional, but Apple's review expects one on anything customer-facing. " +
					"Can be updated in place.",
				Optional:   true,
				Validators: GetDescriptionValidator(),
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Apple's review state for this localization: `PREPARE_FOR_SUBMISSION`, " +
					"`WAITING_FOR_REVIEW`, `APPROVED` or `REJECTED`. Computed by Apple.",
				Computed: true,
			},
		},
	}
}

// Create a new resource.
func (r *subscriptionLocalizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating subscription localization resource")

	var plan subscriptionLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "subscription_id", plan.SubscriptionID.ValueString())
	ctx = tflog.SetField(ctx, "locale", plan.Locale.ValueString())

	localization, err := r.client.CreateSubscriptionLocalization(
		plan.SubscriptionID.ValueString(),
		plan.Locale.ValueString(),
		plan.Name.ValueString(),
		stringOrNil(plan.Description),
		nil,
	)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "already exists"):
			resp.Diagnostics.AddError(
				"Localization Already Exists",
				fmt.Sprintf("Subscription '%s' already has a localization for locale '%s'. Apple permits "+
					"only one per locale.", plan.SubscriptionID.ValueString(), plan.Locale.ValueString()),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			resp.Diagnostics.AddError(
				"Subscription Not Found",
				fmt.Sprintf("Subscription '%s' was not found.", plan.SubscriptionID.ValueString()),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Creating Subscription Localization",
				fmt.Sprintf("Could not create localization for locale '%s': %s\n\n"+
					"If Apple reports the locale as invalid, check that the app itself is localized "+
					"into it — a subscription cannot be localized into a locale the app does not support.",
					plan.Locale.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create subscription localization", map[string]interface{}{"error": err.Error()})
		return
	}

	plan.ID = types.StringValue(localization.ID)
	applyLocalizationAttributes(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *subscriptionLocalizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading subscription localization resource")

	var state subscriptionLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "localization_id", state.ID.ValueString())

	localization, err := r.client.GetSubscriptionLocalization(state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription localization not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Subscription Localization",
			fmt.Sprintf("Could not read localization '%s': %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	applyLocalizationAttributes(&state, localization)

	if state.SubscriptionID.IsNull() || state.SubscriptionID.ValueString() == "" {
		if localization.Relationships != nil && localization.Relationships.Subscription != nil {
			if id := localization.Relationships.Subscription.Data.ID; id != "" {
				state.SubscriptionID = types.StringValue(id)
			}
		}
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *subscriptionLocalizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating subscription localization resource")

	var plan subscriptionLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "localization_id", plan.ID.ValueString())

	name := plan.Name.ValueString()
	localization, err := r.client.UpdateSubscriptionLocalization(
		plan.ID.ValueString(),
		&name,
		stringOrNil(plan.Description),
		nil,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Subscription Localization",
			fmt.Sprintf("Could not update localization '%s': %s", plan.ID.ValueString(), err.Error()),
		)
		return
	}

	applyLocalizationAttributes(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *subscriptionLocalizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting subscription localization resource")

	var state subscriptionLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "localization_id", state.ID.ValueString())

	err := r.client.DeleteSubscriptionLocalization(state.ID.ValueString(), nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription localization already deleted")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Subscription Localization",
			fmt.Sprintf("Could not delete localization '%s': %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Subscription localization deleted successfully")
}

// Configure adds the provider configured client to the resource.
func (r *subscriptionLocalizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an existing localization.
//
// A bare localization ID is accepted because GetSubscriptionLocalization asks
// for include=subscription, so the parent comes back with it. The composite
// "<subscription_id>/<localization_id>" form is accepted too, and additionally
// confirms the localization belongs to the subscription named.
func (r *subscriptionLocalizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing subscription localization resource", map[string]interface{}{"import_id": req.ID})

	subscriptionID, localizationID, composite := strings.Cut(req.ID, "/")
	if !composite {
		localizationID = req.ID
		subscriptionID = ""
	}

	localization, err := r.client.GetSubscriptionLocalization(localizationID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Subscription Localization",
			fmt.Sprintf("Could not import localization '%s': %s\n\n"+
				"Import ID should be either the localization ID on its own, or "+
				"\"<subscription_id>/<localization_id>\".", localizationID, err.Error()),
		)
		return
	}

	reported := ""
	if localization.Relationships != nil && localization.Relationships.Subscription != nil {
		reported = localization.Relationships.Subscription.Data.ID
	}

	if composite && reported != "" && reported != subscriptionID {
		resp.Diagnostics.AddError(
			"Localization Not Owned By Subscription",
			fmt.Sprintf("Localization '%s' belongs to subscription '%s', not '%s'.",
				localizationID, reported, subscriptionID),
		)
		return
	}

	if subscriptionID == "" {
		subscriptionID = reported
	}
	if subscriptionID == "" {
		resp.Diagnostics.AddError(
			"Incomplete Localization Import",
			fmt.Sprintf("Apple did not report a parent subscription for localization '%s'. "+
				"Import it as \"<subscription_id>/<localization_id>\" instead.", localizationID),
		)
		return
	}

	state := subscriptionLocalizationModel{
		ID:             types.StringValue(localization.ID),
		SubscriptionID: types.StringValue(subscriptionID),
	}
	applyLocalizationAttributes(&state, localization)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// applyLocalizationAttributes copies Apple's reported attributes into a model.
func applyLocalizationAttributes(model *subscriptionLocalizationModel, localization *models.SubscriptionLocalization) {
	model.Locale = types.StringValue(localization.Attributes.Locale)
	model.Name = types.StringValue(localization.Attributes.Name)
	model.Description = stringOrNull(localization.Attributes.Description)

	if localization.Attributes.State != nil {
		model.State = types.StringValue(string(*localization.Attributes.State))
	} else {
		model.State = types.StringNull()
	}
}
