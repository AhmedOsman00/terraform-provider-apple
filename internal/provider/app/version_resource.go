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
	_ resource.Resource                   = &appStoreVersionResource{}
	_ resource.ResourceWithConfigure      = &appStoreVersionResource{}
	_ resource.ResourceWithImportState    = &appStoreVersionResource{}
	_ resource.ResourceWithModifyPlan     = &appStoreVersionResource{}
	_ resource.ResourceWithValidateConfig = &appStoreVersionResource{}
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
			"Set `build_number` to attach an uploaded build. The provider names the build the way a " +
			"build pipeline does — `CURRENT_PROJECT_VERSION` and `MARKETING_VERSION` — and resolves " +
			"Apple's opaque build ID itself; uploading the binary is still Xcode's, Transporter's or " +
			"fastlane's job.\n\n" +
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
			"build_number": schema.StringAttribute{
				MarkdownDescription: "The build number of the uploaded build to attach — `CFBundleVersion`, " +
					"which a pipeline knows as `CURRENT_PROJECT_VERSION`. Dot-separated numbers, such as " +
					"`42` or `1.2.3`.\n\n" +
					"This provider does not upload builds; the binary must already be in App Store Connect, " +
					"put there by Xcode, Transporter or fastlane. Apple rejects a build that is still " +
					"processing, which takes five to thirty minutes after the upload finishes — the " +
					"provider reports that state rather than waiting for it, so a pipeline that applies " +
					"immediately after uploading should wait before it does.\n\n" +
					"Leaving this unset leaves whatever build is attached alone, so a build attached by " +
					"hand or by a separate pipeline is not torn off by the next apply. Removing it from a " +
					"configuration that had it detaches the build.",
				Optional:   true,
				Validators: GetBuildNumberValidator(),
			},
			"pre_release_version": schema.StringAttribute{
				MarkdownDescription: "The marketing version of the build train to take `build_number` from " +
					"— `CFBundleShortVersionString`, which a pipeline knows as `MARKETING_VERSION`. " +
					"Defaults to `version_string`, and is meant to be left unset: App Store Connect only " +
					"offers a version the builds whose train matches it, so a train that differs from " +
					"`version_string` is likely to be refused by Apple rather than resolved. It exists as " +
					"an escape hatch, and does nothing without `build_number`.",
				Optional:   true,
				Validators: GetVersionStringValidator(),
			},
			"build_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the attached build, resolved from `build_number` and " +
					"`pre_release_version`. Null when no build is attached.",
				Computed: true,
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

	var plan appStoreVersionResourceModel
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

	applyAppStoreVersion(&plan.appStoreVersionModel, version)
	plan.BuildID = types.StringNull()

	// The build is a second call -- Apple's create request carries only the app
	// relationship -- and the version exists whether or not it succeeds. State
	// is written before the attachment is reported, because a version left out
	// of state is a version the next apply cannot see and Apple answers with a
	// 409 about the one already being prepared.
	buildDiags := r.attachBuild(ctx, &plan)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(buildDiags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *appStoreVersionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading app store version resource")

	var state appStoreVersionResourceModel
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

	applyAppStoreVersion(&state.appStoreVersionModel, version)
	r.readBuild(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// readBuild refreshes the attached build, and only when one was configured.
//
// A null build_number means the configuration said nothing about the build, and
// adopting whatever Apple has would put a build attached by hand or by a
// separate pipeline into state -- which the next plan would then detach. This
// is the same rule apple_app_settings follows for its optional URLs and this
// resource already follows for earliest_release_date: an attribute the
// configuration does not speak to is not refreshed.
//
// pre_release_version is never refreshed. Apple reports the train as a
// relationship rather than a value on the build, and reading it back would cost
// a second request to say what version_string already says.
func (r *appStoreVersionResource) readBuild(ctx context.Context, state *appStoreVersionResourceModel, diags *diag.Diagnostics) {
	if state.BuildNumber.IsNull() {
		return
	}

	build, err := r.client.GetAppStoreVersionBuild(state.ID.ValueString())
	if err != nil {
		diags.AddError(
			"Error Reading Attached Build",
			fmt.Sprintf("Could not read the build attached to version '%s': %s",
				state.ID.ValueString(), err.Error()),
		)
		return
	}

	if build == nil {
		tflog.Info(ctx, "App store version has no build attached")
		state.BuildNumber = types.StringNull()
		state.BuildID = types.StringNull()

		return
	}

	state.BuildNumber = stringOrNull(build.Attributes.Version)
	state.BuildID = types.StringValue(build.ID)
}

// Update updates the version in place.
func (r *appStoreVersionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating app store version resource")

	var plan appStoreVersionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state appStoreVersionResourceModel
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

	applyAppStoreVersion(&plan.appStoreVersionModel, version)
	plan.BuildID = state.BuildID

	buildDiags := r.reconcileBuild(ctx, &plan, &state)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(buildDiags...)
}

// reconcileBuild brings the attached build in line with the plan.
//
// Three cases: a build named where none was, a different build named, and a
// build_number removed from a configuration that had one -- which is the only
// thing that detaches, and is why the linkage uses a nullable relationship.
// Anything else carries the ID in state forward untouched, because Terraform
// proposed that value for the computed attribute and producing a different one
// is an inconsistent result.
func (r *appStoreVersionResource) reconcileBuild(ctx context.Context, plan, state *appStoreVersionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if plan.BuildNumber.IsNull() {
		if state.BuildNumber.IsNull() {
			return diags
		}

		tflog.Info(ctx, "Detaching build from app store version")

		if err := r.client.UpdateAppStoreVersionBuild(plan.ID.ValueString(), nil, nil); err != nil {
			diags.AddError(
				"Error Detaching Build",
				fmt.Sprintf("Could not detach build '%s' from version '%s': %s",
					state.BuildNumber.ValueString(), plan.VersionString.ValueString(), err.Error()),
			)

			return diags
		}

		plan.BuildID = types.StringNull()

		return diags
	}

	unchanged := plan.BuildNumber.Equal(state.BuildNumber) &&
		plan.trainVersion() == state.trainVersion() &&
		!state.BuildID.IsNull()
	if unchanged {
		return diags
	}

	return r.attachBuild(ctx, plan)
}

// attachBuild resolves the configured build number to Apple's ID and links it
// to the version.
//
// The resolution is unavoidable however the resource is modelled: the linkage
// endpoint takes an opaque build ID, and nobody has one. Apple does filter this
// one server-side -- filter[version] is the build number and
// filter[preReleaseVersion.version] its train -- so it is one request rather
// than the full-collection scan every other by-identifier lookup in this
// provider is forced into.
func (r *appStoreVersionResource) attachBuild(ctx context.Context, plan *appStoreVersionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if plan.BuildNumber.IsNull() || plan.BuildNumber.IsUnknown() {
		plan.BuildID = types.StringNull()

		return diags
	}

	var (
		appID       = plan.AppID.ValueString()
		platform    = plan.Platform.ValueString()
		train       = plan.trainVersion()
		buildNumber = plan.BuildNumber.ValueString()
	)

	ctx = tflog.SetField(ctx, "build_number", buildNumber)
	ctx = tflog.SetField(ctx, "pre_release_version", train)
	tflog.Info(ctx, "Attaching build to app store version")

	build, err := r.client.GetBuildByNumber(appID, platform, train, buildNumber)
	if err != nil {
		diags.AddError(
			"Build Not Found",
			fmt.Sprintf("Could not find build '%s' of version '%s' for app '%s' on %s: %s\n\n"+
				"This provider does not upload builds — the binary must already be in App Store Connect, "+
				"put there by Xcode, Transporter or fastlane.%s",
				buildNumber, train, appID, platform, err.Error(), r.describeAvailableBuilds(appID, platform, train)),
		)

		return diags
	}

	// A build Apple is still processing exists but cannot be attached, and the
	// wait is five to thirty minutes -- far longer than an apply should block
	// for. Naming the state is more useful than either waiting or reporting it
	// as a missing build.
	if processing := build.Attributes.ProcessingState; processing != nil && *processing != models.BuildProcessingStateValid {
		diags.AddError(
			"Build Not Ready",
			fmt.Sprintf("Build '%s' of version '%s' is %s, and Apple accepts only a VALID build.\n\n"+
				"Processing takes five to thirty minutes after the upload finishes. A pipeline that "+
				"uploads and applies in one run should wait for processing to complete before it applies; "+
				"this provider reports the state rather than blocking the apply on it.",
				buildNumber, train, *processing),
		)

		return diags
	}

	if err := r.client.UpdateAppStoreVersionBuild(plan.ID.ValueString(), &build.ID, nil); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "STATE_ERROR") || strings.Contains(errMsg, "not allowed") {
			diags.AddError(
				"App Store Version Not Editable",
				fmt.Sprintf("Apple refused to attach build '%s' to version '%s': %s\n\n"+
					"A version that is in review or already released is frozen. Create the next version "+
					"instead of changing this one's build.",
					buildNumber, plan.VersionString.ValueString(), errMsg),
			)
		} else {
			diags.AddError(
				"Error Attaching Build",
				fmt.Sprintf("Could not attach build '%s' of version '%s' to version '%s': %s",
					buildNumber, train, plan.ID.ValueString(), errMsg),
			)
		}

		return diags
	}

	plan.BuildID = types.StringValue(build.ID)

	return diags
}

