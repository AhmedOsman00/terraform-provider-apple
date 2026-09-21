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
	_ resource.Resource                = &betaBuildLocalizationResource{}
	_ resource.ResourceWithConfigure   = &betaBuildLocalizationResource{}
	_ resource.ResourceWithImportState = &betaBuildLocalizationResource{}
)

func NewBetaBuildLocalizationResource() resource.Resource {
	return &betaBuildLocalizationResource{}
}

type betaBuildLocalizationResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *betaBuildLocalizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_beta_build_localization"
}

// Schema defines the schema for the resource.
func (r *betaBuildLocalizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages one build's **What to Test** note in one language — the text " +
			"TestFlight shows a tester beside the Install button.\n\n" +
			"This is the half of TestFlight metadata that is **replaced with every upload**: the note " +
			"hangs off a build, so each new build starts without one. What survives across builds — the " +
			"app's TestFlight description, feedback address and links — is " +
			"`apple_beta_app_localization`.\n\n" +
			"The build is named the way a pipeline already names it, by `build_number` and " +
			"`pre_release_version` rather than by Apple's opaque build ID, exactly as " +
			"`apple_app_store_version` does. The provider resolves the ID itself.\n\n" +
			"~> This provider does not upload builds. The binary must already be in App Store Connect, " +
			"put there by Xcode, Transporter or fastlane.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the localization.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app the build belongs to. Read it from the " +
					"`apple_apps` data source. Cannot be changed.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"build_number": schema.StringAttribute{
				MarkdownDescription: "The build number of the uploaded build — `CFBundleVersion`, which a " +
					"pipeline knows as `CURRENT_PROJECT_VERSION`. Dot-separated numbers, such as `42` or " +
					"`1.2.3`. The note belongs to one build, so naming a different one replaces it.",
				Required:   true,
				Validators: GetBuildNumberValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pre_release_version": schema.StringAttribute{
				MarkdownDescription: "The marketing version of the build train — " +
					"`CFBundleShortVersionString`, which a pipeline knows as `MARKETING_VERSION`. Required " +
					"here, unlike on `apple_app_store_version`, because there is no version string to " +
					"default it to.",
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
			"locale": schema.StringAttribute{
				MarkdownDescription: "The language this note is in, as an App Store locale code — " +
					"`en-US`, `es-MX`, `ar-SA`. It identifies the record and is absent from Apple's " +
					"update request, so changing it replaces the localization.",
				Required:   true,
				Validators: []validator.String{LocaleValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"whats_new": schema.StringAttribute{
				MarkdownDescription: "What changed in this build and what testers should look at. At most " +
					"4000 characters, counted in characters rather than bytes.",
				Optional:   true,
				Validators: GetWhatsNewValidator(),
			},
		},
	}
}

