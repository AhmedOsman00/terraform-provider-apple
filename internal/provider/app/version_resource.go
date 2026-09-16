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
	_ resource.Resource                = &appStoreVersionResource{}
	_ resource.ResourceWithConfigure   = &appStoreVersionResource{}
	_ resource.ResourceWithImportState = &appStoreVersionResource{}
)

func NewAppStoreVersionResource() resource.Resource {
	return &appStoreVersionResource{}
}

type appStoreVersionResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *appStoreVersionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_store_version"
}

// Schema defines the schema for the resource.
func (r *appStoreVersionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages one release of an app on one platform.\n\n" +
			"A version owns everything that describes a particular release rather than the app itself: " +
			"the version string, the copyright line, how it is released once approved, and — through " +
			"`apple_app_store_version_localization` — the description, keywords and release notes. What " +
			"describes the app across releases (name, subtitle, categories, age rating) belongs to " +
			"`apple_app_info` and `apple_app_info_localization`.\n\n" +
			"An app can hold only **one editable version per platform** at a time; Apple answers a " +
			"second with a 409 naming the one already being prepared. Import the existing one rather " +
			"than creating another.\n\n" +
			"Only an editable version can be deleted. A version that has been released, or is in " +
			"review, is part of the app's history — destroying it warns and drops it from state.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the version.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app this version belongs to. Read it from the " +
					"`apple_apps` data source. Cannot be changed.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "The platform this version is for: `IOS`, `MAC_OS`, `TV_OS` or " +
					"`VISION_OS`. Note that this is a different list from an `apple_bundle_id`'s " +
					"platform — Apple accepts `TV_OS` and `VISION_OS` here and rejects both there. " +
					"Absent from Apple's update request, so changing it replaces the version.",
				Required:   true,
				Validators: []validator.String{AppStorePlatformValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"version_string": schema.StringAttribute{
				MarkdownDescription: "The version number customers see, such as `1.0` or `2.14.3`. Up to " +
					"three dot-separated numbers. Updatable in place while the version is editable, and " +
					"it must match the `CFBundleShortVersionString` of the build attached to it.",
				Required:   true,
				Validators: GetVersionStringValidator(),
			},
			"copyright": schema.StringAttribute{
				MarkdownDescription: "The copyright line shown on the product page, conventionally the " +
					"year and the rights holder — `2026 AO Studio`. Apple wants the year alone, not a " +
					"`©` symbol, which it adds itself.",
				Optional:   true,
				Validators: GetCopyrightValidator(),
			},
			"release_type": schema.StringAttribute{
				MarkdownDescription: "How the version is released once App Review approves it:\n\n" +
					"- `AFTER_APPROVAL` — released automatically the moment it passes review.\n" +
					"- `MANUAL` — held until released by hand in App Store Connect.\n" +
					"- `SCHEDULED` — released at `earliest_release_date`.\n\n" +
					"This is the attribute an \"automatic release\" setting maps onto: automatic is " +
					"`AFTER_APPROVAL`, manual is `MANUAL`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{ReleaseTypeValidator},
			},
			"earliest_release_date": schema.StringAttribute{
				MarkdownDescription: "When a `SCHEDULED` release goes live, as an ISO 8601 timestamp with " +
					"seconds and a zone — `2026-03-01T08:00:00-07:00`. Ignored unless `release_type` is " +
					"`SCHEDULED`.",
				Optional:   true,
				Validators: []validator.String{TimestampValidator},
			},
			"review_type": schema.StringAttribute{
				MarkdownDescription: "Which review queue the version goes through: `APP_STORE` or " +
					"`NOTARIZATION`. Notarization is for Mac software distributed outside the App Store.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{ReviewTypeValidator},
			},
			"uses_idfa": schema.BoolAttribute{
				MarkdownDescription: "Whether the app uses the Advertising Identifier. Apple derives this " +
					"from the binary for recent submissions and rejects a value that contradicts it.",
				Optional: true,
			},
			"app_version_state": schema.StringAttribute{
				MarkdownDescription: "Where the version stands, for example `PREPARE_FOR_SUBMISSION`, " +
					"`WAITING_FOR_REVIEW` or `READY_FOR_DISTRIBUTION`. Apple's, never the provider's.",
				Computed: true,
			},
			"downloadable": schema.BoolAttribute{
				MarkdownDescription: "Whether the version is currently downloadable from the App Store.",
				Computed:            true,
			},
			"created_date": schema.StringAttribute{
				MarkdownDescription: "When Apple created the version record.",
				Computed:            true,
			},
		},
	}
}

