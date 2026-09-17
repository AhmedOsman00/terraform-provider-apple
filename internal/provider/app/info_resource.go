// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

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
	_ resource.Resource                = &appInfoResource{}
	_ resource.ResourceWithConfigure   = &appInfoResource{}
	_ resource.ResourceWithImportState = &appInfoResource{}
)

func NewAppInfoResource() resource.Resource {
	return &appInfoResource{}
}

type appInfoResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *appInfoResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_info"
}

// Schema defines the schema for the resource.
func (r *appInfoResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an app's App Store categories.\n\n" +
			"Categories hang off the app's *AppInfo* record — the half of the metadata that describes " +
			"the app rather than a release, alongside the localized name and the age rating. A category " +
			"is named by Apple's own constant (`FINANCE`, `PRODUCTIVITY`, `GAMES_PUZZLE`); list the " +
			"valid ones with the `apple_app_categories` data source.\n\n" +
			"~> **This resource adopts a record Apple already created; it does not create or destroy " +
			"one.** Apple makes an AppInfo with the app and publishes no `POST` and no `DELETE` for it. " +
			"`terraform destroy` drops it from state and warns, leaving the categories as last set.\n\n" +
			"~> **Writes only land while a version is being prepared.** Apple freezes an AppInfo once it " +
			"is in review or distributed, and offers no way to make a new one — so a category change " +
			"has to wait for the current review to finish. The provider selects the editable record " +
			"itself and says so plainly when there is none.\n\n" +
			"A category removed from the configuration is cleared at Apple rather than left in place: " +
			"the request sends an explicit null for every unset member.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the AppInfo record these categories are set on. Apple " +
					"issues a new one for each version cycle, so this changes over the app's life.",
				Computed: true,
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app. Read it from the `apple_apps` data source. " +
					"Cannot be changed.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"primary_category": schema.StringAttribute{
				MarkdownDescription: "The app's primary App Store category, for example `FINANCE`. This " +
					"is the one that decides where the app is ranked and browsed.",
				Optional:   true,
				Validators: []validator.String{CategoryValidator},
			},
			"primary_subcategory_one": schema.StringAttribute{
				MarkdownDescription: "First subcategory of the primary category. Only some categories " +
					"have subcategories — `GAMES` and `STICKERS` do, most do not — and Apple rejects a " +
					"subcategory that does not belong to the category it is paired with.",
				Optional:   true,
				Validators: []validator.String{CategoryValidator},
			},
			"primary_subcategory_two": schema.StringAttribute{
				MarkdownDescription: "Second subcategory of the primary category.",
				Optional:            true,
				Validators:          []validator.String{CategoryValidator},
			},
			"secondary_category": schema.StringAttribute{
				MarkdownDescription: "The app's secondary App Store category, for example `PRODUCTIVITY`. " +
					"Optional — an app may list in one category only.",
				Optional:   true,
				Validators: []validator.String{CategoryValidator},
			},
			"secondary_subcategory_one": schema.StringAttribute{
				MarkdownDescription: "First subcategory of the secondary category.",
				Optional:            true,
				Validators:          []validator.String{CategoryValidator},
			},
			"secondary_subcategory_two": schema.StringAttribute{
				MarkdownDescription: "Second subcategory of the secondary category.",
				Optional:            true,
				Validators:          []validator.String{CategoryValidator},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "The review state of the AppInfo record Apple selected, for example " +
					"`PREPARE_FOR_SUBMISSION`. Apple's, never the provider's.",
				Computed: true,
			},
			"app_store_age_rating": schema.StringAttribute{
				MarkdownDescription: "The age rating Apple calculated from the answers on " +
					"`apple_app_age_rating_declaration`, for example `FOUR_PLUS`. Computed by Apple.",
				Computed: true,
			},
		},
	}
}