// Create adopts an existing note or creates one.
//
// Apple writes a note for the app's primary locale when a build finishes
// processing, so a plain POST would 409 on exactly the locale a configuration
// is most likely to start with.
func (r *betaBuildLocalizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating beta build localization resource")

	var plan betaBuildLocalizationModel
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
// app_id, build_number, pre_release_version and platform are not refreshed:
// they are the coordinates that name the build rather than anything Apple
// reports on the localization, and Apple reports the train as a relationship on
// the build rather than a value, so reading it back would cost a request to say
// what the configuration already says.
func (r *betaBuildLocalizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading beta build localization resource")

	var state betaBuildLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	localization, err := r.client.GetBetaBuildLocalization(localizationID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Beta build localization not found, removing from state")
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Beta Build Localization",
			fmt.Sprintf("Could not read beta build localization '%s': %s", localizationID, err.Error()),
		)

		return
	}

	applyBetaBuildLocalization(&state, localization)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update patches the note in place.
func (r *betaBuildLocalizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating beta build localization resource")

	var plan betaBuildLocalizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state betaBuildLocalizationModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	localization, err := r.client.UpdateBetaBuildLocalization(localizationID, stringOrNil(plan.WhatsNew), nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Beta Build Localization",
			fmt.Sprintf("Could not update the '%s' note on build '%s': %s",
				plan.Locale.ValueString(), plan.BuildNumber.ValueString(), err.Error()),
		)

		return
	}

	plan.BuildID = state.BuildID
	applyBetaBuildLocalization(&plan, localization)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete removes the note.
func (r *betaBuildLocalizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state betaBuildLocalizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "localization_id", localizationID)

	if err := r.client.DeleteBetaBuildLocalization(localizationID, nil); err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			tflog.Info(ctx, "Beta build localization already gone")
		case strings.Contains(errMsg, "last localization") || strings.Contains(errMsg, "at least one"):
			resp.Diagnostics.AddWarning(
				"Beta Build Localization Not Deleted",
				fmt.Sprintf("Apple refused to delete the '%s' note on build '%s': %s\n\n"+
					"A build must keep at least one. Terraform has removed the resource from state only; "+
					"the record goes away with the build.",
					state.Locale.ValueString(), state.BuildNumber.ValueString(), errMsg),
			)
		default:
			resp.Diagnostics.AddError(
				"Error Deleting Beta Build Localization",
				fmt.Sprintf("Could not delete beta build localization '%s': %s", localizationID, errMsg),
			)

			return
		}
	}

	tflog.Info(ctx, "Beta build localization deleted")
}

// Configure adds the provider configured client to the resource.
func (r *betaBuildLocalizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports a build's note.
//
// The import ID is composite and there is no bare-ID form: the record names its
// build by Apple's opaque ID, and app_id, build_number and pre_release_version
// cannot be recovered from that without two further requests -- while a person
// importing already has all three.
func (r *betaBuildLocalizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing beta build localization resource", map[string]interface{}{"import_id": req.ID})

	var (
		appID, platform, train, buildNumber, locale string
	)

	parts := strings.Split(req.ID, "/")
	switch len(parts) {
	case 4:
		appID, train, buildNumber, locale = parts[0], parts[1], parts[2], parts[3]
	case 5:
		appID, platform, train, buildNumber, locale = parts[0], parts[1], parts[2], parts[3], parts[4]
	default:
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Could not parse import ID '%s'. Accepted forms are "+
				"'<app_id>/<pre_release_version>/<build_number>/<locale>' and "+
				"'<app_id>/<platform>/<pre_release_version>/<build_number>/<locale>'.\n\n"+
				"A bare localization ID is not accepted: it names the build by Apple's opaque ID, from "+
				"which the build number and its train cannot be recovered without further requests.", req.ID),
		)

		return
	}

	build, err := r.client.GetBuildByNumber(appID, platform, train, buildNumber)
	if err != nil {
		resp.Diagnostics.AddError(
			"Build Not Found",
			fmt.Sprintf("Could not find build '%s' of version '%s' for app '%s': %s", buildNumber, train, appID, err.Error()),
		)

		return
	}

	localization, err := r.client.GetBetaBuildLocalizationByLocale(build.ID, locale)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Beta Build Localization",
			fmt.Sprintf("Could not read the '%s' note on build '%s': %s", locale, buildNumber, err.Error()),
		)

		return
	}

	state := betaBuildLocalizationModel{
		AppID:             types.StringValue(appID),
		BuildNumber:       types.StringValue(buildNumber),
		PreReleaseVersion: types.StringValue(train),
		Platform:          types.StringNull(),
	}
	if platform != "" {
		state.Platform = types.StringValue(platform)
	}

	applyBetaBuildLocalization(&state, localization)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write resolves the build and then adopts or creates the note on it.
