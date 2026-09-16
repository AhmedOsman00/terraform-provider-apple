// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

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
	_ resource.Resource                = &appPriceScheduleResource{}
	_ resource.ResourceWithConfigure   = &appPriceScheduleResource{}
	_ resource.ResourceWithImportState = &appPriceScheduleResource{}
	_ resource.ResourceWithModifyPlan  = &appPriceScheduleResource{}
)

func NewAppPriceScheduleResource() resource.Resource {
	return &appPriceScheduleResource{}
}

type appPriceScheduleResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *appPriceScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_price_schedule"
}

// Schema defines the schema for the resource.
func (r *appPriceScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages what an app costs.\n\n" +
			"This is App Store Connect's **Pricing** — the section that blocks a submission with \"You " +
			"must choose a price tier in Pricing\". Apple has since retired price tiers: a price is now " +
			"a base territory whose price is equalized into every other storefront, plus the manual " +
			"prices that override the equalized ones.\n\n" +
			"A price is never written as a number. Apple publishes a catalogue of price points, each " +
			"fixing a customer price and the developer proceeds for one territory, and a price " +
			"references one of them — find the one you want with the `apple_app_price_points` data " +
			"source. **A free app is a price point of `0`**, not the absence of a schedule.\n\n" +
			"~> **This resource cannot be destroyed.** Apple publishes no `DELETE` for a price " +
			"schedule: an app that has been priced stays priced. `terraform destroy` drops it from " +
			"state and warns. Updates replace the whole schedule, which is what Apple's `POST` does.\n\n" +
			"Prices already in state are checked for existence rather than refreshed: Apple derives an " +
			"end date for any price a later one supersedes, and adopting a derived date would show as a " +
			"permanent diff against a configuration that never set one.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the price schedule. " +
					"Apple issues a new one every time the schedule is replaced.",
				Computed: true,
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app this schedule prices. Cannot be changed.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"base_territory": schema.StringAttribute{
				MarkdownDescription: "The three-letter Apple territory code whose price Apple equalizes " +
					"into every other storefront, for example `USA`, `GBR` or `EGY`. Required, and " +
					"normally the territory of the price point you actually care about.",
				Required:   true,
				Validators: []validator.String{TerritoryValidator},
			},
			"prices": schema.SetNestedAttribute{
				MarkdownDescription: "The prices set explicitly, as opposed to the ones Apple equalizes " +
					"from the base territory. A set, because Apple returns them in no particular order. " +
					"At least one is required — the base territory's own price.",
				Required: true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"price_point_id": schema.StringAttribute{
							MarkdownDescription: "The ID of the Apple price point this price references. " +
								"Obtain it from the `apple_app_price_points` data source — the ID " +
								"encodes the app, the territory and the price together, so a price " +
								"point read for one app cannot be used on another, and neither a " +
								"subscription's nor an in-app purchase's price points work here. The " +
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
func (r *appPriceScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating app price schedule resource")

	var plan appPriceScheduleModel
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
func (r *appPriceScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading app price schedule resource")

	var state appPriceScheduleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := state.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	schedule, prices, err := r.client.GetAppPrices(appID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "App price schedule not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading App Price Schedule",
			fmt.Sprintf("Could not read the price schedule of app '%s': %s", appID, err.Error()),
		)
		return
	}

	state.ID = types.StringValue(schedule.ID)

	if schedule.Relationships != nil {
		if territory := relationshipID(schedule.Relationships.BaseTerritory); !territory.IsNull() {
			state.BaseTerritory = territory
		}
	}

	state.Prices = reconcileAppPrices(state.Prices, prices)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update replaces the schedule.
//
// Apple publishes no PATCH: posting a schedule for an app that already has one
// supersedes it wholesale, which is exactly the semantics Terraform wants for an
// in-place update.
func (r *appPriceScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating app price schedule resource")

	var plan appPriceScheduleModel
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
// Apple publishes no DELETE for a price schedule, and an app that has been
// priced cannot be returned to having no price.
func (r *appPriceScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appPriceScheduleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"App Price Schedule Not Deleted",
		fmt.Sprintf("Apple publishes no DELETE endpoint for a price schedule, so the prices on app '%s' "+
			"remain in App Store Connect. Terraform has removed the resource from state only.\n\nTo "+
			"stop selling the app, narrow its apple_app_availability or remove it from sale in App "+
			"Store Connect.", state.AppID.ValueString()),
	)

	tflog.Info(ctx, "App price schedule removed from state", map[string]interface{}{
		"app_id": state.AppID.ValueString(),
	})
}

// ModifyPlan marks the ID unknown whenever anything else changes.
//
// Apple replaces the record rather than patching it, and issues a new ID each
// time. Terraform proposes the prior value for a computed attribute, so without
// this the plan would predict the old ID, apply would return a new one, and the
// framework would reject the result as inconsistent.
func (r *appPriceScheduleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
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
func (r *appPriceScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an app's price schedule.
//
// The import ID is the app ID, not the schedule ID: an app has exactly one
// schedule, and Apple offers no way to find one without knowing the app.
func (r *appPriceScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing app price schedule resource", map[string]interface{}{"import_id": req.ID})

	schedule, prices, err := r.client.GetAppPrices(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing App Price Schedule",
			fmt.Sprintf("Could not read the price schedule of app '%s': %s\n\n"+
				"The import ID is the app's Apple ID — an app has exactly one price schedule, and "+
				"Apple publishes no collection of schedules to look one up in.", req.ID, err.Error()),
		)
		return
	}

	state := appPriceScheduleModel{
		ID:     types.StringValue(schedule.ID),
		AppID:  types.StringValue(req.ID),
		Prices: reconcileAppPrices(nil, prices),
	}

	if schedule.Relationships != nil {
		state.BaseTerritory = relationshipID(schedule.Relationships.BaseTerritory)
	}
	if state.BaseTerritory.IsNull() {
		resp.Diagnostics.AddError(
			"Incomplete Price Schedule Import",
			fmt.Sprintf("Apple did not report a base territory for the price schedule of app '%s', so "+
				"base_territory cannot be written. Apple requires one on every schedule it accepts, so "+
				"this is unexpected — please report it.", req.ID),
		)
		return
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write posts the schedule and records the ID Apple issued.
func (r *appPriceScheduleResource) write(ctx context.Context, plan *appPriceScheduleModel, diags *diag.Diagnostics) {
	appID := plan.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)
	ctx = tflog.SetField(ctx, "base_territory", plan.BaseTerritory.ValueString())

	prices := make([]apple.AppManualPrice, 0, len(plan.Prices))
	for _, price := range plan.Prices {
		prices = append(prices, apple.AppManualPrice{
			PricePointID: price.PricePointID.ValueString(),
			StartDate:    stringOrNil(price.StartDate),
			EndDate:      stringOrNil(price.EndDate),
		})
	}

	tflog.Debug(ctx, "Writing app price schedule", map[string]interface{}{"price_count": len(prices)})

	schedule, err := r.client.CreateAppPriceSchedule(appID, plan.BaseTerritory.ValueString(), prices, nil)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			diags.AddError(
				"App or Price Point Not Found",
				fmt.Sprintf("Could not write the price schedule: %s\n\n"+
					"A price point ID is only valid for the app it was read from — it encodes the app, "+
					"the territory and the price together. Read it from the apple_app_price_points "+
					"data source scoped to app '%s'.", errMsg, appID),
			)
		case strings.Contains(errMsg, "baseTerritory"):
			diags.AddError(
				"Base Territory Rejected",
				fmt.Sprintf("Apple refused the base territory on app '%s': %s\n\n"+
					"The base territory must be one the app is actually available in — check "+
					"apple_app_availability.", appID, errMsg),
			)
		default:
			diags.AddError(
				"Error Writing App Price Schedule",
				fmt.Sprintf("Could not write the price schedule of app '%s': %s", appID, errMsg),
			)
		}
		tflog.Error(ctx, "Failed to write app price schedule", map[string]interface{}{"error": errMsg})

		return
	}

	plan.ID = types.StringValue(schedule.ID)
}

// reconcileAppPrices matches Apple's manual prices against what state holds.
//
// A price state already knows about keeps its configured dates: Apple derives
// an end date for any price a later one supersedes, and adopting that would
// show as a permanent diff against a configuration that never set one. A price
// Apple reports that state does not know about is added with Apple's own dates,
// so a change made in App Store Connect surfaces as drift rather than silently
// disappearing.
func reconcileAppPrices(state []appPriceModel, reported []models.AppPrice) []appPriceModel {
	byPricePoint := make(map[string]models.AppPrice, len(reported))
	for _, price := range reported {
		if price.Relationships == nil || price.Relationships.AppPricePoint == nil {
			continue
		}
		if pointID := price.Relationships.AppPricePoint.Data.ID; pointID != "" {
			byPricePoint[pointID] = price
		}
	}

	reconciled := make([]appPriceModel, 0, len(byPricePoint))
	for _, price := range state {
		pointID := price.PricePointID.ValueString()
		if _, ok := byPricePoint[pointID]; !ok {
			continue
		}
		reconciled = append(reconciled, price)
		delete(byPricePoint, pointID)
	}

	for pointID, price := range byPricePoint {
		reconciled = append(reconciled, appPriceModel{
			PricePointID: types.StringValue(pointID),
			StartDate:    stringOrNull(price.Attributes.StartDate),
			EndDate:      stringOrNull(price.Attributes.EndDate),
		})
	}

	return reconciled
}
