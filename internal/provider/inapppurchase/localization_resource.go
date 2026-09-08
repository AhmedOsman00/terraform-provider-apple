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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &inAppPurchaseLocalizationResource{}
	_ resource.ResourceWithConfigure   = &inAppPurchaseLocalizationResource{}
	_ resource.ResourceWithImportState = &inAppPurchaseLocalizationResource{}
)

func NewInAppPurchaseLocalizationResource() resource.Resource {
	return &inAppPurchaseLocalizationResource{}
}

type inAppPurchaseLocalizationResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *inAppPurchaseLocalizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_in_app_purchase_localization"
}

// Schema defines the schema for the resource.
func (r *inAppPurchaseLocalizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the customer-facing name and description of an in-app purchase in one locale.\n\n" +
			"This is what a customer actually reads on the App Store and in the system purchase sheet — " +
			"the `name` on `apple_in_app_purchase` is an internal reference name that customers never see. " +
			"A purchase stays in `MISSING_METADATA` until it has at least one localization.\n\n" +
			"The locale must be one the app itself supports; Apple rejects a localization for a locale " +
			"the app has not been localized into.\n\n" +
			"## Versions\n\n" +
			"Apple attaches localizations to an *in-app purchase version* — the draft that carries " +
			"metadata through App Review — rather than to the purchase itself. This resource resolves " +
			"the version for you: it writes into whichever draft still accepts edits, and creates one " +
			"when every existing version is in review or already approved, which is what App Store " +
			"Connect's own UI does when you edit approved metadata. The version it used is reported as " +
			"`version_id`. Versions cannot be deleted, so one created this way outlives the localization.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the localization.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"in_app_purchase_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_in_app_purchase` this localization describes. " +
					"Cannot be changed after creation.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"version_id": schema.StringAttribute{
				MarkdownDescription: "The in-app purchase version this localization was written into. " +
					"Resolved by the provider and reported for reference; it is not configurable.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"locale": schema.StringAttribute{
				MarkdownDescription: "The App Store locale this localization applies to, for example `en-US`, " +
					"`ar-SA` or `es-MX`. Apple permits only one localization per locale per version. " +
					"Cannot be changed after creation — the locale identifies the record, and Apple's " +
					"update request has no locale member.",
				Required:   true,
				Validators: []validator.String{LocaleValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The purchase name customers see in this locale. Limited to 30 characters. " +
					"Can be updated in place, though a change on an approved purchase sends it back for review.",
				Required:   true,
				Validators: GetNameValidator(),
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The purchase description customers see in this locale. Limited to " +
					"45 characters. Optional, but Apple's review expects one on anything customer-facing. " +
					"Can be updated in place.",
				Optional:   true,
				Validators: GetDescriptionValidator(),
			},
		},
	}
}

// Create a new resource.
func (r *inAppPurchaseLocalizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating in-app purchase localization resource")

	var plan inAppPurchaseLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	purchaseID := plan.InAppPurchaseID.ValueString()
	ctx = tflog.SetField(ctx, "in_app_purchase_id", purchaseID)
	ctx = tflog.SetField(ctx, "locale", plan.Locale.ValueString())

	version, err := r.client.EnsureEditableInAppPurchaseVersion(purchaseID, nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			resp.Diagnostics.AddError(
				"In-App Purchase Not Found",
				fmt.Sprintf("In-app purchase '%s' was not found.", purchaseID),
			)
			return
		}

		resp.Diagnostics.AddError(
			"Error Resolving In-App Purchase Version",
			fmt.Sprintf("Could not find or create a version of in-app purchase '%s' to localize: %s\n\n"+
				"Apple attaches localizations to a version rather than to the purchase, so one that "+
				"still accepts edits has to exist before a localization can be written.",
				purchaseID, err.Error()),
		)
		return
	}

	ctx = tflog.SetField(ctx, "version_id", version.ID)

	localization, err := r.client.CreateInAppPurchaseLocalization(
		version.ID,
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
				fmt.Sprintf("In-app purchase '%s' already has a localization for locale '%s'. Apple "+
					"permits only one per locale.", purchaseID, plan.Locale.ValueString()),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			resp.Diagnostics.AddError(
				"In-App Purchase Version Not Found",
				fmt.Sprintf("Version '%s' of in-app purchase '%s' was not found.", version.ID, purchaseID),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Creating In-App Purchase Localization",
				fmt.Sprintf("Could not create localization for locale '%s': %s\n\n"+
					"If Apple reports the locale as invalid, check that the app itself is localized "+
					"into it — an in-app purchase cannot be localized into a locale the app does not "+
					"support.", plan.Locale.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create in-app purchase localization", map[string]interface{}{"error": err.Error()})
		return
	}

	plan.ID = types.StringValue(localization.ID)
	plan.VersionID = types.StringValue(version.ID)
	applyLocalizationAttributes(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *inAppPurchaseLocalizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading in-app purchase localization resource")

	var state inAppPurchaseLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "localization_id", state.ID.ValueString())

	localization, err := r.client.GetInAppPurchaseLocalization(state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "In-app purchase localization not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading In-App Purchase Localization",
			fmt.Sprintf("Could not read localization '%s': %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	applyLocalizationAttributes(&state, localization)

	if localization.Relationships != nil {
		if versionID := relationshipID(localization.Relationships.Version); !versionID.IsNull() {
			state.VersionID = versionID
		}
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *inAppPurchaseLocalizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating in-app purchase localization resource")

	var plan inAppPurchaseLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "localization_id", plan.ID.ValueString())

	name := plan.Name.ValueString()
	localization, err := r.client.UpdateInAppPurchaseLocalization(
		plan.ID.ValueString(),
		&name,
		stringOrNil(plan.Description),
		nil,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating In-App Purchase Localization",
			fmt.Sprintf("Could not update localization '%s': %s\n\n"+
				"A localization can only be edited while the version that owns it still accepts "+
				"changes. One already in review has to be superseded by a new version instead.",
				plan.ID.ValueString(), err.Error()),
		)
		return
	}

	applyLocalizationAttributes(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *inAppPurchaseLocalizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting in-app purchase localization resource")

	var state inAppPurchaseLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "localization_id", state.ID.ValueString())

	err := r.client.DeleteInAppPurchaseLocalization(state.ID.ValueString(), nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "In-app purchase localization already deleted")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting In-App Purchase Localization",
			fmt.Sprintf("Could not delete localization '%s': %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "In-app purchase localization deleted successfully")
}

// Configure adds the provider configured client to the resource.
func (r *inAppPurchaseLocalizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
// A bare localization ID is accepted: Apple reports the version that owns it,
// and the version reports the purchase, so the parent can be resolved in two
// hops. The composite "<in_app_purchase_id>/<localization_id>" form is accepted
// too, and additionally confirms the localization belongs to the purchase named.
func (r *inAppPurchaseLocalizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing in-app purchase localization resource", map[string]interface{}{"import_id": req.ID})

	purchaseID, localizationID, composite := strings.Cut(req.ID, "/")
	if !composite {
		localizationID = req.ID
		purchaseID = ""
	}

	localization, err := r.client.GetInAppPurchaseLocalization(localizationID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing In-App Purchase Localization",
			fmt.Sprintf("Could not import localization '%s': %s\n\n"+
				"Import ID should be either the localization ID on its own, or "+
				"\"<in_app_purchase_id>/<localization_id>\".", localizationID, err.Error()),
		)
		return
	}

	reported, err := r.client.GetInAppPurchaseLocalizationPurchaseID(localization)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Resolving Localization Owner",
			fmt.Sprintf("Could not resolve the in-app purchase behind localization '%s': %s\n\n"+
				"Apple reports the version that owns a localization, and the version reports the "+
				"purchase; one of those two hops failed.", localizationID, err.Error()),
		)
		return
	}

	if composite && reported != "" && reported != purchaseID {
		resp.Diagnostics.AddError(
			"Localization Not Owned By In-App Purchase",
			fmt.Sprintf("Localization '%s' belongs to in-app purchase '%s', not '%s'.",
				localizationID, reported, purchaseID),
		)
		return
	}

	if purchaseID == "" {
		purchaseID = reported
	}

	state := inAppPurchaseLocalizationModel{
		ID:              types.StringValue(localization.ID),
		InAppPurchaseID: types.StringValue(purchaseID),
		VersionID:       types.StringNull(),
	}
	if localization.Relationships != nil {
		state.VersionID = relationshipID(localization.Relationships.Version)
	}
	applyLocalizationAttributes(&state, localization)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// applyLocalizationAttributes copies Apple's reported attributes into a model.
func applyLocalizationAttributes(model *inAppPurchaseLocalizationModel, localization *models.InAppPurchaseLocalization) {
	model.Locale = types.StringValue(localization.Attributes.Locale)
	model.Name = types.StringValue(localization.Attributes.Name)
	model.Description = stringOrNull(localization.Attributes.Description)
}
