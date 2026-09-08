// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &subscriptionGroupResource{}
	_ resource.ResourceWithConfigure   = &subscriptionGroupResource{}
	_ resource.ResourceWithImportState = &subscriptionGroupResource{}
)

func NewSubscriptionGroupResource() resource.Resource {
	return &subscriptionGroupResource{}
}

type subscriptionGroupResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *subscriptionGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_group"
}

// Schema defines the schema for the resource.
func (r *subscriptionGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an App Store Connect subscription group.\n\n" +
			"A subscription group holds the auto-renewable subscriptions a customer chooses between. " +
			"Only one subscription in a group can be active at a time, which is what makes upgrades, " +
			"downgrades and crossgrades possible: a customer moving between levels of the same group " +
			"keeps one entitlement rather than accumulating several.\n\n" +
			"The group belongs to an app, and Apple's API cannot create app records — its documentation " +
			"says to create new apps on the App Store Connect website. Look the app up with the " +
			"`apple_apps` data source and pass its ID here.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the subscription group.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app that owns this subscription group. " +
					"Obtain it from the `apple_apps` data source. This cannot be changed after creation — " +
					"Apple's update request has no app member, so moving a group between apps is not possible.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"reference_name": schema.StringAttribute{
				MarkdownDescription: "The internal name of the subscription group, shown in App Store Connect " +
					"and never to customers. What customers see is a subscription group localization, which " +
					"this provider does not manage. Can be updated in place.",
				Required:   true,
				Validators: GetReferenceNameValidator(),
			},
		},
	}
}

// Create a new resource.
func (r *subscriptionGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating subscription group resource")

	var plan subscriptionGroupModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "app_id", plan.AppID.ValueString())
	ctx = tflog.SetField(ctx, "reference_name", plan.ReferenceName.ValueString())

	tflog.Debug(ctx, "Creating subscription group with Apple API")

	group, err := r.client.CreateSubscriptionGroup(
		plan.AppID.ValueString(),
		plan.ReferenceName.ValueString(),
		nil,
	)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "already exists"):
			resp.Diagnostics.AddError(
				"Subscription Group Already Exists",
				fmt.Sprintf("A subscription group named '%s' already exists on app '%s'.",
					plan.ReferenceName.ValueString(), plan.AppID.ValueString()),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			resp.Diagnostics.AddError(
				"App Not Found",
				fmt.Sprintf("App '%s' was not found. Verify the app ID with the apple_apps data source — "+
					"it is Apple's numeric app ID, not the bundle identifier.", plan.AppID.ValueString()),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Creating Subscription Group",
				fmt.Sprintf("Could not create subscription group '%s': %s", plan.ReferenceName.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create subscription group", map[string]interface{}{"error": err.Error()})
		return
	}

	plan.ID = types.StringValue(group.ID)
	plan.ReferenceName = types.StringValue(group.Attributes.ReferenceName)

	tflog.Info(ctx, "Subscription group created successfully", map[string]interface{}{
		"subscription_group_id": group.ID,
	})

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *subscriptionGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading subscription group resource")

	var state subscriptionGroupModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "subscription_group_id", state.ID.ValueString())

	group, err := r.client.GetSubscriptionGroup(state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription group not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Subscription Group",
			fmt.Sprintf("Could not read subscription group '%s': %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// app_id is deliberately not refreshed. GET /v1/subscriptionGroups/{id}
	// accepts no "app" value for its include parameter, so Apple never reports
	// the owning app -- assigning from the response would blank a value only
	// the configuration holds, and app_id forces replacement, so the next plan
	// would destroy the group and every subscription under it.
	state.ReferenceName = types.StringValue(group.Attributes.ReferenceName)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *subscriptionGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating subscription group resource")

	var plan subscriptionGroupModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "subscription_group_id", plan.ID.ValueString())

	group, err := r.client.UpdateSubscriptionGroup(
		plan.ID.ValueString(),
		plan.ReferenceName.ValueString(),
		nil,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Subscription Group",
			fmt.Sprintf("Could not update subscription group '%s': %s", plan.ID.ValueString(), err.Error()),
		)
		return
	}

	plan.ReferenceName = types.StringValue(group.Attributes.ReferenceName)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *subscriptionGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting subscription group resource")

	var state subscriptionGroupModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "subscription_group_id", state.ID.ValueString())

	err := r.client.DeleteSubscriptionGroup(state.ID.ValueString(), nil)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404") {
			tflog.Info(ctx, "Subscription group already deleted")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Subscription Group",
			fmt.Sprintf("Could not delete subscription group '%s': %s\n\n"+
				"Apple refuses to delete a group that still contains a subscription which has been "+
				"approved. Such a subscription can be removed from sale but never deleted, and the "+
				"group outlives it.", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Subscription group deleted successfully")
}

// Configure adds the provider configured client to the resource.
func (r *subscriptionGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an existing subscription group.
//
// The composite "<app_id>/<group_id>" form is the only one that yields complete
// state: Apple never reports a group's owning app, so a bare group ID would
// leave app_id null and the next plan would see a change to a
// replacement-forcing attribute.
func (r *subscriptionGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing subscription group resource", map[string]interface{}{"import_id": req.ID})

	appID, groupID, found := strings.Cut(req.ID, "/")
	if !found || appID == "" || groupID == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Subscription group import ID '%s' is not in the expected form.\n\n"+
				"Use \"<app_id>/<subscription_group_id>\", for example \"6448459855/21451234\".\n\n"+
				"The app ID is required because Apple's API never reports which app a subscription "+
				"group belongs to: GET /v1/subscriptionGroups/{id} accepts no \"app\" value for its "+
				"include parameter. Find the app ID with the apple_apps data source.", req.ID),
		)
		return
	}

	group, err := r.client.GetSubscriptionGroup(groupID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Subscription Group",
			fmt.Sprintf("Could not read subscription group '%s': %s", groupID, err.Error()),
		)
		return
	}

	// Confirm the group really belongs to the app named in the import ID,
	// rather than trusting what was typed: the group is otherwise written into
	// state under an app that may not own it.
	groups, err := r.client.GetSubscriptionGroups(appID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Subscription Group",
			fmt.Sprintf("Could not list subscription groups for app '%s' to confirm ownership: %s", appID, err.Error()),
		)
		return
	}

	owned := false
	for _, candidate := range groups {
		if candidate.ID == groupID {
			owned = true
			break
		}
	}
	if !owned {
		resp.Diagnostics.AddError(
			"Subscription Group Not Owned By App",
			fmt.Sprintf("Subscription group '%s' exists but does not belong to app '%s'. "+
				"Check the app ID with the apple_apps data source.", groupID, appID),
		)
		return
	}

	state := subscriptionGroupModel{
		ID:            types.StringValue(group.ID),
		AppID:         types.StringValue(appID),
		ReferenceName: types.StringValue(group.Attributes.ReferenceName),
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
