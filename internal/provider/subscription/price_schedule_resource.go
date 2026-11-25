// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &subscriptionPriceScheduleResource{}
	_ resource.ResourceWithConfigure   = &subscriptionPriceScheduleResource{}
	_ resource.ResourceWithImportState = &subscriptionPriceScheduleResource{}
)

func NewSubscriptionPriceScheduleResource() resource.Resource {
	return &subscriptionPriceScheduleResource{}
}

type subscriptionPriceScheduleResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *subscriptionPriceScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_price_schedule"
}

// Schema defines the schema for the resource.
func (r *subscriptionPriceScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sets every price of an auto-renewable subscription in one request.\n\n" +
			"This is the bulk counterpart to `apple_subscription_price`. That resource maps onto " +
			"`POST /v1/subscriptionPrices`, which takes one price point in one territory, so pricing a " +
			"subscription in all 175 App Store storefronts through it means 175 resources and 175 API " +
			"calls on apply — and a refresh that lists the subscription's whole price collection once per " +
			"resource, because Apple publishes no `GET` for a single price. This resource sends the same " +
			"set as a single `PATCH /v1/subscriptions/{id}` carrying the prices inline, and reads them " +
			"back in one listing.\n\n" +
			"Pair it with `apple_subscription_price_point_equalizations`, which maps a base price point " +
			"to the equivalent point in every other territory: the two together are what App Store " +
			"Connect's own price matrix does.\n\n" +
			"A price is never expressed as a number. Apple publishes a catalogue of price points, each " +
			"fixing a customer price and the developer proceeds for one territory, and a price references " +
			"one of them. Price point IDs are scoped to the subscription they were read from.\n\n" +
			"~> **An `apple_subscription_availability` has to exist first.** Apple rejects pricing for a " +
			"subscription with no availability record, and answers with a message that mentions neither " +
			"availability nor a territory. Nothing in a price references one, so declare the ordering " +
			"with `depends_on`.\n\n" +
			"**How a write works depends on the subscription's state**, which is read first. Before " +
			"approval (`MISSING_METADATA`, `READY_TO_SUBMIT`, `DEVELOPER_ACTION_NEEDED`) the whole set " +
			"is sent as one `PATCH`, which Apple reads as the initial price. After approval Apple " +
			"refuses an initial price, and a price change is a new dated record: the resource sends " +
			"one `POST /v1/subscriptionPrices` for each territory whose price differs from the one " +
			"in effect, and deletes a scheduled change the configuration no longer names. A territory " +
			"the configuration leaves out keeps its price, because a live price cannot be withdrawn, " +
			"and the apply warns about it.\n\n" +
			"~> **Apple refuses to change prices while the subscription is in review** " +
			"(`WAITING_FOR_REVIEW`, `IN_REVIEW`, `PENDING_BINARY_APPROVAL`). Apply again once the " +
			"review has finished.\n\n" +
			"~> **This resource cannot be destroyed.** A subscription that has been priced stays priced, " +
			"and Apple deletes only price changes scheduled for the future — a live price is what the " +
			"subscription currently costs. `terraform destroy` drops the resource from state and warns; " +
			"the prices remain in App Store Connect. Deleting the subscription removes them with it.\n\n" +
			"Do not manage the same subscription with both this resource and `apple_subscription_price`: " +
			"each write here replaces the subscription's manual price set, so a price only the other " +
			"resource knows about would be removed behind its back.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the subscription these prices belong to. A subscription " +
					"has no separate price schedule record at Apple — unlike an in-app purchase — so " +
					"this mirrors `subscription_id`.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_subscription` these prices apply to. Cannot be " +
					"changed after creation.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"prices": schema.SetNestedAttribute{
				MarkdownDescription: "The complete set of prices the subscription should have. A set, " +
					"because Apple returns them in no particular order and their order carries no " +
					"meaning. At least one is required — a subscription stays in `MISSING_METADATA` " +
					"until it has a price.\n\n" +
					"Before approval every write replaces the whole set, so a price removed from the " +
					"configuration is removed at Apple. After approval a price already in effect cannot " +
					"be withdrawn, only superseded by a new one: a territory removed from the " +
					"configuration keeps its price.",
				Required: true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"price_point_id": schema.StringAttribute{
							MarkdownDescription: "The ID of the Apple price point this price references. " +
								"Obtain it from the `apple_subscription_price_points` data source, or " +
								"from `apple_subscription_price_point_equalizations` for the fan-out " +
								"across territories — the ID encodes the subscription, the territory and " +
								"the price tier together, so a price point from one subscription cannot " +
								"be used on another.",
							Required: true,
						},
						"territory_id": schema.StringAttribute{
							MarkdownDescription: "The three-letter Apple territory code this price applies " +
								"to, for example `USA`, `GBR` or `EGY`. Optional: a price point already " +
								"encodes its territory and Apple infers it. Setting it makes an " +
								"equalized fan-out self-describing, which is the case it exists for. " +
								"Changing the price of an approved subscription is posted per territory, " +
								"so a new price point there needs its territory_id unless Apple already " +
								"lists a price at it.",
							Optional:   true,
							Validators: []validator.String{TerritoryValidator},
						},
						"start_date": schema.StringAttribute{
							MarkdownDescription: "The date the price takes effect, as a plain date in " +
								"`YYYY-MM-DD` form — not a timestamp. Omit it for the soonest Apple " +
								"allows: immediately for the initial price of a subscription not yet " +
								"approved, and three days from today (UTC) for a change to an approved " +
								"one, since Apple refuses an undated price change and needs one to two " +
								"days' notice depending on the time zone.",
							Optional:   true,
							Validators: []validator.String{StartDateValidator},
						},
						"preserve_current_price": schema.BoolAttribute{
							MarkdownDescription: "Whether existing subscribers keep the price they signed " +
								"up at rather than moving to this one. Applies only when this price " +
								"replaces an earlier one, on an approved subscription. It is an " +
								"instruction sent with the change and Apple never reports it back, so " +
								"it is not refreshed.",
							Optional: true,
						},
						"plan_type": schema.StringAttribute{
							MarkdownDescription: "Which plan the price applies to. Leave it unset for " +
								"the ordinary price, which Apple reports as `UPFRONT`. `MONTHLY` is " +
								"Apple's monthly-payment plan on a longer commitment.",
							Optional:   true,
							Validators: []validator.String{PlanTypeValidator},
						},
					},
				},
			},
		},
	}
}