func (r *betaBuildLocalizationResource) write(ctx context.Context, plan *betaBuildLocalizationModel, diags *diag.Diagnostics) {
	var (
		appID       = plan.AppID.ValueString()
		platform    = plan.Platform.ValueString()
		train       = plan.PreReleaseVersion.ValueString()
		buildNumber = plan.BuildNumber.ValueString()
		locale      = plan.Locale.ValueString()
	)

	ctx = tflog.SetField(ctx, "app_id", appID)
	ctx = tflog.SetField(ctx, "build_number", buildNumber)
	ctx = tflog.SetField(ctx, "pre_release_version", train)
	ctx = tflog.SetField(ctx, "locale", locale)

	build, err := r.client.GetBuildByNumber(appID, platform, train, buildNumber)
	if err != nil {
		diags.AddError(
			"Build Not Found",
			fmt.Sprintf("Could not find build '%s' of version '%s' for app '%s': %s\n\n"+
				"This provider does not upload builds — the binary must already be in App Store Connect, "+
				"put there by Xcode, Transporter or fastlane.%s",
				buildNumber, train, appID, err.Error(), r.describeAvailableBuilds(appID, platform, train)),
		)

		return
	}

	plan.BuildID = types.StringValue(build.ID)

	var localization *models.BetaBuildLocalization

	existing, readErr := r.client.GetBetaBuildLocalizationByLocale(build.ID, locale)
	switch {
	case readErr == nil:
		tflog.Debug(ctx, "Patching existing beta build localization", map[string]interface{}{"localization_id": existing.ID})
		localization, err = r.client.UpdateBetaBuildLocalization(existing.ID, stringOrNil(plan.WhatsNew), nil)
	case strings.Contains(readErr.Error(), "not found") || strings.Contains(readErr.Error(), "404"):
		tflog.Debug(ctx, "Creating beta build localization")
		localization, err = r.client.CreateBetaBuildLocalization(build.ID, models.BetaBuildLocalizationCreateAttributes{
			Locale:   locale,
			WhatsNew: stringOrNil(plan.WhatsNew),
		}, nil)
	default:
		diags.AddError(
			"Error Reading Beta Build Localizations",
			fmt.Sprintf("Could not check whether build '%s' already has a '%s' note: %s",
				buildNumber, locale, readErr.Error()),
		)

		return
	}

	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "locale") && strings.Contains(errMsg, "INVALID") {
			diags.AddError(
				"Locale Not Available",
				fmt.Sprintf("Apple refused locale '%s' on build '%s': %s\n\n"+
					"TestFlight accepts the App Store locale codes — en-US, es-MX, ar-SA.",
					locale, buildNumber, errMsg),
			)
		} else {
			diags.AddError(
				"Error Writing Beta Build Localization",
				fmt.Sprintf("Could not write the '%s' note on build '%s': %s", locale, buildNumber, errMsg),
			)
		}
		tflog.Error(ctx, "Failed to write beta build localization", map[string]interface{}{"error": errMsg})

		return
	}

	applyBetaBuildLocalization(plan, localization)
}

// describeAvailableBuilds names the builds Apple does hold for the train, so a
// failed lookup reports the mismatch rather than only the miss.
//
// Best-effort: a listing that fails adds nothing to a diagnostic that is
// already being raised about the build that was asked for.
func (r *betaBuildLocalizationResource) describeAvailableBuilds(appID, platform, train string) string {
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

// applyBetaBuildLocalization copies Apple's view of the record into the model.
//
// The build coordinates are left alone -- see Read.
func applyBetaBuildLocalization(model *betaBuildLocalizationModel, localization *models.BetaBuildLocalization) {
	model.ID = types.StringValue(localization.ID)

	if localization.Attributes.Locale != "" {
		model.Locale = types.StringValue(localization.Attributes.Locale)
	}

	model.WhatsNew = stringOrNull(localization.Attributes.WhatsNew)

	if localization.Relationships != nil {
		if buildID := relationshipID(localization.Relationships.Build); !buildID.IsNull() {
			model.BuildID = buildID
		}
	}
}
