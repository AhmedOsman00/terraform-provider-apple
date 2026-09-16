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
	_ resource.Resource                = &subscriptionGroupLocalizationResource{}
	_ resource.ResourceWithConfigure   = &subscriptionGroupLocalizationResource{}
	_ resource.ResourceWithImportState = &subscriptionGroupLocalizationResource{}
)

func NewSubscriptionGroupLocalizationResource() resource.Resource {
	return &subscriptionGroupLocalizationResource{}
}

type subscriptionGroupLocalizationResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *subscriptionGroupLocalizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_group_localization"
}

// Schema defines the schema for the resource.
func (r *subscriptionGroupLocalizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the customer-facing name of a subscription group in one locale.\n\n" +
			"A subscription group's `reference_name` is internal to App Store Connect and customers never " +
			"see it. This resource holds what they do see: the heading above the list of plans when they " +
			"choose between the subscriptions in the group, and when they manage or change an existing " +
			"subscription in Settings.\n\n" +
			"It is a different record from `apple_subscription_localization`, which names an individual " +
			"subscription rather than the group it sits in — a group called “Pro” might hold “Pro Monthly” " +
			"and “Pro Yearly”.\n\n" +
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
			"group_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_subscription_group` this localization describes. " +
					"Cannot be changed after creation.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"locale": schema.StringAttribute{
				MarkdownDescription: "The App Store locale this localization applies to, for example `en-US`, " +
					"`ar-SA` or `es-MX`. Apple permits only one localization per locale per group. " +
					"Cannot be changed after creation — the locale identifies the record, and Apple's update " +
					"request has no locale member.",
				Required:   true,
				Validators: []validator.String{LocaleValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The group name customers see in this locale. Limited to 30 characters. " +
					"Can be updated in place, though a change on an approved group sends it back for review.",
				Required:   true,
				Validators: GetNameValidator(),
			},
			"custom_app_name": schema.StringAttribute{
				MarkdownDescription: "An alternative app name to display alongside the group in this locale, " +
					"limited to 30 characters. Optional — Apple falls back to the app's own name when it is " +
					"unset. Use it when the app's store name is not the name customers associate with the " +
					"subscription. Can be updated in place.",
				Optional:   true,
				Validators: GetCustomAppNameValidator(),
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
func (r *subscriptionGroupLocalizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating subscription group localization resource")

	var plan subscriptionGroupLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "subscription_group_id", plan.GroupID.ValueString())
	ctx = tflog.SetField(ctx, "locale", plan.Locale.ValueString())

	localization, err := r.client.CreateSubscriptionGroupLocalization(
		plan.GroupID.ValueString(),
		plan.Locale.ValueString(),
		plan.Name.ValueString(),
		stringOrNil(plan.CustomAppName),
		nil,
	)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "already exists"):
			resp.Diagnostics.AddError(
				"Localization Already Exists",
				fmt.Sprintf("Subscription group '%s' already has a localization for locale '%s'. Apple "+
					"permits only one per locale.", plan.GroupID.ValueString(), plan.Locale.ValueString()),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			resp.Diagnostics.AddError(
				"Subscription Group Not Found",
				fmt.Sprintf("Subscription group '%s' was not found.", plan.GroupID.ValueString()),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Creating Subscription Group Localization",
				fmt.Sprintf("Could not create localization for locale '%s': %s\n\n"+
					"If Apple reports the locale as invalid, check that the app itself is localized "+
					"into it — a subscription group cannot be localized into a locale the app does not "+
					"support.", plan.Locale.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create subscription group localization", map[string]interface{}{"error": err.Error()})
		return
	}

	plan.ID = types.StringValue(localization.ID)
	applyGroupLocalizationAttributes(&plan, localization)

	tflog.Info(ctx, "Subscription group localization created successfully", map[string]interface{}{
		"localization_id": localization.ID,
	})

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *subscriptionGroupLocalizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading subscription group localization resource")

	var state subscriptionGroupLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "localization_id", state.ID.ValueString())

	localization, err := r.client.GetSubscriptionGroupLocalization(state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription group localization not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Subscription Group Localization",
			fmt.Sprintf("Could not read localization '%s': %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	applyGroupLocalizationAttributes(&state, localization)

	if state.GroupID.IsNull() || state.GroupID.ValueString() == "" {
		if id := groupLocalizationParentID(localization); id != "" {
			state.GroupID = types.StringValue(id)
		}
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *subscriptionGroupLocalizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating subscription group localization resource")

	var plan subscriptionGroupLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "localization_id", plan.ID.ValueString())

	name := plan.Name.ValueString()
	localization, err := r.client.UpdateSubscriptionGroupLocalization(
		plan.ID.ValueString(),
		&name,
		stringOrNil(plan.CustomAppName),
		nil,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Subscription Group Localization",
			fmt.Sprintf("Could not update localization '%s': %s", plan.ID.ValueString(), err.Error()),
		)
		return
	}

	applyGroupLocalizationAttributes(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *subscriptionGroupLocalizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting subscription group localization resource")

	var state subscriptionGroupLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "localization_id", state.ID.ValueString())

	err := r.client.DeleteSubscriptionGroupLocalization(state.ID.ValueString(), nil)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404") {
			tflog.Info(ctx, "Subscription group localization already deleted")
			return
		}

		// Tolerated defensively, in case Apple applies to a group the rule an
		// in-app purchase version is under -- a version must keep at least one
		// localization. Erroring here would fail the whole destroy and leave
		// the group behind, and the record goes away with the group anyway, so
		// warn and drop it from state instead.
		if strings.Contains(errMsg, "last localization") {
			resp.Diagnostics.AddWarning(
				"Subscription Group Localization Not Deleted",
				fmt.Sprintf("Apple refused to delete localization '%s' because a subscription group must "+
					"keep at least one localization. It has been removed from Terraform state and will be "+
					"deleted along with the group itself.\n\nApple reported: %s", state.ID.ValueString(), errMsg),
			)
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Subscription Group Localization",
			fmt.Sprintf("Could not delete localization '%s': %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Subscription group localization deleted successfully")
}

// Configure adds the provider configured client to the resource.
func (r *subscriptionGroupLocalizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an existing group localization.
//
// A bare localization ID is accepted because GetSubscriptionGroupLocalization
// asks for include=subscriptionGroup, so the parent comes back with it. This is
// the one place in the group hierarchy where a parent can be read back: the
// group's own owning app cannot be, which is why apple_subscription_group
// insists on the composite form and this resource does not. The composite
// "<group_id>/<localization_id>" form is accepted too, and additionally
// confirms the localization belongs to the group named.
func (r *subscriptionGroupLocalizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing subscription group localization resource", map[string]interface{}{"import_id": req.ID})

	groupID, localizationID, composite := strings.Cut(req.ID, "/")
	if !composite {
		localizationID = req.ID
		groupID = ""
	}

	localization, err := r.client.GetSubscriptionGroupLocalization(localizationID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Subscription Group Localization",
			fmt.Sprintf("Could not import localization '%s': %s\n\n"+
				"Import ID should be either the localization ID on its own, or "+
				"\"<group_id>/<localization_id>\".", localizationID, err.Error()),
		)
		return
	}

	reported := groupLocalizationParentID(localization)

	if composite && reported != "" && reported != groupID {
		resp.Diagnostics.AddError(
			"Localization Not Owned By Subscription Group",
			fmt.Sprintf("Localization '%s' belongs to subscription group '%s', not '%s'.",
				localizationID, reported, groupID),
		)
		return
	}

	if groupID == "" {
		groupID = reported
	}
	if groupID == "" {
		resp.Diagnostics.AddError(
			"Incomplete Localization Import",
			fmt.Sprintf("Apple did not report a parent subscription group for localization '%s'. "+
				"Import it as \"<group_id>/<localization_id>\" instead.", localizationID),
		)
		return
	}

	state := subscriptionGroupLocalizationModel{
		ID:      types.StringValue(localization.ID),
		GroupID: types.StringValue(groupID),
	}
	applyGroupLocalizationAttributes(&state, localization)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// applyGroupLocalizationAttributes copies Apple's reported attributes into a
// model.
func applyGroupLocalizationAttributes(model *subscriptionGroupLocalizationModel, localization *models.SubscriptionGroupLocalization) {
	model.Locale = types.StringValue(localization.Attributes.Locale)
	model.Name = types.StringValue(localization.Attributes.Name)
	model.CustomAppName = stringOrNull(localization.Attributes.CustomAppName)

	if localization.Attributes.State != nil {
		model.State = types.StringValue(string(*localization.Attributes.State))
	} else {
		model.State = types.StringNull()
	}
}

// groupLocalizationParentID reads the owning group out of the relationship,
// returning "" when Apple did not populate the linkage.
func groupLocalizationParentID(localization *models.SubscriptionGroupLocalization) string {
	if localization.Relationships == nil || localization.Relationships.SubscriptionGroup == nil {
		return ""
	}
	return localization.Relationships.SubscriptionGroup.Data.ID
}
