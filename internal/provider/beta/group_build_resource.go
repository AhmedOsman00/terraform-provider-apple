// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package beta

import (
	"context"
	"fmt"
	"slices"
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
	_ resource.Resource                = &betaGroupBuildResource{}
	_ resource.ResourceWithConfigure   = &betaGroupBuildResource{}
	_ resource.ResourceWithImportState = &betaGroupBuildResource{}
)

func NewBetaGroupBuildResource() resource.Resource {
	return &betaGroupBuildResource{}
}

type betaGroupBuildResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *betaGroupBuildResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_beta_group_build"
}

// Schema defines the schema for the resource.
func (r *betaGroupBuildResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gives one TestFlight group one build — the distribution half of " +
			"`fastlane pilot distribute`, and what a group without `has_access_to_all_builds` needs " +
			"before its testers can install anything.\n\n" +
			"The build is named the way a pipeline already names it, by `build_number` and " +
			"`pre_release_version` rather than by Apple's opaque build ID, exactly as " +
			"`apple_beta_build_localization` does. The group's app is read from the group, so no " +
			"`app_id` is needed.\n\n" +
			"Like `apple_beta_tester`, this is a **linkage, not a record**: destroying it takes the " +
			"build away from this group's testers and leaves the build, and every other group it was " +
			"given to, alone.\n\n" +
			"Apple refuses a build that has not finished processing; wait for `VALID` first, with " +
			"`apple build wait`. A group with `has_access_to_all_builds` already receives every build " +
			"and is refused here at apply.\n\n" +
			"~> For an **external** group, assigning a build is not the same as testers seeing it: " +
			"Apple's beta review must approve the build first, and submitting it is not modelled — " +
			"it is an event rather than state. Submit from App Store Connect.\n\n" +
			"~> Terraform does not upload builds. The binary must already be in App Store Connect, " +
			"put there by `apple build upload`, Xcode or Transporter.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`<group_id>/<build_id>`. Apple has no identifier for an " +
					"assignment, so this is composed from the two it links.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the `apple_beta_group` to give the build to. " +
					"Giving it to a different group is a different assignment and replaces the resource.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"build_number": schema.StringAttribute{
				MarkdownDescription: "The build number of the uploaded build — `CFBundleVersion`, which a " +
					"pipeline knows as `CURRENT_PROJECT_VERSION`. Dot-separated numbers, such as `42` or " +
					"`1.2.3`. Naming a different build replaces the assignment: the old build comes off " +
					"the group and the new one goes on.",
				Required:   true,
				Validators: GetBuildNumberValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pre_release_version": schema.StringAttribute{
				MarkdownDescription: "The marketing version of the build train — " +
					"`CFBundleShortVersionString`, which a pipeline knows as `MARKETING_VERSION`.",
				Required:   true,
				Validators: GetVersionStringValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "Which platform's build to take: `IOS`, `MAC_OS`, `TV_OS` or " +
					"`VISION_OS`. Only needed when the same train and build number exist on more than one " +
					"platform; left unset, the lookup is not narrowed.",
				Optional:   true,
				Validators: []validator.String{BuildPlatformValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"build_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the build, resolved from `build_number` and " +
					"`pre_release_version`.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create resolves the build through the group's app and assigns it.
//
// A build already assigned is an error rather than an adoption, for the reason
// apple_beta_tester gives: destroying this resource would then take away a
// build a configuration never gave.
func (r *betaGroupBuildResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating beta group build resource")

	var plan betaGroupBuildModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var (
		groupID     = plan.GroupID.ValueString()
		platform    = plan.Platform.ValueString()
		train       = plan.PreReleaseVersion.ValueString()
		buildNumber = plan.BuildNumber.ValueString()
	)
	ctx = tflog.SetField(ctx, "beta_group_id", groupID)
	ctx = tflog.SetField(ctx, "build_number", buildNumber)
	ctx = tflog.SetField(ctx, "pre_release_version", train)

	group, err := r.client.GetBetaGroup(groupID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Beta Group",
			fmt.Sprintf("Could not read beta group '%s' to find its app: %s", groupID, err.Error()),
		)

		return
	}
	if group.Relationships == nil || group.Relationships.App == nil || group.Relationships.App.Data.ID == "" {
		resp.Diagnostics.AddError(
			"Error Reading Beta Group",
			fmt.Sprintf("Apple did not report which app beta group '%s' belongs to.", groupID),
		)

		return
	}
	if all := group.Attributes.HasAccessToAllBuilds; all != nil && *all {
		resp.Diagnostics.AddError(
			"Beta Group Already Receives Every Build",
			fmt.Sprintf("Beta group '%s' has access to all builds, so there is nothing to assign: its "+
				"testers already see build '%s' once it finishes processing.\n\n"+
				"Remove this resource, or recreate the group with has_access_to_all_builds = false to "+
				"choose its builds one at a time.", groupID, buildNumber),
		)

		return
	}
	appID := group.Relationships.App.Data.ID

	build, err := r.client.GetBuildByNumber(appID, platform, train, buildNumber)
	if err != nil {
		resp.Diagnostics.AddError(
			"Build Not Found",
			fmt.Sprintf("Could not find build '%s' of version '%s' for app '%s': %s\n\n"+
				"Terraform does not upload builds — the binary must already be in App Store Connect, "+
				"put there by `apple build upload`, Xcode or Transporter.%s",
				buildNumber, train, appID, err.Error(), describeAvailableBuilds(r.client, appID, platform, train)),
		)

		return
	}

	assigned, err := r.client.GetBetaGroupBuildIDs(groupID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Beta Group Builds",
			fmt.Sprintf("Could not check whether beta group '%s' already has build '%s': %s", groupID, buildNumber, err.Error()),
		)

		return
	}
	if slices.Contains(assigned, build.ID) {
		resp.Diagnostics.AddError(
			"Build Already In Beta Group",
			fmt.Sprintf("Build '%s' of version '%s' is already assigned to beta group '%s'.\n\n"+
				"Import the assignment instead of creating it:\n\n"+
				"  terraform import <address> %s/%s/%s", buildNumber, train, groupID, groupID, train, buildNumber),
		)

		return
	}

	tflog.Debug(ctx, "Assigning build to beta group", map[string]interface{}{"build_id": build.ID})

	if err := r.client.AddBuildToBetaGroup(groupID, build.ID, nil); err != nil {
		addAssignError(&resp.Diagnostics, groupID, buildNumber, train, build, err)
		tflog.Error(ctx, "Failed to assign build to beta group", map[string]interface{}{"error": err.Error()})

		return
	}

	applyBetaGroupBuild(&plan, build.ID)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read checks that the build is still assigned.
//
// The group's linkage is what is asked, because the resource is the
// assignment: a build taken off the group by hand has to read as gone. The
// build coordinates are not refreshed -- they are inputs, and Apple reports the
// train as a relationship on the build rather than a value.
func (r *betaGroupBuildResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading beta group build resource")

	var state betaGroupBuildModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := state.GroupID.ValueString()
	buildID := state.BuildID.ValueString()
	ctx = tflog.SetField(ctx, "beta_group_id", groupID)
	ctx = tflog.SetField(ctx, "build_id", buildID)

	assigned, err := r.client.GetBetaGroupBuildIDs(groupID)
	if err != nil {
		if isNotFound(err) {
			tflog.Info(ctx, "Beta group no longer exists, removing from state")
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Beta Group Builds",
			fmt.Sprintf("Could not read the builds of beta group '%s': %s", groupID, err.Error()),
		)

		return
	}

	if !slices.Contains(assigned, buildID) {
		tflog.Info(ctx, "Build no longer assigned to the group, removing from state")
		resp.State.RemoveResource(ctx)

		return
	}

	applyBetaGroupBuild(&state, buildID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update is unreachable: every configurable attribute forces replacement,
// because an assignment is a linkage with nothing on it to change.
func (r *betaGroupBuildResource) Update(ctx context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Error(ctx, "Update called on a Beta Group Build, which has nothing to update")

	resp.Diagnostics.AddError(
		"Beta Group Build Update Not Supported",
		"A build assignment has nothing to update in place, so this resource replaces it instead. "+
			"Reaching this method means an attribute changed without forcing replacement, which is a "+
			"bug in the provider: please report it.",
	)
}

// Delete takes the build away from the group. The build stays in App Store
// Connect and in every other group it was given to.
func (r *betaGroupBuildResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state betaGroupBuildModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := state.GroupID.ValueString()
	buildID := state.BuildID.ValueString()
	ctx = tflog.SetField(ctx, "beta_group_id", groupID)
	ctx = tflog.SetField(ctx, "build_id", buildID)

	if err := r.client.RemoveBuildFromBetaGroup(groupID, buildID, nil); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError(
				"Error Removing Build From Beta Group",
				fmt.Sprintf("Could not remove build '%s' from beta group '%s': %s",
					state.BuildNumber.ValueString(), groupID, err.Error()),
			)

			return
		}

		tflog.Info(ctx, "Build or group already gone")
	}

	tflog.Info(ctx, "Build removed from the beta group")
}

// Configure adds the provider configured client to the resource.
func (r *betaGroupBuildResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports one assignment.
//
// The import ID names the build by its numbers, as the configuration does, and
// there is no bare-ID form: a build ID cannot be turned back into its build
// number and train without further requests, and a person importing already
// has both.
func (r *betaGroupBuildResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing beta group build resource", map[string]interface{}{"import_id": req.ID})

	var groupID, platform, train, buildNumber string

	parts := strings.Split(req.ID, "/")
	switch len(parts) {
	case 3:
		groupID, train, buildNumber = parts[0], parts[1], parts[2]
	case 4:
		groupID, platform, train, buildNumber = parts[0], parts[1], parts[2], parts[3]
	default:
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Could not parse import ID '%s'. Accepted forms are "+
				"'<group_id>/<pre_release_version>/<build_number>' and "+
				"'<group_id>/<platform>/<pre_release_version>/<build_number>'.", req.ID),
		)

		return
	}

	group, err := r.client.GetBetaGroup(groupID)
	if err != nil || group.Relationships == nil || group.Relationships.App == nil || group.Relationships.App.Data.ID == "" {
		reason := "Apple did not report the group's app"
		if err != nil {
			reason = err.Error()
		}
		resp.Diagnostics.AddError(
			"Error Importing Beta Group Build",
			fmt.Sprintf("Could not read beta group '%s' to find its app: %s", groupID, reason),
		)

		return
	}
	appID := group.Relationships.App.Data.ID

	build, err := r.client.GetBuildByNumber(appID, platform, train, buildNumber)
	if err != nil {
		resp.Diagnostics.AddError(
			"Build Not Found",
			fmt.Sprintf("Could not find build '%s' of version '%s' for app '%s': %s", buildNumber, train, appID, err.Error()),
		)

		return
	}

	assigned, err := r.client.GetBetaGroupBuildIDs(groupID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Beta Group Build",
			fmt.Sprintf("Could not read the builds of beta group '%s': %s", groupID, err.Error()),
		)

		return
	}
	if !slices.Contains(assigned, build.ID) {
		resp.Diagnostics.AddError(
			"Build Not In Beta Group",
			fmt.Sprintf("Build '%s' of version '%s' is not assigned to beta group '%s', so there is "+
				"nothing to import. Create the resource instead.", buildNumber, train, groupID),
		)

		return
	}

	state := betaGroupBuildModel{
		GroupID:           types.StringValue(groupID),
		BuildNumber:       types.StringValue(buildNumber),
		PreReleaseVersion: types.StringValue(train),
		Platform:          types.StringNull(),
	}
	if platform != "" {
		state.Platform = types.StringValue(platform)
	}

	applyBetaGroupBuild(&state, build.ID)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// addAssignError maps Apple's refusals onto diagnostics that name the cause.
