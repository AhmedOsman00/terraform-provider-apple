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
	_ resource.Resource                = &appSettingsResource{}
	_ resource.ResourceWithConfigure   = &appSettingsResource{}
	_ resource.ResourceWithImportState = &appSettingsResource{}
)

func NewAppSettingsResource() resource.Resource {
	return &appSettingsResource{}
}

type appSettingsResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *appSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_settings"
}

// Schema defines the schema for the resource.
func (r *appSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the account-level settings on an App Store Connect app record.\n\n" +
			"This is where **Content Rights Information** lives — the App Information question App Store " +
			"Connect blocks a submission on until it is answered. It also carries the app's primary " +
			"language and the server-to-server notification URLs for subscriptions.\n\n" +
			"~> **This resource adopts an app that already exists; it does not create or destroy one.** " +
			"Apple's API cannot create an app record — its documentation says to create apps on the App " +
			"Store Connect website — and cannot delete one either. `terraform apply` patches the " +
			"existing record and `terraform destroy` drops it from state and warns, leaving every " +
			"setting exactly as it was. Find the app's ID with the `apple_apps` data source.",

		Attributes: map[string]schema.Attribute{
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app these settings belong to. Read it from the " +
					"`apple_apps` data source. Cannot be changed — pointing this resource at a different " +
					"app replaces it.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"content_rights_declaration": schema.StringAttribute{
				MarkdownDescription: "Whether the app contains, shows or accesses third-party content. " +
					"One of `DOES_NOT_USE_THIRD_PARTY_CONTENT` or `USES_THIRD_PARTY_CONTENT`.\n\n" +
					"This is App Store Connect's *Content Rights Information* question. Leaving it unset " +
					"leaves Apple's answer alone, which for a new app means unanswered — and an " +
					"unanswered declaration blocks the submission with \"You must set up Content Rights " +
					"Information in App Information\".",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{ContentRightsValidator},
			},
			"primary_locale": schema.StringAttribute{
				MarkdownDescription: "The app's primary language, as an App Store locale code such as " +
					"`en-US`. This is the language App Review falls back to, and the one whose " +
					"`apple_app_info_localization` cannot be deleted.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{LocaleValidator},
			},
			"accessibility_url": schema.StringAttribute{
				MarkdownDescription: "A link to the app's accessibility information, shown on the product page.",
				Optional:            true,
				Validators:          []validator.String{URLValidator},
			},
			"subscription_status_url": schema.StringAttribute{
				MarkdownDescription: "The production URL Apple posts App Store server notifications to. " +
					"Only relevant to an app selling auto-renewable subscriptions.",
				Optional:   true,
				Validators: []validator.String{URLValidator},
			},
			"subscription_status_url_version": schema.StringAttribute{
				MarkdownDescription: "The version of the server notification payload sent to " +
					"`subscription_status_url`. One of `V1` or `V2`. V2 is the current format.",
				Optional:   true,
				Validators: []validator.String{SubscriptionStatusURLVersionValidator},
			},
			"subscription_status_url_for_sandbox": schema.StringAttribute{
				MarkdownDescription: "The sandbox URL Apple posts App Store server notifications to.",
				Optional:            true,
				Validators:          []validator.String{URLValidator},
			},
			"subscription_status_url_version_for_sandbox": schema.StringAttribute{
				MarkdownDescription: "The version of the server notification payload sent to " +
					"`subscription_status_url_for_sandbox`. One of `V1` or `V2`.",
				Optional:   true,
				Validators: []validator.String{SubscriptionStatusURLVersionValidator},
			},
			"streamlined_purchasing_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the app uses Apple's streamlined purchasing flow. Apple " +
					"treats this as one-way on a live app: it can be turned on but not reliably turned " +
					"off again.",
				Optional: true,
				Computed: true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The app's name in App Store Connect. Set on the website when the " +
					"app was created; the customer-facing name is on `apple_app_info_localization`.",
				Computed: true,
			},
			"bundle_id": schema.StringAttribute{
				MarkdownDescription: "The app's bundle identifier. Read-only here — repointing a live " +
					"app at a different App ID is not something a Terraform attribute should be able to " +
					"do by accident.",
				Computed: true,
			},
			"sku": schema.StringAttribute{
				MarkdownDescription: "The app's SKU, fixed when the app record was created.",
				Computed:            true,
			},
		},
	}
}

