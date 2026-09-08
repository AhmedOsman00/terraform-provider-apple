// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package inapppurchase

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &inAppPurchasePriceScheduleResource{}
	_ resource.ResourceWithConfigure   = &inAppPurchasePriceScheduleResource{}
	_ resource.ResourceWithImportState = &inAppPurchasePriceScheduleResource{}
	_ resource.ResourceWithModifyPlan  = &inAppPurchasePriceScheduleResource{}
)

func NewInAppPurchasePriceScheduleResource() resource.Resource {
	return &inAppPurchasePriceScheduleResource{}
}

type inAppPurchasePriceScheduleResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *inAppPurchasePriceScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_in_app_purchase_price_schedule"
}

// Schema defines the schema for the resource.
func (r *inAppPurchasePriceScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the price of a one-time in-app purchase.\n\n" +
			"An in-app purchase has exactly one price schedule, not a price per territory the way an " +
			"`apple_subscription` does. You set the price in a base territory and Apple equalizes it " +
			"into every other storefront; each entry in `prices` then overrides the equalized price for " +
			"the territory its price point belongs to. A purchase stays in `MISSING_METADATA` until it " +
			"has a price.\n\n" +
			"A price is never expressed as a number. Apple publishes a catalogue of price points, each " +
			"fixing a customer price and the developer proceeds for one territory, and a price " +
			"references one of them — look one up with the `apple_in_app_purchase_price_points` data " +
			"source. Price point IDs are scoped to the purchase they were read from.\n\n" +
			"~> **This resource cannot be destroyed.** Apple publishes no `DELETE` for a price schedule: " +
			"a purchase that has been priced stays priced. `terraform destroy` drops it from state and " +
			"warns; the prices remain in App Store Connect. Updates work by replacing the whole " +
			"schedule, which is what Apple's `POST` does.\n\n" +
			"Prices already in state are not refreshed from Apple, only checked for existence: Apple " +
			"derives an end date for any price a later one supersedes, and adopting a derived date " +
			"would show as a permanent diff against a configuration that never set one.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the price schedule. " +
					"Apple issues a new one every time the schedule is replaced.",
				Computed: true,
			},
			"in_app_purchase_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_in_app_purchase` this schedule prices. " +
					"Cannot be changed after creation.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"base_territory": schema.StringAttribute{
				MarkdownDescription: "The three-letter Apple territory code whose price Apple equalizes " +
					"into every other storefront, for example `USA`, `GBR` or `EGY`. Required — Apple " +
					"rejects a schedule without one — and normally the territory of the price point you " +
					"actually care about.",
				Required:   true,
				Validators: []validator.String{TerritoryValidator},
			},
			"prices": schema.SetNestedAttribute{
				MarkdownDescription: "The prices set explicitly, as opposed to the ones Apple equalizes " +
					"from the base territory. A set, because Apple returns them in no particular order " +
					"and their order carries no meaning. At least one is required.",
				Required: true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"price_point_id": schema.StringAttribute{
							MarkdownDescription: "The ID of the Apple price point this price references. " +
								"Obtain it from the `apple_in_app_purchase_price_points` data source — " +
								"the ID encodes the purchase, the territory and the price tier together, " +
								"so a price point from one purchase cannot be used on another. The " +
								"territory is the price point's own; there is nothing to set separately.",
							Required: true,
						},
						"start_date": schema.StringAttribute{
							MarkdownDescription: "The date the price takes effect, as a plain date in " +
								"`YYYY-MM-DD` form — not a timestamp. Omit it for a price that takes " +
								"effect immediately.",
							Optional:   true,
							Validators: []validator.String{DateValidator},
						},
						"end_date": schema.StringAttribute{
							MarkdownDescription: "The date the price stops applying, as a plain date in " +
								"`YYYY-MM-DD` form. Omit it for a price that runs until the next " +
								"scheduled one takes over.",
							Optional:   true,
							Validators: []validator.String{DateValidator},
						},
					},
				},
			},
		},
	}
}

