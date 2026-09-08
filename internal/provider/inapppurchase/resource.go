// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package inapppurchase

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &inAppPurchaseResource{}
	_ resource.ResourceWithConfigure   = &inAppPurchaseResource{}
	_ resource.ResourceWithImportState = &inAppPurchaseResource{}
)

func NewInAppPurchaseResource() resource.Resource {
	return &inAppPurchaseResource{}
}

type inAppPurchaseResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *inAppPurchaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_in_app_purchase"
}

// Schema defines the schema for the resource.
func (r *inAppPurchaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a one-time in-app purchase on an App Store Connect app: a consumable, " +
			"a non-consumable, or a non-renewing subscription.\n\n" +
			"Auto-renewable subscriptions are a different Apple resource and are managed by " +
			"`apple_subscription`. Nothing is shared between the two, price points included.\n\n" +
			"A purchase created here is not yet sellable. Apple reports it as `MISSING_METADATA` " +
			"until it has an `apple_in_app_purchase_localization`, an " +
			"`apple_in_app_purchase_price_schedule` and an `apple_in_app_purchase_availability` — " +
			"separate resources because Apple models them as separate records.\n\n" +
			"This resource does not submit the purchase for review. Moving it from `READY_TO_SUBMIT` " +
			"into review remains a manual step in App Store Connect.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the in-app purchase.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app this purchase belongs to. Read it from the " +
					"`apple_apps` data source — Apple's API cannot create an app record, so the app must " +
					"already exist in App Store Connect.\n\n" +
					"Write-once and unreadable: Apple's in-app purchase resource has no app relationship, " +
					"so this value can never be refreshed and is kept from configuration. That is also why " +
					"import takes the composite `<app_id>/<in_app_purchase_id>` form.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"product_id": schema.StringAttribute{
				MarkdownDescription: "The product identifier the app passes to StoreKit, for example " +
					"`com.example.app.prounlock`. Must be unique across the entire Apple Developer account, " +
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
				MarkdownDescription: "The reference name of the purchase, used in App Store Connect and in " +
					"sales reports. Limited to 64 characters. This is not what customers see — the " +
					"customer-facing name is the `name` on `apple_in_app_purchase_localization`. " +
					"Can be updated in place.",
				Required:   true,
				Validators: GetReferenceNameValidator(),
			},
			"in_app_purchase_type": schema.StringAttribute{
				MarkdownDescription: "What kind of purchase this is. One of:\n\n" +
					"- `CONSUMABLE` — bought over and over, such as a pack of credits.\n" +
					"- `NON_CONSUMABLE` — bought once and owned forever, such as unlocking a pro feature.\n" +
					"- `NON_RENEWING_SUBSCRIPTION` — a fixed-term entitlement the customer has to buy " +
					"again by hand when it lapses.\n\n" +
					"Cannot be changed after creation: Apple's update request has no " +
					"`inAppPurchaseType` member.",
				Required:   true,
				Validators: []validator.String{TypeValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"family_sharable": schema.BoolAttribute{
				MarkdownDescription: "Whether the purchase can be shared with the customer's Family Sharing " +
					"group. Defaults to `false`. Can be updated in place, but Apple treats turning it off " +
					"as a change affecting existing customers.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"review_note": schema.StringAttribute{
				MarkdownDescription: "A note for App Review explaining how to reach and test the purchase. " +
					"Can be updated in place.",
				Optional: true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Apple's review state for the purchase: `MISSING_METADATA`, " +
					"`WAITING_FOR_UPLOAD`, `PROCESSING_CONTENT`, `READY_TO_SUBMIT`, `WAITING_FOR_REVIEW`, " +
					"`IN_REVIEW`, `DEVELOPER_ACTION_NEEDED`, `PENDING_BINARY_APPROVAL`, `APPROVED`, " +
					"`DEVELOPER_REMOVED_FROM_SALE`, `REMOVED_FROM_SALE` or `REJECTED`. Computed by Apple " +
					"and never sent — a newly created purchase reports `MISSING_METADATA` until its " +
					"metadata, price and availability all exist.",
				Computed: true,
			},
			"content_hosting": schema.BoolAttribute{
				MarkdownDescription: "Whether Apple hosts the downloadable content for this purchase. " +
					"Computed by Apple; the provider does not manage hosted content.",
				Computed: true,
			},
		},
	}
}

