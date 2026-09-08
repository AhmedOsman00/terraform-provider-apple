// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &subscriptionAvailabilityResource{}
	_ resource.ResourceWithConfigure   = &subscriptionAvailabilityResource{}
	_ resource.ResourceWithImportState = &subscriptionAvailabilityResource{}
	_ resource.ResourceWithModifyPlan  = &subscriptionAvailabilityResource{}
)

func NewSubscriptionAvailabilityResource() resource.Resource {
	return &subscriptionAvailabilityResource{}
}

type subscriptionAvailabilityResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *subscriptionAvailabilityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_availability"
}

// Schema defines the schema for the resource.
func (r *subscriptionAvailabilityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the territories an auto-renewable subscription is sold in.\n\n" +
			"A subscription has exactly one availability record, and it is a **prerequisite for pricing**: " +
			"until one exists, Apple rejects `apple_subscription_price` with a 409 that mentions only " +
			"\"an error occurred while processing the pricing information\" and never names the missing " +
			"availability. Create this resource before any `apple_subscription_price`, or let Terraform " +
			"order it for you by referencing the subscription from both.\n\n" +
			"~> **This resource cannot be destroyed.** Apple publishes neither `PATCH` nor `DELETE` for " +
			"an availability: a change is a `POST` that supersedes the previous record, and there is no " +
			"way to return a subscription to having no availability at all. `terraform destroy` drops it " +
			"from state and warns; the territories remain as last set. To stop selling in a territory, " +
			"remove it from `available_territories` — that is a normal in-place update.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the availability record. " +
					"Apple issues a new one every time availability is replaced.",
				Computed: true,
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_subscription` whose availability this is. " +
					"Cannot be changed after creation.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"available_in_new_territories": schema.BoolAttribute{
				MarkdownDescription: "Whether the subscription is offered automatically in territories the " +
					"App Store adds in future. Defaults to `true`, matching App Store Connect's own " +
					"default. Set it to `false` to keep the territory list exactly as configured.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"available_territories": schema.SetAttribute{
				MarkdownDescription: "The three-letter Apple territory codes the subscription is sold in, " +
					"for example `[\"USA\", \"GBR\", \"EGY\"]`. A set, because Apple returns them in no " +
					"particular order. At least one is required, and a territory must be listed here " +
					"before an `apple_subscription_price` can name a price point in it.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(TerritoryValidator),
				},
			},
		},
	}
}