// Create a new resource.
func (r *subscriptionPriceScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating subscription price schedule resource")

	var plan subscriptionPriceScheduleModel
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
// One listing of the subscription's price collection covers every price, where
// a configuration built from apple_subscription_price resources pays for that
// listing once per territory. Prices are matched against state by price point;
// the dates and flags on a price already in state are left as configured -- see
// reconcileSchedulePrices.
func (r *subscriptionPriceScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading subscription price schedule resource")

	var state subscriptionPriceScheduleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()
	ctx = tflog.SetField(ctx, "subscription_id", subscriptionID)

	prices, err := r.client.GetSubscriptionPrices(subscriptionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription not found, removing price schedule from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Subscription Price Schedule",
			fmt.Sprintf("Could not read the prices of subscription '%s': %s", subscriptionID, err.Error()),
		)
		return
	}

	// A subscription with no prices at all is one that has been un-priced
	// outside Terraform, which is the only way this resource can cease to
	// exist: there is no record of its own to delete.
	if len(prices) == 0 {
		tflog.Info(ctx, "Subscription has no prices, removing price schedule from state")
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(subscriptionID)
	state.Prices = reconcileSchedulePrices(state.Prices, prices)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update replaces the price set.
//
// Apple's PATCH names the complete set of manual prices the subscription should
// have afterwards, which is exactly the semantics Terraform wants for an
// in-place update -- so this is a genuine update rather than a replacement.
func (r *subscriptionPriceScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating subscription price schedule resource")

	var plan subscriptionPriceScheduleModel
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
// There is no record to delete: the prices are the subscription's, and Apple
// deletes only price changes scheduled for the future. Warn and let Terraform
// forget them, the way apple_device does for a deletion Apple cannot perform
// either.
func (r *subscriptionPriceScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state subscriptionPriceScheduleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Subscription Prices Not Deleted",
		fmt.Sprintf("A subscription that has been priced stays priced: Apple deletes only price changes "+
			"scheduled for the future, and a price already in effect is what subscription '%s' currently "+
			"costs. Terraform has removed the resource from state only, and the prices still apply in "+
			"App Store Connect.\n\nTo stop selling the subscription, narrow its "+
			"apple_subscription_availability or remove it from sale in App Store Connect. Deleting the "+
			"subscription removes its prices along with it.",
			state.SubscriptionID.ValueString()),
	)

	tflog.Info(ctx, "Subscription price schedule removed from state", map[string]interface{}{
		"subscription_id": state.SubscriptionID.ValueString(),
	})
}

// Configure adds the provider configured client to the resource.
func (r *subscriptionPriceScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports the prices of a subscription.
//
// The import ID is the subscription ID: the prices are reachable only through
// the collection hanging off it, and this resource has no record of its own.
func (r *subscriptionPriceScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing subscription price schedule resource", map[string]interface{}{"import_id": req.ID})

	prices, err := r.client.GetSubscriptionPrices(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Subscription Price Schedule",
			fmt.Sprintf("Could not read the prices of subscription '%s': %s\n\n"+
				"The import ID is the subscription ID — a subscription has no separate price schedule "+
				"record, and its prices are only reachable through the collection hanging off it.",
				req.ID, err.Error()),
		)
		return
	}

	if len(prices) == 0 {
		resp.Diagnostics.AddError(
			"Subscription Has No Prices",
			fmt.Sprintf("Subscription '%s' has no prices to import, so there is nothing for this "+
				"resource to adopt. Apply the configuration instead of importing it.", req.ID),
		)
		return
	}

	state := subscriptionPriceScheduleModel{
		ID:             types.StringValue(req.ID),
		SubscriptionID: types.StringValue(req.ID),
		Prices:         reconcileSchedulePrices(nil, prices),
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// now is the clock a write dates price changes by; tests replace it.
var now = time.Now

// write sets the planned prices.
//
// How depends on the subscription's state, which is read first. Before
// approval (initialPriceStates) the whole set goes in one replace-all PATCH,
// which Apple reads as the initial price. After it, Apple refuses that PATCH
// with 409 "Initial price cannot be created again after subscription is
// approved": a price change is a new dated record per territory, so
// writePriceChanges posts one for each territory whose price differs.
func (r *subscriptionPriceScheduleResource) write(ctx context.Context, plan *subscriptionPriceScheduleModel, diags *diag.Diagnostics) {
	subscriptionID := plan.SubscriptionID.ValueString()
	ctx = tflog.SetField(ctx, "subscription_id", subscriptionID)

	subscription, err := r.client.GetSubscription(subscriptionID)
	if err != nil {
		diags.AddError(
			"Error Reading Subscription",
			fmt.Sprintf("Could not read subscription '%s' before writing its prices: %s\n\n"+
				"The subscription's state decides how a price is written, so it is read first.",
				subscriptionID, err.Error()),
		)
		return
	}

	var state models.SubscriptionState
	if subscription.Attributes.State != nil {
		state = *subscription.Attributes.State
	}
	ctx = tflog.SetField(ctx, "subscription_state", string(state))

	if state == "" || initialPriceStates[state] {
		r.writeInitialPrices(ctx, plan, state, diags)
	} else {
		r.writePriceChanges(ctx, plan, state, diags)
	}
	if diags.HasError() {
		return
	}

	plan.ID = types.StringValue(subscriptionID)
}

// writeInitialPrices sends the whole price set in one PATCH.
func (r *subscriptionPriceScheduleResource) writeInitialPrices(ctx context.Context, plan *subscriptionPriceScheduleModel, state models.SubscriptionState, diags *diag.Diagnostics) {
	subscriptionID := plan.SubscriptionID.ValueString()

	prices := make([]apple.SubscriptionManualPrice, 0, len(plan.Prices))
	for _, price := range plan.Prices {
		manual := apple.SubscriptionManualPrice{
			PricePointID:         price.PricePointID.ValueString(),
			TerritoryID:          price.TerritoryID.ValueString(),
			StartDate:            stringOrNil(price.StartDate),
			PreserveCurrentPrice: boolOrNil(price.PreserveCurrentPrice),
		}
		if planType := stringOrNil(price.PlanType); planType != nil {
			t := models.SubscriptionPlanType(*planType)
			manual.PlanType = &t
		}

		prices = append(prices, manual)
	}

	tflog.Debug(ctx, "Writing subscription price schedule", map[string]interface{}{"price_count": len(prices)})

	if _, err := r.client.SetSubscriptionPrices(subscriptionID, prices, nil); err != nil {
		addPriceWriteError(diags, subscriptionID, state, len(prices), err)
		tflog.Error(ctx, "Failed to write subscription price schedule", map[string]interface{}{"error": err.Error()})
	}
}

// writePriceChanges changes the prices of an approved subscription.
//
// planPriceChanges decides the requests: a scheduled change the plan no longer
// wants is deleted, and every territory whose price differs gets one dated
// POST /v1/subscriptionPrices. A territory the plan does not name is left
// alone, because a live price cannot be withdrawn, and reported as a warning.
func (r *subscriptionPriceScheduleResource) writePriceChanges(ctx context.Context, plan *subscriptionPriceScheduleModel, state models.SubscriptionState, diags *diag.Diagnostics) {
	subscriptionID := plan.SubscriptionID.ValueString()

	reported, err := r.client.GetSubscriptionPrices(subscriptionID)
	if err != nil {
		diags.AddError(
			"Error Reading Subscription Prices",
			fmt.Sprintf("Could not read the prices of subscription '%s' before changing them: %s",
				subscriptionID, err.Error()),
		)
		return
	}

	changes := planPriceChanges(plan.Prices, reported, now())
	if len(changes.NoTerritory) > 0 {
		diags.AddError(
			"Price Has No Territory",
			fmt.Sprintf("Subscription '%s' is %s, so a price change is posted per territory, and these "+
				"price points have no territory_id: %s.\n\nSet territory_id on each of them. The "+
				"price_point_ids map of apple_subscription_price_point_equalizations is keyed by "+
				"territory, which is the value to use.",
				subscriptionID, state, strings.Join(changes.NoTerritory, ", ")),
		)
		return
	}

	tflog.Debug(ctx, "Changing subscription prices", map[string]interface{}{
		"deletions": len(changes.Deletions),
		"creations": len(changes.Creations),
	})

	for _, priceID := range changes.Deletions {
		if err := r.client.DeleteSubscriptionPrice(priceID, nil); err != nil {
			diags.AddError(
				"Error Removing Scheduled Price Change",
				fmt.Sprintf("Could not delete scheduled price '%s' of subscription '%s', which the "+
					"configuration no longer names: %s", priceID, subscriptionID, err.Error()),
			)
			return
		}
	}

	for _, change := range changes.Creations {
		territory := change.TerritoryID
		attributes := &models.SubscriptionPriceCreateAttributes{
			StartDate:            change.StartDate,
			PreserveCurrentPrice: change.PreserveCurrentPrice,
			PlanType:             change.PlanType,
		}
		if _, err := r.client.CreateSubscriptionPrice(subscriptionID, change.PricePointID, &territory, attributes, nil); err != nil {
			addPriceWriteError(diags, subscriptionID, state, 1, fmt.Errorf("territory %s: %w", territory, err))
			tflog.Error(ctx, "Failed to change subscription price", map[string]interface{}{
				"territory": territory,
				"error":     err.Error(),
			})
			return
		}
	}

	if len(changes.Unmanaged) > 0 {
		diags.AddWarning(
			"Prices Left In Place",
			fmt.Sprintf("Subscription '%s' has prices in territories the configuration does not name: %s.\n\n"+
				"A live price cannot be withdrawn, so they were left as they are, and the next plan will "+
				"show them again. Add them to prices, or narrow the subscription's "+
				"apple_subscription_availability to stop selling there.",
				subscriptionID, strings.Join(changes.Unmanaged, ", ")),
		)
	}
}

// addPriceWriteError maps Apple's answer to a price write onto a diagnostic.
func addPriceWriteError(diags *diag.Diagnostics, subscriptionID string, state models.SubscriptionState, count int, err error) {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
		diags.AddError(
			"Subscription or Price Point Not Found",
			fmt.Sprintf("Could not write the prices of subscription '%s': %s\n\n"+
				"A price point ID is only valid for the subscription it was read from — it encodes "+
				"the subscription, the territory and the tier together. Read it from the "+
				"apple_subscription_price_points data source scoped to subscription '%s', or from "+
				"apple_subscription_price_point_equalizations on a price point of that subscription.",
				subscriptionID, errMsg, subscriptionID),
		)
	case strings.Contains(errMsg, "processing the pricing information"):
		// Apple answers a rejected price with "An error occurred while
		// processing the pricing information" and nothing else -- no mention
		// of which part of the request it objected to. The usual cause is
		// the one that message is least likely to suggest: the subscription
		// has no availability record, so there is no territory it can be
		// priced in. Apple requires availability before pricing and reports
		// its absence only here.
		diags.AddError(
			"Error Writing Subscription Price Schedule",
			fmt.Sprintf("Could not write %d price(s) for subscription '%s': %s\n\n"+
				"Apple rejects pricing with this message when the subscription has no availability "+
				"record, which it requires before any pricing. Add an apple_subscription_availability "+
				"for subscription '%s' listing every territory these prices cover, and make this "+
				"resource depend on it.\n\n"+
				"The same message also covers a price point that is not valid for this subscription, "+
				"and two prices for the same territory at the same start date.",
				count, subscriptionID, errMsg, subscriptionID),
		)
	case strings.Contains(errMsg, "Initial price cannot be created again"):
		diags.AddError(
			"Error Writing Subscription Price Schedule",
			fmt.Sprintf("Could not write %d price(s) for subscription '%s' (state %s): %s\n\n"+
				"Apple read the write as an initial price, which it accepts only before the subscription "+
				"is approved. The provider chooses between the two kinds of write from the state it "+
				"read, so this is a provider bug: please report it with the state above.",
				count, subscriptionID, state, errMsg),
		)
	case reviewPriceStates[state]:
		diags.AddError(
			"Error Writing Subscription Price Schedule",
			fmt.Sprintf("Could not write %d price(s) for subscription '%s': %s\n\n"+
				"The subscription is %s, and Apple refuses to change prices while it is in review. "+
				"Apply again once the review has finished.", count, subscriptionID, errMsg, state),
		)
	default:
		diags.AddError(
			"Error Writing Subscription Price Schedule",
			fmt.Sprintf("Could not write %d price(s) for subscription '%s' (state %s): %s",
				count, subscriptionID, state, errMsg),
		)
	}
}
