// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
	_ resource.Resource                   = &appStoreVersionLocalizationResource{}
	_ resource.ResourceWithConfigure      = &appStoreVersionLocalizationResource{}
	_ resource.ResourceWithImportState    = &appStoreVersionLocalizationResource{}
	_ resource.ResourceWithValidateConfig = &appStoreVersionLocalizationResource{}
)

func NewAppStoreVersionLocalizationResource() resource.Resource {
	return &appStoreVersionLocalizationResource{}
}

type appStoreVersionLocalizationResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *appStoreVersionLocalizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_store_version_localization"
}

// Schema defines the schema for the resource.
func (r *appStoreVersionLocalizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages one language of a version's App Store product page.\n\n" +
			"This is the per-release half of the localized metadata: the description, the keywords, the " +
			"promotional text and the release notes. The app's name, subtitle and privacy policy link " +
			"outlive any release and belong to `apple_app_info_localization` instead.\n\n" +
			"~> **`keywords` is a list here and a single 100-character string at Apple.** The provider " +
			"joins the list with commas and no spaces — a space is a character, and every one of them " +
			"comes out of the hundred. The limit applies to the joined string, so it is checked after " +
			"joining rather than per keyword.\n\n" +
			"The localization for the app's primary language cannot be deleted; destroying that one " +
			"warns and drops it from state.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the localization.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_store_version_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_app_store_version` this product page belongs " +
					"to. Cannot be changed.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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
			"description": schema.StringAttribute{
				MarkdownDescription: "The product page description, at most 4000 characters. Counted in " +
					"characters rather than bytes, so an Arabic or Japanese description gets the full " +
					"four thousand.",
				Optional:   true,
				Validators: GetVersionDescriptionValidator(),
			},
			"keywords": schema.ListAttribute{
				MarkdownDescription: "Search keywords for this language. Joined with commas into the " +
					"single string Apple stores, which is capped at **100 characters including the " +
					"commas** — the validator counts the joined length, not the list. Do not repeat " +
					"words already in the app name or subtitle; Apple indexes those separately.",
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"promotional_text": schema.StringAttribute{
				MarkdownDescription: "Up to 170 characters shown above the description. The one piece of " +
					"product page text that can be changed **without submitting a new version**, which " +
					"is what it is for.",
				Optional:   true,
				Validators: GetPromotionalTextValidator(),
			},
			"marketing_url": schema.StringAttribute{
				MarkdownDescription: "A link to the app's marketing site for this language.",
				Optional:            true,
				Validators:          []validator.String{URLValidator},
			},
			"support_url": schema.StringAttribute{
				MarkdownDescription: "A link to the app's support page for this language. Required by " +
					"App Review before a version can be submitted.",
				Optional:   true,
				Validators: []validator.String{URLValidator},
			},
			"whats_new": schema.StringAttribute{
				MarkdownDescription: "The release notes, at most 4000 characters. Apple rejects them on " +
					"the first version of an app — there is nothing new about it — and requires them on " +
					"every version after.",
				Optional:   true,
				Validators: GetWhatsNewValidator(),
			},
		},
	}
}

// ValidateConfig checks the joined keyword length.
//
// The limit Apple enforces is on the comma-joined string, so no per-element
// validator can express it: ten legal keywords can still be four characters too
// long together. Checking it here reports the real number at plan time instead
// of surfacing Apple's 409 during apply.
func (r *appStoreVersionLocalizationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config appStoreVersionLocalizationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// An unknown list -- keywords supplied from a variable, or through a
	// for_each -- has nothing to measure yet. Apple's own error is the backstop
	// for that case.
	if config.Keywords.IsNull() || config.Keywords.IsUnknown() {
		return
	}

	joined := joinKeywords(config.Keywords)
	if length := len([]rune(joined)); length > keywordsMaxLength {
		resp.Diagnostics.AddAttributeError(
			path.Root("keywords"),
			"Keywords Too Long",
			fmt.Sprintf("Apple stores keywords as one comma-separated string capped at %d characters. "+
				"These %d keywords join to %d characters:\n\n  %s\n\nShorten or drop some.",
				keywordsMaxLength, len(config.Keywords.Elements()), length, joined),
		)
	}
}

// Create a new resource.
func (r *appStoreVersionLocalizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating app store version localization resource")

	var plan appStoreVersionLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	versionID := plan.AppStoreVersionID.ValueString()
	locale := plan.Locale.ValueString()
	ctx = tflog.SetField(ctx, "app_store_version_id", versionID)
	ctx = tflog.SetField(ctx, "locale", locale)

	localization, err := r.client.CreateAppStoreVersionLocalization(versionID, models.AppStoreVersionLocalizationCreateAttributes{
		Locale:          locale,
		Description:     stringOrNil(plan.Description),
		Keywords:        keywordsOrNil(plan.Keywords),
		PromotionalText: stringOrNil(plan.PromotionalText),
		MarketingURL:    stringOrNil(plan.MarketingURL),
		SupportURL:      stringOrNil(plan.SupportURL),
		WhatsNew:        stringOrNil(plan.WhatsNew),
	}, nil)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "already exist"):
			resp.Diagnostics.AddError(
				"App Store Version Localization Already Exists",
				fmt.Sprintf("Version '%s' already has a localization for locale '%s'. Import it "+
					"instead:\n\n  terraform import <address> %s/%s", versionID, locale, versionID, locale),
			)
		case strings.Contains(errMsg, "whatsNew"):
			resp.Diagnostics.AddError(
				"Release Notes Not Accepted",
				fmt.Sprintf("Apple refused the release notes on version '%s': %s\n\n"+
					"Apple rejects whats_new on the first version of an app — there is nothing new "+
					"about it — and requires it on every version after.", versionID, errMsg),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			resp.Diagnostics.AddError(
				"App Store Version Not Found",
				fmt.Sprintf("Could not localize version '%s': %s", versionID, errMsg),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Creating App Store Version Localization",
				fmt.Sprintf("Could not create the '%s' localization of version '%s': %s", locale, versionID, errMsg),
			)
		}
		return
	}

	applyVersionLocalization(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *appStoreVersionLocalizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading app store version localization resource")

	var state appStoreVersionLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	localization, err := r.client.GetAppStoreVersionLocalization(localizationID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "App store version localization not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading App Store Version Localization",
			fmt.Sprintf("Could not read app store version localization '%s': %s", localizationID, err.Error()),
		)
		return
	}

	applyVersionLocalization(&state, localization)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the localization in place.