// Create a new resource.
func (r *inAppPurchaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating in-app purchase resource")

	var plan inAppPurchaseModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "app_id", plan.AppID.ValueString())
	ctx = tflog.SetField(ctx, "product_id", plan.ProductID.ValueString())

	attributes := models.InAppPurchaseCreateAttributes{
		Name:              plan.Name.ValueString(),
		ProductID:         plan.ProductID.ValueString(),
		InAppPurchaseType: models.InAppPurchaseType(plan.InAppPurchaseType.ValueString()),
		FamilySharable:    boolOrNil(plan.FamilySharable),
		ReviewNote:        stringOrNil(plan.ReviewNote),
	}

	tflog.Debug(ctx, "Creating in-app purchase with Apple API")

	purchase, err := r.client.CreateInAppPurchase(plan.AppID.ValueString(), attributes, nil)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "not unique"):
			resp.Diagnostics.AddError(
				"In-App Purchase Product ID Already Exists",
				fmt.Sprintf("The product ID '%s' is already in use. Product identifiers must be unique "+
					"across the whole Apple Developer account, and Apple never releases one for reuse — "+
					"even a deleted purchase keeps its identifier reserved.", plan.ProductID.ValueString()),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			resp.Diagnostics.AddError(
				"App Not Found",
				fmt.Sprintf("App '%s' was not found. Apple's API cannot create an app record — it has to "+
					"exist in App Store Connect first. Read its ID with the apple_apps data source.",
					plan.AppID.ValueString()),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Creating In-App Purchase",
				fmt.Sprintf("Could not create in-app purchase '%s': %s", plan.ProductID.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create in-app purchase", map[string]interface{}{"error": err.Error()})
		return
	}

	plan.ID = types.StringValue(purchase.ID)
	applyPurchaseAttributes(&plan, purchase)

	tflog.Info(ctx, "In-app purchase created successfully", map[string]interface{}{
		"in_app_purchase_id": purchase.ID,
		"state":              plan.State.ValueString(),
	})

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
//
// app_id is deliberately left alone: Apple never reports it, so state keeps
// what configuration supplied.
func (r *inAppPurchaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading in-app purchase resource")

	var state inAppPurchaseModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "in_app_purchase_id", state.ID.ValueString())

	purchase, err := r.client.GetInAppPurchase(state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "In-app purchase not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading In-App Purchase",
			fmt.Sprintf("Could not read in-app purchase '%s': %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	applyPurchaseAttributes(&state, purchase)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *inAppPurchaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating in-app purchase resource")

	var plan inAppPurchaseModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "in_app_purchase_id", plan.ID.ValueString())

	name := plan.Name.ValueString()
	attributes := models.InAppPurchaseUpdateAttributes{
		Name:           &name,
		FamilySharable: boolOrNil(plan.FamilySharable),
		ReviewNote:     stringOrNil(plan.ReviewNote),
	}

	purchase, err := r.client.UpdateInAppPurchase(plan.ID.ValueString(), attributes, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating In-App Purchase",
			fmt.Sprintf("Could not update in-app purchase '%s': %s", plan.ID.ValueString(), err.Error()),
		)
		return
	}

	applyPurchaseAttributes(&plan, purchase)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *inAppPurchaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting in-app purchase resource")

	var state inAppPurchaseModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "in_app_purchase_id", state.ID.ValueString())

	err := r.client.DeleteInAppPurchase(state.ID.ValueString(), nil)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404") {
			tflog.Info(ctx, "In-app purchase already deleted")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting In-App Purchase",
			fmt.Sprintf("Could not delete in-app purchase '%s': %s\n\n"+
				"Apple only permits deleting a purchase that has never been approved. Once it has been "+
				"available for sale it can be removed from sale in App Store Connect but never deleted, "+
				"and its product ID stays reserved forever either way.", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "In-app purchase deleted successfully")
}