// Create a new resource.
func (r *inAppPurchasePriceScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating in-app purchase price schedule resource")

	var plan inAppPurchasePriceScheduleModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
//
// Apple's manual price collection is matched against state by price point,
// which is what detects a price deleted or replaced outside Terraform. The
// dates on a price already in state are left as configured -- see the schema
// description.
func (r *inAppPurchasePriceScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading in-app purchase price schedule resource")

	var state inAppPurchasePriceScheduleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	purchaseID := state.InAppPurchaseID.ValueString()
	ctx = tflog.SetField(ctx, "in_app_purchase_id", purchaseID)

	schedule, prices, err := r.client.GetInAppPurchasePrices(purchaseID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "In-app purchase price schedule not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading In-App Purchase Price Schedule",
			fmt.Sprintf("Could not read the price schedule of in-app purchase '%s': %s", purchaseID, err.Error()),
		)
		return
	}

	state.ID = types.StringValue(schedule.ID)

	if schedule.Relationships != nil {
		if territory := relationshipID(schedule.Relationships.BaseTerritory); !territory.IsNull() {
			state.BaseTerritory = territory
		}
	}

	state.Prices = reconcilePrices(state.Prices, prices)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update replaces the schedule.
//
// Apple publishes no PATCH: posting a schedule for a purchase that already has
// one supersedes it wholesale, which is exactly the semantics Terraform wants
// for an in-place update.
func (r *inAppPurchasePriceScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating in-app purchase price schedule resource")

	var plan inAppPurchasePriceScheduleModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete drops the schedule from state.
//
// Apple publishes no DELETE for a price schedule, and a purchase that has been
// priced cannot be returned to having no price. The prices stay in App Store
// Connect; only Terraform forgets about them.
func (r *inAppPurchasePriceScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state inAppPurchasePriceScheduleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"In-App Purchase Price Schedule Not Deleted",
		fmt.Sprintf("Apple publishes no DELETE endpoint for a price schedule, so the prices on in-app "+
			"purchase '%s' remain in App Store Connect. Terraform has removed the resource from state "+
			"only.\n\nTo stop selling the purchase, narrow its "+
			"apple_in_app_purchase_availability or remove it from sale in App Store Connect.",
			state.InAppPurchaseID.ValueString()),
	)

	tflog.Info(ctx, "In-app purchase price schedule removed from state", map[string]interface{}{
		"in_app_purchase_id": state.InAppPurchaseID.ValueString(),
	})
}

// ModifyPlan marks the ID unknown whenever anything else changes.
//
// Apple replaces the record rather than patching it, and issues a new ID each
// time. Terraform proposes the prior value for a computed attribute, so without
// this the plan would predict the old ID, apply would return a new one, and the
// framework would reject the result as inconsistent.
func (r *inAppPurchasePriceScheduleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Create and destroy need nothing: the ID is already unknown on one and
	// irrelevant on the other.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	if req.Plan.Raw.Equal(req.State.Raw) {
		return
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())...)
}

