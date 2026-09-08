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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &subscriptionResource{}
	_ resource.ResourceWithConfigure   = &subscriptionResource{}
	_ resource.ResourceWithImportState = &subscriptionResource{}
)

func NewSubscriptionResource() resource.Resource {
	return &subscriptionResource{}
}

type subscriptionResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *subscriptionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription"
}

// Schema defines the schema for the resource.
func (r *subscriptionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an auto-renewable subscription in an App Store Connect subscription group.\n\n" +
			"A subscription created here is not yet sellable. Apple reports it as `MISSING_METADATA` until " +
			"it has at least one `apple_subscription_localization` and at least one `apple_subscription_price`, " +
			"which are separate resources because Apple models them as separate records — a subscription " +
			"carries no name or price of its own.\n\n" +
			"This resource does not submit the subscription for review. Moving it from `READY_TO_SUBMIT` " +
			"into review remains a manual step in App Store Connect.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the subscription.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_subscription_group` this subscription belongs to. " +
					"Cannot be changed after creation: Apple's update request has no group member, so a " +
					"subscription cannot be moved between groups.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"product_id": schema.StringAttribute{
				MarkdownDescription: "The product identifier the app passes to StoreKit, for example " +
					"`com.example.app.pro.monthly`. Must be unique across the entire Apple Developer account, " +
					"not merely within the app. Cannot be changed after creation — Apple's update request " +
					"has no `productId` member — and Apple never releases an identifier for reuse, so a " +
					"replacement needs a new one.",
				Required:   true,
				Validators: GetProductIDValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The reference name of the subscription, used in App Store Connect and " +
					"in sales reports. Limited to 30 characters. This is not what customers see — the " +
					"customer-facing name is the `name` on `apple_subscription_localization`. Can be updated in place.",
				Required:   true,
				Validators: GetNameValidator(),
			},
			"subscription_period": schema.StringAttribute{
				MarkdownDescription: "How often the subscription renews. One of `ONE_WEEK`, `ONE_MONTH`, " +
					"`TWO_MONTHS`, `THREE_MONTHS`, `SIX_MONTHS`, `ONE_YEAR`. These are the only durations " +
					"the App Store offers. Can be updated in place, though a change affects only new " +
					"subscribers — existing ones stay on the duration they bought.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{SubscriptionPeriodValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"family_sharable": schema.BoolAttribute{
				MarkdownDescription: "Whether the subscription can be shared with the customer's Family Sharing group. " +
					"Defaults to `false`. Can be updated in place, but note that Apple treats turning it off " +
					"as a change affecting existing subscribers.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"group_level": schema.Int64Attribute{
				MarkdownDescription: "The rank of this subscription within its group, where 1 is the highest " +
					"service level. The level decides whether a move between subscriptions is an upgrade, " +
					"a downgrade or a crossgrade, which in turn decides whether it takes effect immediately " +
					"or at the next renewal. Apple assigns 1 when this is omitted.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"review_note": schema.StringAttribute{
				MarkdownDescription: "A note for App Review explaining how to reach and test the subscription. " +
					"Can be updated in place.",
				Optional: true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Apple's review state for the subscription: `MISSING_METADATA`, " +
					"`READY_TO_SUBMIT`, `WAITING_FOR_REVIEW`, `IN_REVIEW`, `DEVELOPER_ACTION_NEEDED`, " +
					"`PENDING_BINARY_APPROVAL`, `APPROVED`, `DEVELOPER_REMOVED_FROM_SALE`, " +
					"`REMOVED_FROM_SALE` or `REJECTED`. Computed by Apple and never sent — a newly created " +
					"subscription reports `MISSING_METADATA` until a localization and a price exist.",
				Computed: true,
			},
		},
	}
}

