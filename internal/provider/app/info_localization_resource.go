// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

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
	_ resource.Resource                = &appInfoLocalizationResource{}
	_ resource.ResourceWithConfigure   = &appInfoLocalizationResource{}
	_ resource.ResourceWithImportState = &appInfoLocalizationResource{}
)

func NewAppInfoLocalizationResource() resource.Resource {
	return &appInfoLocalizationResource{}
}

type appInfoLocalizationResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *appInfoLocalizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_info_localization"
}

// Schema defines the schema for the resource.
func (r *appInfoLocalizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an app's name, subtitle and privacy policy link in one language.\n\n" +
			"This is the half of the localized metadata that survives a release. The description, " +
			"keywords and promotional text belong to a particular version and live on " +
			"`apple_app_store_version_localization` instead — a split Apple makes in the API and App " +
			"Store Connect hides in its editor.\n\n" +
			"~> **Writes only land while a version is being prepared.** These records hang off the app's " +
			"AppInfo, which Apple freezes once it is in review or distributed.\n\n" +
			"The localization for the app's primary language cannot be deleted: every app must have a " +
			"name in the language it was created in. Destroying that one warns and drops it from state " +
			"rather than failing the whole destroy.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the localization.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app. Cannot be changed.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"app_info_id": schema.StringAttribute{
				MarkdownDescription: "The AppInfo record this localization was written to. Resolved by " +
					"the provider; Apple issues a new one each version cycle.",
				Computed: true,
			},
			"locale": schema.StringAttribute{
				MarkdownDescription: "The App Store locale this text is for, such as `en-US`, `ar-SA` or " +
					"`es-MX`. Identifies the record and is absent from Apple's update request, so " +
					"changing it replaces the localization.",
				Required:   true,
				Validators: []validator.String{LocaleValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The app's name on the App Store in this language. At most 30 " +
					"characters — counted in characters, not bytes, so Arabic and Japanese names get " +
					"the full thirty.",
				Required:   true,
				Validators: GetAppNameValidator(),
			},
			"subtitle": schema.StringAttribute{
				MarkdownDescription: "The line shown under the app name, at most 30 characters. Not a " +
					"tagline for the developer's benefit: it is indexed for search.",
				Optional:   true,
				Validators: GetSubtitleValidator(),
			},
			"privacy_policy_url": schema.StringAttribute{
				MarkdownDescription: "A link to the app's privacy policy in this language.\n\n" +
					"~> This is **not** App Store Connect's App Privacy section. That questionnaire — " +
					"the one that blocks a submission with \"Admin must provide information about the " +
					"app's privacy practices\" — has no API at all and must be answered on the website.",
				Optional:   true,
				Validators: []validator.String{URLValidator},
			},
			"privacy_choices_url": schema.StringAttribute{
				MarkdownDescription: "A link to a page where customers can manage their privacy choices.",
				Optional:            true,
				Validators:          []validator.String{URLValidator},
			},
			"privacy_policy_text": schema.StringAttribute{
				MarkdownDescription: "Privacy policy text shown in place of a link. Apple uses this only " +
					"for Apple TV apps; every other platform wants `privacy_policy_url`.",
				Optional: true,
			},
		},
	}
}

