// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package beta

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
	_ resource.Resource                = &betaAppLocalizationResource{}
	_ resource.ResourceWithConfigure   = &betaAppLocalizationResource{}
	_ resource.ResourceWithImportState = &betaAppLocalizationResource{}
)

func NewBetaAppLocalizationResource() resource.Resource {
	return &betaAppLocalizationResource{}
}

type betaAppLocalizationResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *betaAppLocalizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_beta_app_localization"
}

// Schema defines the schema for the resource.
func (r *betaAppLocalizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the text on an app's TestFlight page in one language: the " +
			"description testers read before they install, the address their feedback goes to, and the " +
			"marketing and privacy links shown beside it.\n\n" +
			"This is the half of TestFlight metadata that **survives every build**. The per-build " +
			"\"What to Test\" note is `apple_beta_build_localization`, and it is replaced with each " +
			"upload — the same two-lifetime split `apple_app_info_localization` and " +
			"`apple_app_store_version_localization` have on the App Store side.\n\n" +
			"An app holds at most one of these per locale. Apple creates one for the app's primary " +
			"locale on its own, so this resource adopts an existing record when it finds one and creates " +
			"it otherwise.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the localization.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app this text belongs to. Read it from the " +
					"`apple_apps` data source. Cannot be changed.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"locale": schema.StringAttribute{
				MarkdownDescription: "The language this text is in, as an App Store locale code — " +
					"`en-US`, `es-MX`, `ar-SA`. It identifies the record and is absent from Apple's " +
					"update request, so changing it replaces the localization.",
				Required:   true,
				Validators: []validator.String{LocaleValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "What the app is, as testers see it on the TestFlight page before " +
					"installing. At most 4000 characters, counted in characters rather than bytes.",
				Optional:   true,
				Validators: GetBetaDescriptionValidator(),
			},
			"feedback_email": schema.StringAttribute{
				MarkdownDescription: "Where tester feedback is sent. This address is shown to external " +
					"testers, so prefer a shared inbox to a person's own.",
				Optional:   true,
				Validators: []validator.String{EmailValidator},
			},
			"marketing_url": schema.StringAttribute{
				MarkdownDescription: "A link testers can follow to read more about the app.",
				Optional:            true,
				Validators:          []validator.String{URLValidator},
			},
			"privacy_policy_url": schema.StringAttribute{
				MarkdownDescription: "A link to the privacy policy shown to TestFlight testers. This is " +
					"the TestFlight link, not the App Store one on `apple_app_info_localization`, and " +
					"not the App Privacy questionnaire — which has no API at all.",
				Optional:   true,
				Validators: []validator.String{URLValidator},
			},
			"tvos_privacy_policy": schema.StringAttribute{
				MarkdownDescription: "The privacy policy as **text rather than a link**, for tvOS, where " +
					"there is no browser to open a URL in. Ignored on every other platform.",
				Optional: true,
			},
		},
	}
}

// Create adopts an existing localization or creates one.
//
// Apple writes a localization for the app's primary locale itself, so a plain
// POST would 409 on exactly the locale a configuration is most likely to start
// with. Reading first and patching when a record exists makes the outcome the
// same either way.
func (r *betaAppLocalizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating beta app localization resource")

	var plan betaAppLocalizationModel
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
func (r *betaAppLocalizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading beta app localization resource")

	var state betaAppLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	localization, err := r.client.GetBetaAppLocalization(localizationID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Beta app localization not found, removing from state")
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Beta App Localization",
			fmt.Sprintf("Could not read beta app localization '%s': %s", localizationID, err.Error()),
		)

		return
	}

	applyBetaAppLocalization(&state, localization)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update patches the localization in place.
func (r *betaAppLocalizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating beta app localization resource")

	var plan betaAppLocalizationModel
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

// Delete removes the localization.
//
// Apple refuses to remove the last one an app has, and the refusal is tolerated
// rather than surfaced: erroring there would fail the whole destroy over a
// record that goes away with the app anyway.
func (r *betaAppLocalizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state betaAppLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	if err := r.client.DeleteBetaAppLocalization(localizationID, nil); err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			tflog.Info(ctx, "Beta app localization already gone")
		case strings.Contains(errMsg, "last localization") || strings.Contains(errMsg, "at least one"):
			resp.Diagnostics.AddWarning(
				"Beta App Localization Not Deleted",
				fmt.Sprintf("Apple refused to delete the '%s' TestFlight text of app '%s': %s\n\n"+
					"An app must keep at least one. Terraform has removed the resource from state only; "+
					"the record goes away with the app.",
					state.Locale.ValueString(), state.AppID.ValueString(), errMsg),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Deleting Beta App Localization",
				fmt.Sprintf("Could not delete beta app localization '%s': %s", localizationID, errMsg),
			)

			return
		}
	}

	tflog.Info(ctx, "Beta app localization deleted")
}