// Create adopts the editable AppInfo and sets the configured categories.
func (r *appInfoResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating app info resource")

	var plan appInfoModel
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
// The editable record is resolved again rather than read by the stored ID:
// Apple issues a new AppInfo each version cycle, so the ID in state goes stale
// on its own, and following it would report the categories of a frozen record.
func (r *appInfoResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading app info resource")

	var state appInfoModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := state.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	info, err := r.client.GetEditableAppInfo(appID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "App info not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		// A frozen record is not a missing one: the categories still exist and
		// the resource should not vanish from state because a review is open.
		if strings.Contains(err.Error(), "no editable app info") {
			tflog.Warn(ctx, "No editable app info; keeping state as-is")
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading App Info",
			fmt.Sprintf("Could not read the app info of app '%s': %s", appID, err.Error()),
		)
		return
	}

	applyAppInfo(&state, info)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update patches the categories in place.
func (r *appInfoResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating app info resource")

	var plan appInfoModel
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

// Delete drops the categories from state.
//
// Apple publishes no DELETE for an AppInfo, and clearing the categories instead
// would be worse than leaving them: an app with no primary category cannot be
// submitted.
func (r *appInfoResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appInfoModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"App Info Not Deleted",
		fmt.Sprintf("Apple publishes no DELETE endpoint for an app info record, and clearing the "+
			"categories of app '%s' instead would leave it unsubmittable — a primary category is "+
			"required. Terraform has removed the resource from state only; the categories remain as "+
			"last set.", state.AppID.ValueString()),
	)

	tflog.Info(ctx, "App info removed from state", map[string]interface{}{
		"app_id": state.AppID.ValueString(),
	})
}

// Configure adds the provider configured client to the resource.
func (r *appInfoResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an app's categories.
//
// The import ID is the app ID, not the AppInfo ID: an app carries several
// AppInfo records and only one of them is the editable one this resource
// manages, so the app is the only stable way to name it.
func (r *appInfoResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing app info resource", map[string]interface{}{"import_id": req.ID})

	info, err := r.client.GetEditableAppInfo(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing App Info",
			fmt.Sprintf("Could not read the app info of app '%s': %s\n\n"+
				"The import ID is the app's Apple ID, not an app info ID — an app carries several app "+
				"info records and only the editable one can be managed.", req.ID, err.Error()),
		)
		return
	}

	state := appInfoModel{AppID: types.StringValue(req.ID)}
	applyAppInfo(&state, info)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write resolves the editable AppInfo and patches its categories.
func (r *appInfoResource) write(ctx context.Context, plan *appInfoModel, diags *diag.Diagnostics) {
	appID := plan.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	info, err := r.client.GetEditableAppInfo(appID)
	if err != nil {
		diags.AddError(
			"No Editable App Info",
			fmt.Sprintf("Could not resolve an app info record to write for app '%s': %s", appID, err.Error()),
		)

		return
	}

	ctx = tflog.SetField(ctx, "app_info_id", info.ID)
	tflog.Debug(ctx, "Writing app info categories")

	written, err := r.client.UpdateAppInfoCategories(info.ID, models.AppInfoUpdateRelationships{
		PrimaryCategory:         categoryLinkage(plan.PrimaryCategory),
		PrimarySubcategoryOne:   categoryLinkage(plan.PrimarySubcategoryOne),
		PrimarySubcategoryTwo:   categoryLinkage(plan.PrimarySubcategoryTwo),
		SecondaryCategory:       categoryLinkage(plan.SecondaryCategory),
		SecondarySubcategoryOne: categoryLinkage(plan.SecondarySubcategoryOne),
		SecondarySubcategoryTwo: categoryLinkage(plan.SecondarySubcategoryTwo),
	}, nil)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			diags.AddError(
				"Category Not Found",
				fmt.Sprintf("Could not set the categories of app '%s': %s\n\n"+
					"A category is named by Apple's own constant, such as FINANCE or GAMES_PUZZLE. List "+
					"the valid values with the apple_app_categories data source, and check that any "+
					"subcategory belongs to the category it is paired with.", appID, errMsg),
			)
		case strings.Contains(errMsg, "STATE_ERROR") || strings.Contains(errMsg, "not allowed"):
			diags.AddError(
				"App Info Not Editable",
				fmt.Sprintf("Apple refused the category change on app '%s': %s\n\n"+
					"An app info in review or already distributed is frozen, and Apple publishes no way "+
					"to create a new one. Wait for the current review to finish.", appID, errMsg),
			)
		default:
			diags.AddError(
				"Error Writing App Info",
				fmt.Sprintf("Could not set the categories of app '%s': %s", appID, errMsg),
			)
		}
		tflog.Error(ctx, "Failed to write app info categories", map[string]interface{}{"error": errMsg})

		return
	}

	applyAppInfoAttributes(plan, written)

	// The PATCH response carries the six category relationships as links alone:
	// Apple fills in a linkage only for an include, and the modify endpoint takes
	// none. Reading the categories off that response would put null in state for
	// every category just written, which Terraform reports as the provider
	// producing an inconsistent result after apply.
	refreshed, err := r.client.GetAppInfo(written.ID)
	if err != nil {
		diags.AddWarning(
			"App Info Categories Not Read Back",
			fmt.Sprintf("The categories of app '%s' were set, but the app info could not be read back: %s\n\n"+
				"State keeps the configured categories; the next plan refreshes them from Apple.", appID, err.Error()),
		)
		tflog.Warn(ctx, "Failed to read app info back after write", map[string]interface{}{"error": err.Error()})

		return
	}

	applyAppInfoCategories(plan, refreshed)
}

// applyAppInfo copies Apple's view of the record into the model.
//
// Only a response that was asked for the category linkages may go through here:
// the two halves are split because a PATCH response has none, and reading its
// empty relationships would null out the categories it had just set.
func applyAppInfo(model *appInfoModel, info *models.AppInfo) {
	applyAppInfoAttributes(model, info)
	applyAppInfoCategories(model, info)
}

// applyAppInfoAttributes copies the members Apple reports on every response.
func applyAppInfoAttributes(model *appInfoModel, info *models.AppInfo) {
	model.ID = types.StringValue(info.ID)

	model.State = types.StringNull()
	if info.Attributes.State != nil {
		model.State = types.StringValue(string(*info.Attributes.State))
	}
	model.AppStoreAgeRating = stringOrNull(info.Attributes.AppStoreAgeRating)
}

// applyAppInfoCategories copies the six category linkages into the model.
//
// They are adopted wholesale, including their absence: a category cleared at
// Apple has to show up as null in state, or the next plan would see no drift.
func applyAppInfoCategories(model *appInfoModel, info *models.AppInfo) {
	model.PrimaryCategory = types.StringNull()
	model.PrimarySubcategoryOne = types.StringNull()
	model.PrimarySubcategoryTwo = types.StringNull()
	model.SecondaryCategory = types.StringNull()
	model.SecondarySubcategoryOne = types.StringNull()
	model.SecondarySubcategoryTwo = types.StringNull()

	if info.Relationships == nil {
		return
	}

	model.PrimaryCategory = relationshipID(info.Relationships.PrimaryCategory)
	model.PrimarySubcategoryOne = relationshipID(info.Relationships.PrimarySubcategoryOne)
	model.PrimarySubcategoryTwo = relationshipID(info.Relationships.PrimarySubcategoryTwo)
	model.SecondaryCategory = relationshipID(info.Relationships.SecondaryCategory)
	model.SecondarySubcategoryOne = relationshipID(info.Relationships.SecondarySubcategoryOne)
	model.SecondarySubcategoryTwo = relationshipID(info.Relationships.SecondarySubcategoryTwo)
}