// Create a new resource.
func (r *subscriptionAvailabilityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating subscription availability resource")

	var plan subscriptionAvailabilityModel
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
func (r *subscriptionAvailabilityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading subscription availability resource")

	var state subscriptionAvailabilityModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()
	ctx = tflog.SetField(ctx, "subscription_id", subscriptionID)

	availability, territories, err := r.read(subscriptionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription availability not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Subscription Availability",
			fmt.Sprintf("Could not read the availability of subscription '%s': %s", subscriptionID, err.Error()),
		)
		return
	}

	state.ID = types.StringValue(availability.ID)
	state.AvailableTerritories = territories
	if availability.Attributes.AvailableInNewTerritories != nil {
		state.AvailableInNewTerritories = types.BoolValue(*availability.Attributes.AvailableInNewTerritories)
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update replaces the availability record.
//
// Apple publishes no PATCH: posting a new availability for the same
// subscription supersedes the old one, which is the semantics Terraform wants
// for an in-place update.
func (r *subscriptionAvailabilityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating subscription availability resource")

	var plan subscriptionAvailabilityModel
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

// Delete drops the availability from state.
//
// Apple publishes no DELETE, and a subscription cannot be returned to having no
// availability record. The territories stay as last set.
func (r *subscriptionAvailabilityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state subscriptionAvailabilityModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Subscription Availability Not Deleted",
		fmt.Sprintf("Apple publishes no DELETE endpoint for a subscription availability, so the "+
			"territories set on subscription '%s' remain as they are. Terraform has removed the "+
			"resource from state only.\n\nTo stop selling in a territory, remove it from "+
			"available_territories instead of destroying this resource.", state.SubscriptionID.ValueString()),
	)

	tflog.Info(ctx, "Subscription availability removed from state", map[string]interface{}{
		"subscription_id": state.SubscriptionID.ValueString(),
	})
}

// ModifyPlan marks the ID unknown whenever anything else changes.
//
// Apple replaces the record rather than patching it, and issues a new ID each
// time. Terraform proposes the prior value for a computed attribute, so without
// this the plan would predict the old ID, apply would return a new one, and the
// framework would reject the result as inconsistent.
func (r *subscriptionAvailabilityResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
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
func (r *subscriptionAvailabilityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports the availability of a subscription.
//
// The import ID is the subscription ID, not the availability ID: a subscription
// has exactly one availability record, and Apple offers no way to find one
// without knowing the subscription it belongs to.
func (r *subscriptionAvailabilityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing subscription availability resource", map[string]interface{}{"import_id": req.ID})

	availability, territories, err := r.read(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Subscription Availability",
			fmt.Sprintf("Could not read the availability of subscription '%s': %s\n\n"+
				"The import ID is the subscription ID — a subscription has exactly one availability "+
				"record, and Apple publishes no collection of them to look one up in.", req.ID, err.Error()),
		)
		return
	}

	state := subscriptionAvailabilityModel{
		ID:                        types.StringValue(availability.ID),
		SubscriptionID:            types.StringValue(req.ID),
		AvailableTerritories:      territories,
		AvailableInNewTerritories: types.BoolValue(false),
	}
	if availability.Attributes.AvailableInNewTerritories != nil {
		state.AvailableInNewTerritories = types.BoolValue(*availability.Attributes.AvailableInNewTerritories)
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// read fetches the availability record and the territory list behind it.
//
// Two requests, because Apple keeps them apart: the record carries only
// availableInNewTerritories, and the territories are a paginated collection
// hanging off it.
func (r *subscriptionAvailabilityResource) read(subscriptionID string) (*models.SubscriptionAvailability, []types.String, error) {
	availability, err := r.client.GetSubscriptionAvailability(subscriptionID)
	if err != nil {
		return nil, nil, err
	}

	territories, err := r.client.GetSubscriptionAvailableTerritories(availability.ID)
	if err != nil {
		return nil, nil, err
	}

	codes := make([]types.String, 0, len(territories))
	for _, territory := range territories {
		codes = append(codes, types.StringValue(territory.ID))
	}

	return availability, codes, nil
}

// write posts the availability and records the ID Apple issued.
func (r *subscriptionAvailabilityResource) write(ctx context.Context, plan *subscriptionAvailabilityModel, diags *diag.Diagnostics) {
	subscriptionID := plan.SubscriptionID.ValueString()
	ctx = tflog.SetField(ctx, "subscription_id", subscriptionID)

	territories := make([]string, 0, len(plan.AvailableTerritories))
	for _, territory := range plan.AvailableTerritories {
		if territory.IsNull() || territory.IsUnknown() {
			continue
		}
		territories = append(territories, territory.ValueString())
	}

	tflog.Debug(ctx, "Writing subscription availability", map[string]interface{}{
		"territory_count": len(territories),
	})

	availability, err := r.client.CreateSubscriptionAvailability(
		subscriptionID,
		plan.AvailableInNewTerritories.ValueBool(),
		territories,
		nil,
	)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			diags.AddError(
				"Subscription or Territory Not Found",
				fmt.Sprintf("Could not write the availability of subscription '%s': %s\n\n"+
					"Territories are named by three-letter Apple code — USA, GBR, EGY — not by the "+
					"two-letter ISO 3166-1 alpha-2 code.", subscriptionID, err.Error()),
			)
		default:
			diags.AddError(
				"Error Writing Subscription Availability",
				fmt.Sprintf("Could not write the availability of subscription '%s': %s",
					subscriptionID, err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to write subscription availability", map[string]interface{}{"error": err.Error()})
		return
	}

	plan.ID = types.StringValue(availability.ID)
}