// Configure adds the provider configured client to the resource.
func (r *betaAppLocalizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports a beta app localization.
//
// Two forms: a bare Apple localization ID -- the client asks for include=app,
// so the parent comes back with it -- and the composite "<app_id>/<locale>",
// which is what a person actually has.
func (r *betaAppLocalizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing beta app localization resource", map[string]interface{}{"import_id": req.ID})

	var (
		localization *models.BetaAppLocalization
		err          error
	)

	parts := strings.SplitN(req.ID, "/", 2)
	switch len(parts) {
	case 1:
		localization, err = r.client.GetBetaAppLocalization(req.ID)
	case 2:
		localization, err = r.client.GetBetaAppLocalizationByLocale(parts[0], parts[1])
	default:
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Could not parse import ID '%s'. Accepted forms are a bare Apple localization ID "+
				"and '<app_id>/<locale>'.", req.ID),
		)

		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Beta App Localization",
			fmt.Sprintf("Could not read beta app localization '%s': %s", req.ID, err.Error()),
		)

		return
	}

	state := betaAppLocalizationModel{}
	if len(parts) == 2 {
		state.AppID = types.StringValue(parts[0])
	}

	applyBetaAppLocalization(&state, localization)

	if state.AppID.IsNull() {
		resp.Diagnostics.AddError(
			"Incomplete Beta App Localization Import",
			fmt.Sprintf("Apple did not report which app localization '%s' belongs to, so app_id cannot be "+
				"written. Import with '<app_id>/<locale>' instead.", req.ID),
		)

		return
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write creates the localization or patches the one already there.
func (r *betaAppLocalizationResource) write(ctx context.Context, plan *betaAppLocalizationModel, diags *diag.Diagnostics) {
	appID := plan.AppID.ValueString()
	locale := plan.Locale.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)
	ctx = tflog.SetField(ctx, "locale", locale)

	var (
		localization *models.BetaAppLocalization
		err          error
	)

	existing, readErr := r.client.GetBetaAppLocalizationByLocale(appID, locale)
	switch {
	case readErr == nil:
		tflog.Debug(ctx, "Patching existing beta app localization", map[string]interface{}{"localization_id": existing.ID})
		localization, err = r.client.UpdateBetaAppLocalization(existing.ID, models.BetaAppLocalizationUpdateAttributes{
			Description:       stringOrNil(plan.Description),
			FeedbackEmail:     stringOrNil(plan.FeedbackEmail),
			MarketingURL:      stringOrNil(plan.MarketingURL),
			PrivacyPolicyURL:  stringOrNil(plan.PrivacyPolicyURL),
			TVOSPrivacyPolicy: stringOrNil(plan.TVOSPrivacyPolicy),
		}, nil)
	case strings.Contains(readErr.Error(), "not found") || strings.Contains(readErr.Error(), "404"):
		tflog.Debug(ctx, "Creating beta app localization")
		localization, err = r.client.CreateBetaAppLocalization(appID, models.BetaAppLocalizationCreateAttributes{
			Locale:            locale,
			Description:       stringOrNil(plan.Description),
			FeedbackEmail:     stringOrNil(plan.FeedbackEmail),
			MarketingURL:      stringOrNil(plan.MarketingURL),
			PrivacyPolicyURL:  stringOrNil(plan.PrivacyPolicyURL),
			TVOSPrivacyPolicy: stringOrNil(plan.TVOSPrivacyPolicy),
		}, nil)
	default:
		diags.AddError(
			"Error Reading Beta App Localizations",
			fmt.Sprintf("Could not check whether app '%s' already has TestFlight text for '%s': %s",
				appID, locale, readErr.Error()),
		)

		return
	}

	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "locale") && strings.Contains(errMsg, "INVALID"):
			diags.AddError(
				"Locale Not Available",
				fmt.Sprintf("Apple refused locale '%s' for app '%s': %s\n\n"+
					"TestFlight accepts the App Store locale codes — en-US, es-MX, ar-SA.", locale, appID, errMsg),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			diags.AddError(
				"App Not Found",
				fmt.Sprintf("Could not write TestFlight text for app '%s': %s", appID, errMsg),
			)
		default:
			diags.AddError(
				"Error Writing Beta App Localization",
				fmt.Sprintf("Could not write the '%s' TestFlight text of app '%s': %s", locale, appID, errMsg),
			)
		}
		tflog.Error(ctx, "Failed to write beta app localization", map[string]interface{}{"error": errMsg})

		return
	}

	applyBetaAppLocalization(plan, localization)
}

// applyBetaAppLocalization copies Apple's view of the record into the model.
func applyBetaAppLocalization(model *betaAppLocalizationModel, localization *models.BetaAppLocalization) {
	model.ID = types.StringValue(localization.ID)

	if localization.Attributes.Locale != "" {
		model.Locale = types.StringValue(localization.Attributes.Locale)
	}

	model.Description = stringOrNull(localization.Attributes.Description)
	model.FeedbackEmail = stringOrNull(localization.Attributes.FeedbackEmail)
	model.MarketingURL = stringOrNull(localization.Attributes.MarketingURL)
	model.PrivacyPolicyURL = stringOrNull(localization.Attributes.PrivacyPolicyURL)
	model.TVOSPrivacyPolicy = stringOrNull(localization.Attributes.TVOSPrivacyPolicy)

	if localization.Relationships != nil {
		if appID := relationshipID(localization.Relationships.App); !appID.IsNull() {
			model.AppID = appID
		}
	}
}