// describeAvailableBuilds names the builds Apple does hold for the train, so a
// failed lookup reports the mismatch rather than only the miss.
//
// It is best-effort: a listing that fails adds nothing to a diagnostic that is
// already being raised about the build that was asked for.
func (r *appStoreVersionResource) describeAvailableBuilds(appID, platform, train string) string {
	builds, err := r.client.GetBuilds(appID, platform, train, "")
	if err != nil || len(builds) == 0 {
		return ""
	}

	numbers := make([]string, 0, len(builds))
	for i := range builds {
		if version := builds[i].Attributes.Version; version != nil {
			numbers = append(numbers, *version)
		}
	}

	if len(numbers) == 0 {
		return ""
	}

	return fmt.Sprintf("\n\nApple holds these build numbers for version '%s': %s.",
		train, strings.Join(numbers, ", "))
}

// Delete removes the version.
//
// Only an editable version can be deleted. One that has been released or is in
// review is part of the app's history, and erroring there would fail the whole
// destroy over a record Apple was never going to remove.
func (r *appStoreVersionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appStoreVersionResourceModel
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

// ValidateConfig rejects a train named without a build to take from it.
//
// pre_release_version only narrows the build lookup, so on its own it selects
// nothing and would be silently ignored -- which reads, in a configuration, as
// a build that was requested and never attached.
func (r *appStoreVersionResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config appStoreVersionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.PreReleaseVersion.IsNull() && config.BuildNumber.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("pre_release_version"),
			"Pre-Release Version Without Build Number",
			"pre_release_version names the build train to take build_number from, so it does nothing "+
				"on its own. Set build_number as well, or remove pre_release_version — it defaults to "+
				"version_string, which is what App Store Connect offers builds from anyway.",
		)
	}
}