// Create a new resource.
func (r *appStoreVersionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating app store version resource")

	var plan appStoreVersionModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := plan.AppID.ValueString()
	platform := plan.Platform.ValueString()
	versionString := plan.VersionString.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)
	ctx = tflog.SetField(ctx, "version_string", versionString)

	version, err := r.client.CreateAppStoreVersion(appID, models.AppStoreVersionCreateAttributes{
		Platform:            platform,
		VersionString:       versionString,
		Copyright:           stringOrNil(plan.Copyright),
		ReviewType:          stringOrNil(plan.ReviewType),
		ReleaseType:         stringOrNil(plan.ReleaseType),
		EarliestReleaseDate: stringOrNil(plan.EarliestReleaseDate),
		UsesIdfa:            boolOrNil(plan.UsesIdfa),
	}, nil)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "already exist") || strings.Contains(errMsg, "editable version"):
			resp.Diagnostics.AddError(
				"App Store Version Already Exists",
				fmt.Sprintf("Apple refused to create version '%s' of app '%s' on %s: %s\n\n"+
					"An app can hold only one editable version per platform. Import the existing one "+
					"instead:\n\n  terraform import <address> %s/%s/%s",
					versionString, appID, platform, errMsg, appID, platform, versionString),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			resp.Diagnostics.AddError(
				"App Not Found",
				fmt.Sprintf("Could not create a version of app '%s': %s\n\n"+
					"Apple's API cannot create an app record — the app must already exist in App Store "+
					"Connect.", appID, errMsg),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Creating App Store Version",
				fmt.Sprintf("Could not create version '%s' of app '%s': %s", versionString, appID, errMsg),
			)
		}
		return
	}

	applyAppStoreVersion(&plan, version)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *appStoreVersionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading app store version resource")

	var state appStoreVersionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	versionID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "version_id", versionID)

	version, err := r.client.GetAppStoreVersion(versionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "App store version not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading App Store Version",
			fmt.Sprintf("Could not read app store version '%s': %s", versionID, err.Error()),
		)
		return
	}

	applyAppStoreVersion(&state, version)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the version in place.
func (r *appStoreVersionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating app store version resource")

	var plan appStoreVersionModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state appStoreVersionModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	versionID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "version_id", versionID)

	version, err := r.client.UpdateAppStoreVersion(versionID, models.AppStoreVersionUpdateAttributes{
		VersionString:       stringOrNil(plan.VersionString),
		Copyright:           stringOrNil(plan.Copyright),
		ReviewType:          stringOrNil(plan.ReviewType),
		ReleaseType:         stringOrNil(plan.ReleaseType),
		EarliestReleaseDate: stringOrNil(plan.EarliestReleaseDate),
		UsesIdfa:            boolOrNil(plan.UsesIdfa),
	}, nil)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "STATE_ERROR") || strings.Contains(errMsg, "not allowed") {
			resp.Diagnostics.AddError(
				"App Store Version Not Editable",
				fmt.Sprintf("Apple refused the change to version '%s': %s\n\n"+
					"A version that is in review or already released is frozen. Create the next version "+
					"instead of editing this one.", state.VersionString.ValueString(), errMsg),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error Updating App Store Version",
				fmt.Sprintf("Could not update app store version '%s': %s", versionID, errMsg),
			)
		}
		return
	}

	applyAppStoreVersion(&plan, version)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete removes the version.