// Create a new resource.
func (r *appInfoLocalizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating app info localization resource")

	var plan appInfoLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := plan.AppID.ValueString()
	locale := plan.Locale.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)
	ctx = tflog.SetField(ctx, "locale", locale)

	info, err := r.client.GetEditableAppInfo(appID)
	if err != nil {
		resp.Diagnostics.AddError(
			"No Editable App Info",
			fmt.Sprintf("Could not resolve an app info record to write for app '%s': %s", appID, err.Error()),
		)
		return
	}

	localization, err := r.client.CreateAppInfoLocalization(info.ID, models.AppInfoLocalizationCreateAttributes{
		Locale:            locale,
		Name:              plan.Name.ValueString(),
		Subtitle:          stringOrNil(plan.Subtitle),
		PrivacyPolicyURL:  stringOrNil(plan.PrivacyPolicyURL),
		PrivacyChoicesURL: stringOrNil(plan.PrivacyChoicesURL),
		PrivacyPolicyText: stringOrNil(plan.PrivacyPolicyText),
	}, nil)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "already exist"):
			resp.Diagnostics.AddError(
				"App Info Localization Already Exists",
				fmt.Sprintf("App '%s' already has a localization for locale '%s'. Import it instead:\n\n"+
					"  terraform import <address> %s/%s", appID, locale, appID, locale),
			)
		case strings.Contains(errMsg, "STATE_ERROR") || strings.Contains(errMsg, "not allowed"):
			resp.Diagnostics.AddError(
				"App Info Not Editable",
				fmt.Sprintf("Apple refused the localization on app '%s': %s\n\n"+
					"An app info in review or already distributed is frozen. Wait for the current "+
					"review to finish.", appID, errMsg),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Creating App Info Localization",
				fmt.Sprintf("Could not create the '%s' localization of app '%s': %s", locale, appID, errMsg),
			)
		}
		return
	}

	plan.AppInfoID = types.StringValue(info.ID)
	applyAppInfoLocalization(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
//
// The record is looked up by locale on the editable AppInfo rather than by the
// stored ID: Apple issues a new AppInfo each version cycle, with new
// localization IDs, so following the stored ID would eventually read a frozen
// record -- or nothing at all.
func (r *appInfoLocalizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading app info localization resource")

	var state appInfoLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := state.AppID.ValueString()
	locale := state.Locale.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)
	ctx = tflog.SetField(ctx, "locale", locale)

	info, err := r.client.GetEditableAppInfo(appID)
	if err != nil {
		if strings.Contains(err.Error(), "no editable app info") {
			tflog.Warn(ctx, "No editable app info; keeping state as-is")
			return
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "App not found, removing localization from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading App Info Localization",
			fmt.Sprintf("Could not resolve the app info of app '%s': %s", appID, err.Error()),
		)
		return
	}

	localization, err := r.client.GetAppInfoLocalizationByLocale(info.ID, locale)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "App info localization not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading App Info Localization",
			fmt.Sprintf("Could not read the '%s' localization of app '%s': %s", locale, appID, err.Error()),
		)
		return
	}

	state.AppInfoID = types.StringValue(info.ID)
	applyAppInfoLocalization(&state, localization)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the localization in place.
func (r *appInfoLocalizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating app info localization resource")

	var plan appInfoLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state appInfoLocalizationModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	localization, err := r.client.UpdateAppInfoLocalization(localizationID, models.AppInfoLocalizationUpdateAttributes{
		Name:              stringOrNil(plan.Name),
		Subtitle:          stringOrNil(plan.Subtitle),
		PrivacyPolicyURL:  stringOrNil(plan.PrivacyPolicyURL),
		PrivacyChoicesURL: stringOrNil(plan.PrivacyChoicesURL),
		PrivacyPolicyText: stringOrNil(plan.PrivacyPolicyText),
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating App Info Localization",
			fmt.Sprintf("Could not update the '%s' localization of app '%s': %s",
				plan.Locale.ValueString(), plan.AppID.ValueString(), err.Error()),
		)
		return
	}

	plan.AppInfoID = state.AppInfoID
	applyAppInfoLocalization(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete removes the localization.
//
// Apple refuses to delete the localization for the app's primary locale, since
// every app must keep a name in the language it was created in. Erroring there
// would fail the whole destroy and leave everything else behind, so that case
// warns and drops state instead -- the same shape apple_device and
// apple_in_app_purchase_localization use.
func (r *appInfoLocalizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appInfoLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	locale := state.Locale.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	if err := r.client.DeleteAppInfoLocalization(localizationID, nil); err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			tflog.Info(ctx, "App info localization already gone")
		case strings.Contains(errMsg, "primary locale") ||
			strings.Contains(errMsg, "last localization") ||
			strings.Contains(errMsg, "cannot be deleted"):
			resp.Diagnostics.AddWarning(
				"App Info Localization Not Deleted",
				fmt.Sprintf("Apple refused to delete the '%s' localization of app '%s': %s\n\n"+
					"An app must keep a name in its primary language. Terraform has removed the "+
					"resource from state only; the localization remains in App Store Connect.",
					locale, state.AppID.ValueString(), errMsg),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Deleting App Info Localization",
				fmt.Sprintf("Could not delete the '%s' localization of app '%s': %s",
					locale, state.AppID.ValueString(), errMsg),
			)
			return
		}
	}

	tflog.Info(ctx, "App info localization deleted")
}