//
// The build's processing state is already in hand from the lookup, so it is
// reported whatever Apple said: an assignment refused for a build that is not
// VALID is almost always refused for that reason.
func addAssignError(diags *diag.Diagnostics, groupID, buildNumber, train string, build *models.Build, err error) {
	errMsg := err.Error()
	lower := strings.ToLower(errMsg)

	state := ""
	if build.Attributes.ProcessingState != nil {
		state = *build.Attributes.ProcessingState
	}

	switch {
	case state != "" && state != models.BuildProcessingStateValid:
		diags.AddError(
			"Build Not Ready",
			fmt.Sprintf("Apple refused to give build '%s' of version '%s' to beta group '%s': %s\n\n"+
				"Apple reports the build as %s, and only a VALID build can be assigned. Wait for it — "+
				"`apple build wait %s %s` — and apply again.", buildNumber, train, groupID, errMsg, state, train, buildNumber),
		)
	case build.Attributes.Expired != nil && *build.Attributes.Expired || strings.Contains(lower, "expired"):
		diags.AddError(
			"Build Expired",
			fmt.Sprintf("Apple refused to give build '%s' of version '%s' to beta group '%s': %s\n\n"+
				"TestFlight builds expire 90 days after upload. Upload a new one.", buildNumber, train, groupID, errMsg),
		)
	case strings.Contains(lower, "export compliance") || strings.Contains(lower, "encryption"):
		diags.AddError(
			"Export Compliance Missing",
			fmt.Sprintf("Apple refused to give build '%s' of version '%s' to beta group '%s': %s\n\n"+
				"The build has no export compliance answer. Set ITSAppUsesNonExemptEncryption in "+
				"Info.plist and upload again, or answer it for this build in App Store Connect.",
				buildNumber, train, groupID, errMsg),
		)
	case isNotFound(err):
		diags.AddError(
			"Beta Group Not Found",
			fmt.Sprintf("Could not give build '%s' to beta group '%s': %s\n\n"+
				"The group must exist before builds can be assigned to it.", buildNumber, groupID, errMsg),
		)
	default:
		diags.AddError(
			"Error Assigning Build To Beta Group",
			fmt.Sprintf("Could not give build '%s' of version '%s' to beta group '%s': %s",
				buildNumber, train, groupID, errMsg),
		)
	}
}

// applyBetaGroupBuild records the resolved build and the composed ID.
func applyBetaGroupBuild(model *betaGroupBuildModel, buildID string) {
	model.BuildID = types.StringValue(buildID)
	model.ID = types.StringValue(model.GroupID.ValueString() + "/" + buildID)
}