// Configure adds the provider configured client to the resource.
func (r *inAppPurchasePriceScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports the price schedule of an in-app purchase.
//
// The import ID is the in-app purchase ID, not the schedule ID: a purchase has
// exactly one schedule, and Apple offers no way to find a schedule without
// knowing the purchase it prices.
func (r *inAppPurchasePriceScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing in-app purchase price schedule resource", map[string]interface{}{"import_id": req.ID})

	schedule, prices, err := r.client.GetInAppPurchasePrices(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing In-App Purchase Price Schedule",
			fmt.Sprintf("Could not read the price schedule of in-app purchase '%s': %s\n\n"+
				"The import ID is the in-app purchase ID — a purchase has exactly one price schedule, "+
				"and Apple publishes no collection of schedules to look one up in.", req.ID, err.Error()),
		)
		return
	}

	state := inAppPurchasePriceScheduleModel{
		ID:              types.StringValue(schedule.ID),
		InAppPurchaseID: types.StringValue(req.ID),
		Prices:          reconcilePrices(nil, prices),
	}

	if schedule.Relationships != nil {
		state.BaseTerritory = relationshipID(schedule.Relationships.BaseTerritory)
	}
	if state.BaseTerritory.IsNull() {
		resp.Diagnostics.AddError(
			"Incomplete Price Schedule Import",
			fmt.Sprintf("Apple did not report a base territory for the price schedule of in-app "+
				"purchase '%s', so base_territory cannot be written. Apple requires one on every "+
				"schedule it accepts, so this is unexpected — please report it.", req.ID),
		)
		return
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write posts the schedule and records the ID Apple issued.
func (r *inAppPurchasePriceScheduleResource) write(ctx context.Context, plan *inAppPurchasePriceScheduleModel, diags *diag.Diagnostics) {
	purchaseID := plan.InAppPurchaseID.ValueString()
	ctx = tflog.SetField(ctx, "in_app_purchase_id", purchaseID)
	ctx = tflog.SetField(ctx, "base_territory", plan.BaseTerritory.ValueString())

	prices := make([]apple.InAppPurchaseManualPrice, 0, len(plan.Prices))
	for _, price := range plan.Prices {
		prices = append(prices, apple.InAppPurchaseManualPrice{
			PricePointID: price.PricePointID.ValueString(),
			StartDate:    stringOrNil(price.StartDate),
			EndDate:      stringOrNil(price.EndDate),
		})
	}

	tflog.Debug(ctx, "Writing in-app purchase price schedule", map[string]interface{}{"price_count": len(prices)})

	schedule, err := r.client.CreateInAppPurchasePriceSchedule(
		purchaseID,
		plan.BaseTerritory.ValueString(),
		prices,
		nil,
	)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			diags.AddError(
				"In-App Purchase or Price Point Not Found",
				fmt.Sprintf("Could not write the price schedule: %s\n\n"+
					"A price point ID is only valid for the in-app purchase it was read from — it "+
					"encodes the purchase, the territory and the tier together. Read it from the "+
					"apple_in_app_purchase_price_points data source scoped to purchase '%s'.",
					err.Error(), purchaseID),
			)
		default:
			diags.AddError(
				"Error Writing In-App Purchase Price Schedule",
				fmt.Sprintf("Could not write the price schedule of in-app purchase '%s': %s",
					purchaseID, err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to write in-app purchase price schedule", map[string]interface{}{"error": err.Error()})
		return
	}

	plan.ID = types.StringValue(schedule.ID)
}

// reconcilePrices matches Apple's manual prices against what state holds.
//
// A price state already knows about keeps its configured dates: Apple derives
// an end date for any price a later one supersedes, and adopting that would
// show as a permanent diff against a configuration that never set one. A price
// Apple reports that state does not know about is added with Apple's own dates,
// so a change made in App Store Connect surfaces as drift rather than silently
// disappearing.
func reconcilePrices(state []inAppPurchasePriceModel, reported []models.InAppPurchasePrice) []inAppPurchasePriceModel {
	byPricePoint := make(map[string]models.InAppPurchasePrice, len(reported))
	for _, price := range reported {
		if price.Relationships == nil || price.Relationships.InAppPurchasePricePoint == nil {
			continue
		}
		if pointID := price.Relationships.InAppPurchasePricePoint.Data.ID; pointID != "" {
			byPricePoint[pointID] = price
		}
	}

	reconciled := make([]inAppPurchasePriceModel, 0, len(byPricePoint))
	for _, price := range state {
		pointID := price.PricePointID.ValueString()
		if _, ok := byPricePoint[pointID]; !ok {
			continue
		}
		reconciled = append(reconciled, price)
		delete(byPricePoint, pointID)
	}

	for pointID, price := range byPricePoint {
		reconciled = append(reconciled, inAppPurchasePriceModel{
			PricePointID: types.StringValue(pointID),
			StartDate:    stringOrNull(price.Attributes.StartDate),
			EndDate:      stringOrNull(price.Attributes.EndDate),
		})
	}

	return reconciled
}