// Configure adds the provider configured client to the resource.
func (r *appInfoLocalizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports a localization.
//
// Two forms are accepted: the composite "<app_id>/<locale>", which is what a
// person writing a configuration has to hand, and a bare Apple localization ID,
// which is resolved back to its app in two hops -- localization to app info,
// app info to app -- since an AppInfo does report the app it belongs to.
func (r *appInfoLocalizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing app info localization resource", map[string]interface{}{"import_id": req.ID})

	var state appInfoLocalizationModel

	if appID, locale, ok := strings.Cut(req.ID, "/"); ok {
		info, err := r.client.GetEditableAppInfo(appID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Importing App Info Localization",
				fmt.Sprintf("Could not resolve the app info of app '%s': %s", appID, err.Error()),
			)
			return
		}

		localization, err := r.client.GetAppInfoLocalizationByLocale(info.ID, locale)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Importing App Info Localization",
				fmt.Sprintf("Could not read the '%s' localization of app '%s': %s", locale, appID, err.Error()),
			)
			return
		}

		state.AppID = types.StringValue(appID)
		state.AppInfoID = types.StringValue(info.ID)
		applyAppInfoLocalization(&state, localization)
	} else {
		localization, err := r.client.GetAppInfoLocalization(req.ID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Importing App Info Localization",
				fmt.Sprintf("Could not read app info localization '%s': %s\n\n"+
					"Accepted import IDs are '<app_id>/<locale>' and a bare Apple localization ID.",
					req.ID, err.Error()),
			)
			return
		}

		if localization.Relationships == nil || localization.Relationships.AppInfo == nil {
			resp.Diagnostics.AddError(
				"Incomplete App Info Localization Import",
				fmt.Sprintf("Apple did not report which app info localization '%s' belongs to, so the "+
					"app cannot be resolved. Import by '<app_id>/<locale>' instead.", req.ID),
			)
			return
		}

		appInfoID := localization.Relationships.AppInfo.Data.ID
		info, err := r.client.GetAppInfo(appInfoID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Importing App Info Localization",
				fmt.Sprintf("Could not read app info '%s': %s", appInfoID, err.Error()),
			)
			return
		}

		if info.Relationships == nil || info.Relationships.App == nil {
			resp.Diagnostics.AddError(
				"Incomplete App Info Localization Import",
				fmt.Sprintf("Apple did not report which app owns app info '%s'. Import by "+
					"'<app_id>/<locale>' instead.", appInfoID),
			)
			return
		}

		state.AppID = types.StringValue(info.Relationships.App.Data.ID)
		state.AppInfoID = types.StringValue(appInfoID)
		applyAppInfoLocalization(&state, localization)
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// applyAppInfoLocalization copies Apple's view of the record into the model.
func applyAppInfoLocalization(model *appInfoLocalizationModel, localization *models.AppInfoLocalization) {
	model.ID = types.StringValue(localization.ID)
	model.Locale = types.StringValue(localization.Attributes.Locale)
	model.Name = types.StringValue(localization.Attributes.Name)
	model.Subtitle = stringOrNull(localization.Attributes.Subtitle)
	model.PrivacyPolicyURL = stringOrNull(localization.Attributes.PrivacyPolicyURL)
	model.PrivacyChoicesURL = stringOrNull(localization.Attributes.PrivacyChoicesURL)
	model.PrivacyPolicyText = stringOrNull(localization.Attributes.PrivacyPolicyText)
}