// Configure adds the provider configured client to the resource.
func (r *inAppPurchaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an existing in-app purchase.
//
// Only the composite "<app_id>/<in_app_purchase_id>" form is accepted. Apple's
// in-app purchase resource has no app relationship and no include produces one,
// so a bare ID would leave app_id null -- and app_id forces replacement, so the
// next plan would destroy the purchase and reserve its product ID forever. The
// app is confirmed to own the purchase by listing its collection, which is the
// only ownership check the API offers.
func (r *inAppPurchaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing in-app purchase resource", map[string]interface{}{"import_id": req.ID})

	appID, purchaseID, found := strings.Cut(req.ID, "/")
	if !found || appID == "" || purchaseID == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("In-app purchase import ID '%s' is not in the expected form.\n\n"+
				"Use \"<app_id>/<in_app_purchase_id>\".\n\n"+
				"The app ID is required because Apple never reports which app an in-app purchase "+
				"belongs to: the resource has no app relationship. Without it, app_id would be null "+
				"and the next plan would replace the purchase.", req.ID),
		)
		return
	}

	purchase, err := r.client.GetInAppPurchase(purchaseID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing In-App Purchase",
			fmt.Sprintf("Could not import in-app purchase '%s': %s", purchaseID, err.Error()),
		)
		return
	}

	owned, err := r.client.InAppPurchaseBelongsToApp(appID, purchaseID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Confirming In-App Purchase Ownership",
			fmt.Sprintf("Could not list the in-app purchases of app '%s' to confirm it owns '%s': %s",
				appID, purchaseID, err.Error()),
		)
		return
	}
	if !owned {
		resp.Diagnostics.AddError(
			"In-App Purchase Not Owned By App",
			fmt.Sprintf("In-app purchase '%s' is not listed under app '%s'. Check the app ID — "+
				"importing with the wrong one would write state that destroys the purchase on the "+
				"next apply.", purchaseID, appID),
		)
		return
	}

	state := inAppPurchaseModel{
		ID:    types.StringValue(purchase.ID),
		AppID: types.StringValue(appID),
	}
	applyPurchaseAttributes(&state, purchase)

	// Unlike Read, an import has no configuration to contradict, so the review
	// note Apple holds is the only value there is. Leaving it null dropped it
	// silently and made the first plan after an import propose a change that
	// only restored what was already there.
	if purchase.Attributes.ReviewNote != nil {
		state.ReviewNote = types.StringValue(*purchase.Attributes.ReviewNote)
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// applyPurchaseAttributes copies Apple's reported attributes into a model.
//
// review_note is left alone: it is Optional without Computed, so a null in
// configuration must stay null in state, and Apple omits the field entirely
// when it was never set. ImportState sets it separately, where there is no
// configuration for a non-null value to disagree with.
func applyPurchaseAttributes(model *inAppPurchaseModel, purchase *models.InAppPurchase) {
	model.Name = types.StringValue(purchase.Attributes.Name)
	model.ProductID = types.StringValue(purchase.Attributes.ProductID)

	if purchase.Attributes.InAppPurchaseType != nil {
		model.InAppPurchaseType = types.StringValue(string(*purchase.Attributes.InAppPurchaseType))
	}

	if purchase.Attributes.FamilySharable != nil {
		model.FamilySharable = types.BoolValue(*purchase.Attributes.FamilySharable)
	}

	if purchase.Attributes.State != nil {
		model.State = types.StringValue(string(*purchase.Attributes.State))
	} else {
		model.State = types.StringNull()
	}

	if purchase.Attributes.ContentHosting != nil {
		model.ContentHosting = types.BoolValue(*purchase.Attributes.ContentHosting)
	} else {
		model.ContentHosting = types.BoolNull()
	}
}