// ModifyPlan marks build_id unknown when the build being asked for changes.
//
// Terraform proposes the prior state value for a computed attribute, so without
// this a changed build_number would plan to keep the old build's ID and the
// apply would be rejected as an inconsistent result -- the same trap
// apple_app_price_schedule carries a ModifyPlan for. version_string counts as a
// change because pre_release_version defaults to it.
func (r *appStoreVersionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Create and destroy need nothing: the ID is already unknown on one and
	// irrelevant on the other.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var plan, state appStoreVersionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.BuildNumber.Equal(state.BuildNumber) &&
		plan.PreReleaseVersion.Equal(state.PreReleaseVersion) &&
		plan.VersionString.Equal(state.VersionString) {
		return
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("build_id"), types.StringUnknown())...)
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

	state := appStoreVersionResourceModel{}
	applyAppStoreVersion(&state.appStoreVersionModel, version)

	// An import has no configuration to contradict, so the attached build is
	// adopted rather than left null -- the "do not refresh what the
	// configuration did not speak to" rule protects a configured value, and
	// here there is none. pre_release_version stays null and defaults to
	// version_string, which is the train Apple took the build from.
	state.BuildNumber = types.StringNull()
	state.BuildID = types.StringNull()

	if build, buildErr := r.client.GetAppStoreVersionBuild(version.ID); buildErr != nil {
		resp.Diagnostics.AddWarning(
			"Attached Build Not Read",
			fmt.Sprintf("Version '%s' was imported, but the build attached to it could not be read: %s\n\n"+
				"build_number and build_id are null in state. The next plan will attach whatever the "+
				"configuration names.", version.ID, buildErr.Error()),
		)
	} else if build != nil {
		state.BuildNumber = stringOrNull(build.Attributes.Version)
		state.BuildID = types.StringValue(build.ID)
	}

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