// Create adopts the existing app record and applies the configured settings.
func (r *appSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating app settings resource")

	var plan appSettingsModel
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
func (r *appSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading app settings resource")

	var state appSettingsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := state.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	app, err := r.client.GetApp(appID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "App not found, removing settings from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading App Settings",
			fmt.Sprintf("Could not read app '%s': %s", appID, err.Error()),
		)
		return
	}

	applyAppSettings(&state, app)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update patches the app record in place.
func (r *appSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating app settings resource")

	var plan appSettingsModel
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

// Delete drops the settings from state.
//
// Apple cannot delete an app record, and there is nothing sensible to reset
// these settings to: clearing the content rights declaration would put the app
// back into the state that blocks a submission.
func (r *appSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appSettingsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"App Settings Not Deleted",
		fmt.Sprintf("Apple's API cannot delete an app record, and these settings have no meaningful "+
			"empty value — clearing the content rights declaration would put app '%s' back into the "+
			"state that blocks a submission. Terraform has removed the resource from state only; every "+
			"setting remains as last applied.", state.AppID.ValueString()),
	)

	tflog.Info(ctx, "App settings removed from state", map[string]interface{}{
		"app_id": state.AppID.ValueString(),
	})
}

// Configure adds the provider configured client to the resource.
func (r *appSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an app's settings.
//
// The import ID is the app ID. A bundle identifier is accepted too, since that
// is what a person is likely to have to hand: it is distinguished by shape,
// Apple's own app IDs being all digits.
func (r *appSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing app settings resource", map[string]interface{}{"import_id": req.ID})

	var (
		app *models.App
		err error
	)

	if strings.Contains(req.ID, ".") {
		app, err = r.client.GetAppByBundleID(req.ID)
	} else {
		app, err = r.client.GetApp(req.ID)
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing App Settings",
			fmt.Sprintf("Could not read app '%s': %s\n\n"+
				"The import ID is the app's Apple ID (all digits, from the apple_apps data source) or "+
				"its bundle identifier.", req.ID, err.Error()),
		)
		return
	}

	state := appSettingsModel{AppID: types.StringValue(app.ID)}
	applyAppSettings(&state, app)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write patches the app record and refreshes the computed attributes from
// Apple's response.
func (r *appSettingsResource) write(ctx context.Context, plan *appSettingsModel, diags *diag.Diagnostics) {
	appID := plan.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	tflog.Debug(ctx, "Writing app settings")

	app, err := r.client.UpdateApp(appID, models.AppUpdateAttributes{
		PrimaryLocale:                          stringOrNil(plan.PrimaryLocale),
		ContentRightsDeclaration:               stringOrNil(plan.ContentRightsDeclaration),
		AccessibilityURL:                       stringOrNil(plan.AccessibilityURL),
		SubscriptionStatusURL:                  stringOrNil(plan.SubscriptionStatusURL),
		SubscriptionStatusURLVersion:           stringOrNil(plan.SubscriptionStatusURLVersion),
		SubscriptionStatusURLForSandbox:        stringOrNil(plan.SubscriptionStatusURLForSandbox),
		SubscriptionStatusURLVersionForSandbox: stringOrNil(plan.SubscriptionStatusURLVersionForSandbox),
		StreamlinedPurchasingEnabled:           boolOrNil(plan.StreamlinedPurchasingEnabled),
	}, nil)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			diags.AddError(
				"App Not Found",
				fmt.Sprintf("Could not update app '%s': %s\n\n"+
					"Apple's API cannot create an app record. The app must already exist in App Store "+
					"Connect — find its ID with the apple_apps data source.", appID, errMsg),
			)
		case strings.Contains(errMsg, "streamlinedPurchasing"):
			diags.AddError(
				"Streamlined Purchasing Cannot Be Changed",
				fmt.Sprintf("Could not update app '%s': %s\n\n"+
					"Apple treats streamlined_purchasing_enabled as one-way on a live app: it can be "+
					"turned on but not reliably turned off again.", appID, errMsg),
			)
		default:
			diags.AddError(
				"Error Writing App Settings",
				fmt.Sprintf("Could not update app '%s': %s", appID, errMsg),
			)
		}
		tflog.Error(ctx, "Failed to write app settings", map[string]interface{}{"error": errMsg})

		return
	}

	applyAppSettings(plan, app)
}

// applyAppSettings copies Apple's view of the record into the model.
//
// The optional URL attributes are deliberately not refreshed: Apple omits a URL
// it holds no value for, and writing null over a configured value would produce
// a diff on the next plan. The computed attributes are always taken from Apple,
// since nothing else can supply them.
func applyAppSettings(model *appSettingsModel, app *models.App) {
	model.AppID = types.StringValue(app.ID)
	model.Name = types.StringValue(app.Attributes.Name)
	model.BundleID = types.StringValue(app.Attributes.BundleID)
	model.SKU = types.StringValue(app.Attributes.SKU)

	if app.Attributes.PrimaryLocale != "" {
		model.PrimaryLocale = types.StringValue(app.Attributes.PrimaryLocale)
	}
	if app.Attributes.ContentRightsDeclaration != nil {
		model.ContentRightsDeclaration = types.StringValue(*app.Attributes.ContentRightsDeclaration)
	} else if model.ContentRightsDeclaration.IsUnknown() {
		// Optional+Computed: an unanswered declaration has to resolve to
		// something, and null is the honest answer.
		model.ContentRightsDeclaration = types.StringNull()
	}
	if app.Attributes.StreamlinedPurchasingEnabled != nil {
		model.StreamlinedPurchasingEnabled = types.BoolValue(*app.Attributes.StreamlinedPurchasingEnabled)
	} else if model.StreamlinedPurchasingEnabled.IsUnknown() {
		model.StreamlinedPurchasingEnabled = types.BoolNull()
	}
}