// Create a new resource.
func (r *subscriptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating subscription resource")

	var plan subscriptionModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "group_id", plan.GroupID.ValueString())
	ctx = tflog.SetField(ctx, "product_id", plan.ProductID.ValueString())

	attributes := models.SubscriptionCreateAttributes{
		Name:           plan.Name.ValueString(),
		ProductID:      plan.ProductID.ValueString(),
		FamilySharable: boolOrNil(plan.FamilySharable),
		ReviewNote:     stringOrNil(plan.ReviewNote),
		GroupLevel:     intOrNil(plan.GroupLevel),
	}
	if period := stringOrNil(plan.SubscriptionPeriod); period != nil {
		p := models.SubscriptionPeriod(*period)
		attributes.SubscriptionPeriod = &p
	}

	tflog.Debug(ctx, "Creating subscription with Apple API")

	subscription, err := r.client.CreateSubscription(plan.GroupID.ValueString(), attributes, nil)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "not unique"):
			resp.Diagnostics.AddError(
				"Subscription Product ID Already Exists",
				fmt.Sprintf("The product ID '%s' is already in use. Product identifiers must be unique "+
					"across the whole Apple Developer account, and Apple never releases one for reuse — "+
					"even a deleted subscription keeps its identifier reserved.", plan.ProductID.ValueString()),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			resp.Diagnostics.AddError(
				"Subscription Group Not Found",
				fmt.Sprintf("Subscription group '%s' was not found.", plan.GroupID.ValueString()),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Creating Subscription",
				fmt.Sprintf("Could not create subscription '%s': %s", plan.ProductID.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create subscription", map[string]interface{}{"error": err.Error()})
		return
	}

	plan.ID = types.StringValue(subscription.ID)
	applySubscriptionAttributes(&plan, subscription)

	tflog.Info(ctx, "Subscription created successfully", map[string]interface{}{
		"subscription_id": subscription.ID,
		"state":           plan.State.ValueString(),
	})

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *subscriptionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading subscription resource")

	var state subscriptionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "subscription_id", state.ID.ValueString())

	subscription, err := r.client.GetSubscription(state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Subscription",
			fmt.Sprintf("Could not read subscription '%s': %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	applySubscriptionAttributes(&state, subscription)

	// The group linkage is populated because GetSubscription asks for
	// include=group, but only adopt it when state has none -- an import. A
	// configured group_id forces replacement, so overwriting it with anything
	// Apple reports differently would destroy the subscription.
	if state.GroupID.IsNull() || state.GroupID.ValueString() == "" {
		if subscription.Relationships != nil && subscription.Relationships.Group != nil {
			if groupID := subscription.Relationships.Group.Data.ID; groupID != "" {
				state.GroupID = types.StringValue(groupID)
			}
		}
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *subscriptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating subscription resource")

	var plan subscriptionModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "subscription_id", plan.ID.ValueString())

	name := plan.Name.ValueString()
	attributes := models.SubscriptionUpdateAttributes{
		Name:           &name,
		FamilySharable: boolOrNil(plan.FamilySharable),
		ReviewNote:     stringOrNil(plan.ReviewNote),
		GroupLevel:     intOrNil(plan.GroupLevel),
	}
	if period := stringOrNil(plan.SubscriptionPeriod); period != nil {
		p := models.SubscriptionPeriod(*period)
		attributes.SubscriptionPeriod = &p
	}

	subscription, err := r.client.UpdateSubscription(plan.ID.ValueString(), attributes, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Subscription",
			fmt.Sprintf("Could not update subscription '%s': %s", plan.ID.ValueString(), err.Error()),
		)
		return
	}

	applySubscriptionAttributes(&plan, subscription)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *subscriptionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting subscription resource")

	var state subscriptionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "subscription_id", state.ID.ValueString())

	err := r.client.DeleteSubscription(state.ID.ValueString(), nil)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404") {
			tflog.Info(ctx, "Subscription already deleted")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Subscription",
			fmt.Sprintf("Could not delete subscription '%s': %s\n\n"+
				"Apple only permits deleting a subscription that has never been approved. Once it has "+
				"been available for sale it can be removed from sale in App Store Connect but never "+
				"deleted, and its product ID stays reserved forever.", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Subscription deleted successfully")
}

// Configure adds the provider configured client to the resource.
func (r *subscriptionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an existing subscription by its Apple ID.
//
// A bare ID is enough here, unlike a subscription group: GetSubscription asks
// for include=group, so Apple reports the parent and state comes back complete.
func (r *subscriptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing subscription resource", map[string]interface{}{"import_id": req.ID})

	subscription, err := r.client.GetSubscription(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Subscription",
			fmt.Sprintf("Could not import subscription '%s': %s\n\n"+
				"The import ID is Apple's subscription ID, which appears in the App Store Connect URL "+
				"for the subscription.", req.ID, err.Error()),
		)
		return
	}

	state := subscriptionModel{ID: types.StringValue(subscription.ID)}
	applySubscriptionAttributes(&state, subscription)

	if subscription.Relationships != nil && subscription.Relationships.Group != nil {
		if groupID := subscription.Relationships.Group.Data.ID; groupID != "" {
			state.GroupID = types.StringValue(groupID)
		}
	}
	if state.GroupID.IsNull() {
		resp.Diagnostics.AddError(
			"Incomplete Subscription Import",
			fmt.Sprintf("Apple did not report a subscription group for subscription '%s', so group_id "+
				"cannot be written. Without it the next plan would replace the subscription.", req.ID),
		)
		return
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// applySubscriptionAttributes copies Apple's reported attributes into a model.
//
// review_note is left alone: it is Optional without Computed, so a null in
// configuration must stay null in state, and Apple omits the field entirely
// when it was never set.
func applySubscriptionAttributes(model *subscriptionModel, subscription *models.Subscription) {
	model.Name = types.StringValue(subscription.Attributes.Name)
	model.ProductID = types.StringValue(subscription.Attributes.ProductID)

	if subscription.Attributes.SubscriptionPeriod != nil {
		model.SubscriptionPeriod = types.StringValue(string(*subscription.Attributes.SubscriptionPeriod))
	} else {
		model.SubscriptionPeriod = types.StringNull()
	}

	if subscription.Attributes.FamilySharable != nil {
		model.FamilySharable = types.BoolValue(*subscription.Attributes.FamilySharable)
	}

	if subscription.Attributes.GroupLevel != nil {
		model.GroupLevel = types.Int64Value(int64(*subscription.Attributes.GroupLevel))
	}

	if subscription.Attributes.State != nil {
		model.State = types.StringValue(string(*subscription.Attributes.State))
	} else {
		model.State = types.StringNull()
	}
}