//
// Only an editable version can be deleted. One that has been released or is in
// review is part of the app's history, and erroring there would fail the whole
// destroy over a record Apple was never going to remove.
func (r *appStoreVersionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appStoreVersionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	versionID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "version_id", versionID)

	if err := r.client.DeleteAppStoreVersion(versionID, nil); err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			tflog.Info(ctx, "App store version already gone")
		case strings.Contains(errMsg, "STATE_ERROR") ||
			strings.Contains(errMsg, "not allowed") ||
			strings.Contains(errMsg, "cannot be deleted"):
			resp.Diagnostics.AddWarning(
				"App Store Version Not Deleted",
				fmt.Sprintf("Apple refused to delete version '%s' of app '%s': %s\n\n"+
					"Only a version still being prepared can be deleted; one in review or already "+
					"released is part of the app's history. Terraform has removed the resource from "+
					"state only.", state.VersionString.ValueString(), state.AppID.ValueString(), errMsg),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Deleting App Store Version",
				fmt.Sprintf("Could not delete app store version '%s': %s", versionID, errMsg),
			)
			return
		}
	}

	tflog.Info(ctx, "App store version deleted")
}

// Configure adds the provider configured client to the resource.
func (r *appStoreVersionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports a version.
//
// Two forms are accepted: a bare Apple version ID, and the composite
// "<app_id>/<platform>/<version_string>" -- which is what a person has to hand,
// and which needs all three parts because the same version string exists once
// per platform.
func (r *appStoreVersionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing app store version resource", map[string]interface{}{"import_id": req.ID})

	var (
		version *models.AppStoreVersion
		err     error
	)

	parts := strings.Split(req.ID, "/")
	switch len(parts) {
	case 1:
		version, err = r.client.GetAppStoreVersion(req.ID)
	case 3:
		version, err = r.client.GetAppStoreVersionByString(parts[0], parts[1], parts[2])
	default:
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Could not parse import ID '%s'. Accepted forms are a bare Apple version ID and "+
				"'<app_id>/<platform>/<version_string>' — all three parts, because the same version "+
				"string exists once per platform.", req.ID),
		)
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing App Store Version",
			fmt.Sprintf("Could not read app store version '%s': %s", req.ID, err.Error()),
		)
		return
	}

	state := appStoreVersionModel{}
	applyAppStoreVersion(&state, version)

	if state.AppID.IsNull() {
		resp.Diagnostics.AddError(
			"Incomplete App Store Version Import",
			fmt.Sprintf("Apple did not report which app version '%s' belongs to, so app_id cannot be "+
				"written. The client requests include=app, so this is unexpected — please report it.",
				req.ID),
		)
		return
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// applyAppStoreVersion copies Apple's view of the record into the model.
//
// earliest_release_date is deliberately not refreshed: Apple returns it
// normalized to UTC whatever offset was sent, so adopting it would rewrite a
// configured "2026-03-01T08:00:00-07:00" as "2026-03-01T15:00:00Z" and show a
// diff on every plan for a value that never changed.
func applyAppStoreVersion(model *appStoreVersionModel, version *models.AppStoreVersion) {
	model.ID = types.StringValue(version.ID)
	model.Platform = types.StringValue(version.Attributes.Platform)
	model.VersionString = types.StringValue(version.Attributes.VersionString)
	model.Copyright = stringOrNull(version.Attributes.Copyright)
	model.ReleaseType = stringOrNull(version.Attributes.ReleaseType)
	model.ReviewType = stringOrNull(version.Attributes.ReviewType)
	model.AppVersionState = stringOrNull(version.Attributes.AppVersionState)
	model.Downloadable = boolOrNull(version.Attributes.Downloadable)
	model.CreatedDate = stringOrNull(version.Attributes.CreatedDate)

	if version.Attributes.UsesIdfa != nil {
		model.UsesIdfa = types.BoolValue(*version.Attributes.UsesIdfa)
	}

	if version.Relationships != nil {
		if appID := relationshipID(version.Relationships.App); !appID.IsNull() {
			model.AppID = appID
		}
	}
}