func (r *appStoreVersionLocalizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating app store version localization resource")

	var plan appStoreVersionLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state appStoreVersionLocalizationModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	localization, err := r.client.UpdateAppStoreVersionLocalization(localizationID, models.AppStoreVersionLocalizationUpdateAttributes{
		Description:     stringOrNil(plan.Description),
		Keywords:        keywordsOrNil(plan.Keywords),
		PromotionalText: stringOrNil(plan.PromotionalText),
		MarketingURL:    stringOrNil(plan.MarketingURL),
		SupportURL:      stringOrNil(plan.SupportURL),
		WhatsNew:        stringOrNil(plan.WhatsNew),
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating App Store Version Localization",
			fmt.Sprintf("Could not update the '%s' localization of version '%s': %s",
				plan.Locale.ValueString(), plan.AppStoreVersionID.ValueString(), err.Error()),
		)
		return
	}

	applyVersionLocalization(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete removes the localization.
//
// Apple refuses to delete the one for the app's primary locale. Erroring there
// would fail the whole destroy and leave the version behind, so that case warns
// and drops state instead.
func (r *appStoreVersionLocalizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appStoreVersionLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	locale := state.Locale.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	if err := r.client.DeleteAppStoreVersionLocalization(localizationID, nil); err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			tflog.Info(ctx, "App store version localization already gone")
		case strings.Contains(errMsg, "primary locale") ||
			strings.Contains(errMsg, "last localization") ||
			strings.Contains(errMsg, "cannot be deleted"):
			resp.Diagnostics.AddWarning(
				"App Store Version Localization Not Deleted",
				fmt.Sprintf("Apple refused to delete the '%s' localization of version '%s': %s\n\n"+
					"A version must keep a product page in the app's primary language. Terraform has "+
					"removed the resource from state only.",
					locale, state.AppStoreVersionID.ValueString(), errMsg),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Deleting App Store Version Localization",
				fmt.Sprintf("Could not delete the '%s' localization of version '%s': %s",
					locale, state.AppStoreVersionID.ValueString(), errMsg),
			)
			return
		}
	}

	tflog.Info(ctx, "App store version localization deleted")
}

// Configure adds the provider configured client to the resource.
func (r *appStoreVersionLocalizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
// Two forms are accepted: a bare Apple localization ID -- the client asks for
// include=appStoreVersion, so the parent comes back with it -- and the
// composite "<app_store_version_id>/<locale>".
func (r *appStoreVersionLocalizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing app store version localization resource", map[string]interface{}{"import_id": req.ID})

	var (
		localization *models.AppStoreVersionLocalization
		versionID    string
		err          error
	)

	if parent, locale, ok := strings.Cut(req.ID, "/"); ok {
		versionID = parent
		localization, err = r.client.GetAppStoreVersionLocalizationByLocale(parent, locale)
	} else {
		localization, err = r.client.GetAppStoreVersionLocalization(req.ID)
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing App Store Version Localization",
			fmt.Sprintf("Could not read app store version localization '%s': %s\n\n"+
				"Accepted import IDs are a bare Apple localization ID and "+
				"'<app_store_version_id>/<locale>'.", req.ID, err.Error()),
		)
		return
	}

	state := appStoreVersionLocalizationModel{}
	applyVersionLocalization(&state, localization)

	if versionID == "" && localization.Relationships != nil {
		versionID = localization.Relationships.AppStoreVersion.Data.ID
	}
	if versionID == "" {
		resp.Diagnostics.AddError(
			"Incomplete App Store Version Localization Import",
			fmt.Sprintf("Apple did not report which version localization '%s' belongs to. Import by "+
				"'<app_store_version_id>/<locale>' instead.", req.ID),
		)
		return
	}
	state.AppStoreVersionID = types.StringValue(versionID)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// applyVersionLocalization copies Apple's view of the record into the model.
func applyVersionLocalization(model *appStoreVersionLocalizationModel, localization *models.AppStoreVersionLocalization) {
	model.ID = types.StringValue(localization.ID)
	model.Locale = types.StringValue(localization.Attributes.Locale)
	model.Description = stringOrNull(localization.Attributes.Description)
	model.Keywords = splitKeywords(localization.Attributes.Keywords)
	model.PromotionalText = stringOrNull(localization.Attributes.PromotionalText)
	model.MarketingURL = stringOrNull(localization.Attributes.MarketingURL)
	model.SupportURL = stringOrNull(localization.Attributes.SupportURL)
	model.WhatsNew = stringOrNull(localization.Attributes.WhatsNew)
}

// keywordsOrNil renders a keyword list for the wire, omitting the attribute
// when nothing was configured.
func keywordsOrNil(keywords types.List) *string {
	joined := joinKeywords(keywords)
	if joined == "" {
		return nil
	}

	return &joined
}
