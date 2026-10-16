// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/provider/media"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                = &appScreenshotSetResource{}
	_ resource.ResourceWithConfigure   = &appScreenshotSetResource{}
	_ resource.ResourceWithImportState = &appScreenshotSetResource{}
	_ resource.ResourceWithModifyPlan  = &appScreenshotSetResource{}
)

func NewAppScreenshotSetResource() resource.Resource {
	return &appScreenshotSetResource{}
}

type appScreenshotSetResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *appScreenshotSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_screenshot_set"
}

// Schema defines the schema for the resource.
func (r *appScreenshotSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the product page screenshots for one device size in one language: " +
			"the images under *Previews and Screenshots* on an App Store version.\n\n" +
			"Apple groups screenshots into a set per display type — `APP_IPHONE_67` for the 6.7\" " +
			"iPhone, `APP_IPAD_PRO_3GEN_129` for the 12.9\" iPad and so on — under an " +
			"`apple_app_store_version_localization`. This resource owns one set and keeps its " +
			"contents equal to `screenshots`, in order.\n\n" +
			"Files are tracked by content, not by path. The MD5 of each file is computed at plan time " +
			"and compared with what Apple holds, so:\n\n" +
			"- editing an image uploads it again;\n" +
			"- reordering or renaming files uploads nothing — the existing screenshots are reordered;\n" +
			"- a screenshot added, removed or replaced by hand in App Store Connect shows as drift and " +
			"is put back on the next apply.\n\n" +
			"When the localization already has a set for the display type, it is adopted, and any " +
			"screenshot in it that the configuration does not name is deleted.\n\n" +
			"The file parts are sent to the presigned storage URLs Apple issues for each upload, over a " +
			"separate HTTP client that never carries the App Store Connect API token.\n\n" +
			"Apple processes each image after upload and checks its pixel dimensions against the " +
			"display type. An image it rejects fails the apply with Apple's reason, and the rejected " +
			"upload is removed.\n\n" +
			"~> Screenshots belong to a version. A version that has been submitted accepts no " +
			"changes, and the next `apple_app_store_version` starts with Apple's copy of the previous " +
			"version's screenshots, which this resource then adopts.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the screenshot set.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_store_version_localization_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_app_store_version_localization` the screenshots " +
					"belong to. Cannot be changed.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"screenshot_display_type": schema.StringAttribute{
				MarkdownDescription: "The device size the screenshots are for. Cannot be changed. Apple's " +
					"current values include `APP_IPHONE_67`, `APP_IPHONE_65`, `APP_IPHONE_61`, " +
					"`APP_IPHONE_58`, `APP_IPHONE_55`, `APP_IPHONE_47`, `APP_IPHONE_40`, `APP_IPHONE_35`, " +
					"`APP_IPAD_PRO_3GEN_129`, `APP_IPAD_PRO_3GEN_11`, `APP_IPAD_PRO_129`, `APP_IPAD_105`, " +
					"`APP_IPAD_97`, `APP_DESKTOP`, `APP_APPLE_TV`, `APP_APPLE_VISION_PRO`, " +
					"`APP_WATCH_ULTRA`, `APP_WATCH_SERIES_10`, `APP_WATCH_SERIES_7`, `APP_WATCH_SERIES_4`, " +
					"`APP_WATCH_SERIES_3`, and an `IMESSAGE_APP_` counterpart of each iPhone and iPad " +
					"size. Any value of that shape is accepted, so a size Apple adds later works without " +
					"a provider release; Apple rejects one it does not know.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(ScreenshotDisplayTypePattern,
						"must be an App Store screenshot display type such as APP_IPHONE_67"),
				},
			},
			"screenshots": schema.ListAttribute{
				MarkdownDescription: fmt.Sprintf("Paths to the screenshot images on the machine running "+
					"Terraform, in the order the App Store shows them — PNG or JPEG, sized for the "+
					"display type. Between 1 and %d. Null after an import until the configuration "+
					"supplies it.", MaxScreenshotsPerSet),
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.List{
					listvalidator.SizeBetween(1, MaxScreenshotsPerSet),
				},
			},
			"screenshot_ids": schema.ListAttribute{
				MarkdownDescription: "Apple's IDs for the screenshots, in the same order as `screenshots`.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"screenshot_checksums": schema.ListAttribute{
				MarkdownDescription: "MD5 of each screenshot, lower-case hex, in the same order as " +
					"`screenshots`. Computed from the files at plan time and compared with what Apple " +
					"records. An empty entry is a screenshot Apple failed to process, which the next " +
					"apply replaces.",
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

// ModifyPlan computes each file's checksum, so that a change of content is a
// planned change, and carries the screenshot IDs forward only when the content
// and order match what Apple holds.
func (r *appScreenshotSetResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan appScreenshotSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ScreenshotIDs = types.ListUnknown(types.StringType)

	paths, known := knownStrings(plan.Screenshots)
	if !known {
		plan.ScreenshotChecksums = types.ListUnknown(types.StringType)
		resp.Diagnostics.Append(resp.Plan.Set(ctx, plan)...)
		return
	}

	checksums := make([]string, len(paths))
	for i, p := range paths {
		_, checksum, err := media.ReadAsset(p)
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("screenshots").AtListIndex(i), "Cannot Read Screenshot", err.Error())
			continue
		}
		checksums[i] = checksum
	}
	if resp.Diagnostics.HasError() {
		return
	}

	list, diags := types.ListValueFrom(ctx, types.StringType, checksums)
	resp.Diagnostics.Append(diags...)
	plan.ScreenshotChecksums = list

	if !req.State.Raw.IsNull() {
		var state appScreenshotSetModel
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if state.ScreenshotChecksums.Equal(plan.ScreenshotChecksums) {
			plan.ScreenshotIDs = state.ScreenshotIDs
		}
	}

	resp.Diagnostics.Append(resp.Plan.Set(ctx, plan)...)
}

// Create adopts or starts the set, then makes its contents match.
func (r *appScreenshotSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating app screenshot set resource")

	var plan appScreenshotSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	files := readScreenshotFiles(ctx, plan.Screenshots, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	localizationID := plan.AppStoreVersionLocalizationID.ValueString()
	displayType := plan.ScreenshotDisplayType.ValueString()
	ctx = tflog.SetField(ctx, "app_store_version_localization_id", localizationID)
	ctx = tflog.SetField(ctx, "screenshot_display_type", displayType)

	// A localization holds one set per display type, and the next version
	// starts with a copy of the previous version's, so the set this resource
	// is asked for often exists already.
	set, err := r.client.GetAppScreenshotSetByDisplayType(localizationID, displayType)
	switch {
	case err == nil:
		tflog.Debug(ctx, "Adopting existing screenshot set", map[string]interface{}{"set_id": set.ID})
	case media.IsNotFound(err):
		set, err = r.client.CreateAppScreenshotSet(localizationID, displayType, nil)
		if err != nil {
			addScreenshotSetWriteError(&resp.Diagnostics, localizationID, displayType, err)
			return
		}
	default:
		resp.Diagnostics.AddError(
			"Error Reading Screenshot Sets",
			fmt.Sprintf("Could not check whether localization '%s' already has a %s screenshot set: %s",
				localizationID, displayType, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(set.ID)

	if !r.reconcile(ctx, set.ID, files, &plan, &resp.Diagnostics) {
		// The set exists at Apple whatever happened to its contents. Record
		// what it holds now rather than dropping it: Terraform marks the
		// resource tainted and replaces it on the next apply, where a set
		// missing from state would be one nothing ever deleted.
		r.recordCurrent(ctx, &plan, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with the screenshots Apple holds now.
//
// screenshots is never refreshed: Apple does not know where a file came from.
// The checksums are what change, and the next plan compares them with the
// files.
func (r *appScreenshotSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading app screenshot set resource")

	var state appScreenshotSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	setID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "set_id", setID)

	set, err := r.client.GetAppScreenshotSet(setID)
	if err != nil {
		if media.IsNotFound(err) {
			tflog.Info(ctx, "Screenshot set not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Screenshot Set",
			fmt.Sprintf("Could not read screenshot set '%s': %s", setID, err.Error()),
		)
		return
	}

	state.ScreenshotDisplayType = types.StringValue(set.Attributes.ScreenshotDisplayType)
	r.recordCurrent(ctx, &state, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update makes the set's contents match the configured files.
func (r *appScreenshotSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating app screenshot set resource")

	var plan appScreenshotSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	files := readScreenshotFiles(ctx, plan.Screenshots, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconcile(ctx, plan.ID.ValueString(), files, &plan, &resp.Diagnostics) {
		r.recordCurrent(ctx, &plan, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete deletes the set, which takes its screenshots with it.
func (r *appScreenshotSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appScreenshotSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	setID := state.ID.ValueString()
	if err := r.client.DeleteAppScreenshotSet(setID, nil); err != nil {
		if media.IsNotFound(err) {
			tflog.Info(ctx, "Screenshot set already deleted", map[string]interface{}{"set_id": setID})
			return
		}

		errMsg := err.Error()
		detail := fmt.Sprintf("Could not delete screenshot set '%s': %s", setID, errMsg)
		if strings.Contains(errMsg, "409") {
			detail += "\n\nApple accepts screenshot changes only while the version is being prepared. " +
				"A submitted or released version's screenshots cannot be removed."
		}
		resp.Diagnostics.AddError("Error Deleting Screenshot Set", detail)
		return
	}

	tflog.Info(ctx, "Screenshot set deleted", map[string]interface{}{"set_id": setID})
}

// Configure adds the provider configured client to the resource.
func (r *appScreenshotSetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports a screenshot set by Apple's ID. The parent localization
// is read back from the set itself.
//
// screenshots stays null until the configuration supplies it. The first plan
// after import then compares checksums, and when the files match what Apple
// holds the apply uploads nothing.
func (r *appScreenshotSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing app screenshot set resource", map[string]interface{}{"import_id": req.ID})

	set, err := r.client.GetAppScreenshotSet(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Screenshot Set",
			fmt.Sprintf("Could not read screenshot set '%s': %s", req.ID, err.Error()),
		)
		return
	}

	if set.Relationships == nil || set.Relationships.AppStoreVersionLocalization == nil ||
		set.Relationships.AppStoreVersionLocalization.Data.ID == "" {
		resp.Diagnostics.AddError(
			"Error Importing Screenshot Set",
			fmt.Sprintf("Apple did not report the localization screenshot set '%s' belongs to.", req.ID),
		)
		return
	}

	state := appScreenshotSetModel{
		ID:                            types.StringValue(set.ID),
		AppStoreVersionLocalizationID: types.StringValue(set.Relationships.AppStoreVersionLocalization.Data.ID),
		ScreenshotDisplayType:         types.StringValue(set.Attributes.ScreenshotDisplayType),
		Screenshots:                   types.ListNull(types.StringType),
	}
	r.recordCurrent(ctx, &state, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// screenshotFile is one configured file, read.
type screenshotFile struct {
	path     string
	data     []byte
	checksum string
}

func readScreenshotFiles(ctx context.Context, list types.List, diags *diag.Diagnostics) []screenshotFile {
	var paths []string
	diags.Append(list.ElementsAs(ctx, &paths, false)...)

	files := make([]screenshotFile, 0, len(paths))
	for i, p := range paths {
		data, checksum, err := media.ReadAsset(p)
		if err != nil {
			diags.AddAttributeError(path.Root("screenshots").AtListIndex(i), "Cannot Read Screenshot", err.Error())
			continue
		}
		files = append(files, screenshotFile{path: p, data: data, checksum: checksum})
	}

	return files
}

// reconcile makes the set hold exactly the configured files, in order, and
// records the result in the model. It reports whether it got there.
//
// Screenshots are matched by content, so only new content is uploaded. The
// unclaimed ones are deleted before anything is uploaded, because a set holds
// at most ten and replacing all ten would otherwise briefly need twenty. The
// final order is then set in one request.
func (r *appScreenshotSetResource) reconcile(ctx context.Context, setID string, files []screenshotFile, model *appScreenshotSetModel, diags *diag.Diagnostics) bool {
	existing, err := r.client.GetAppScreenshots(setID)
	if err != nil {
		diags.AddError(
			"Error Reading Screenshots",
			fmt.Sprintf("Could not list the screenshots in set '%s': %s", setID, err.Error()),
		)
		return false
	}

	checksums := make([]string, len(files))
	for i, f := range files {
		checksums[i] = f.checksum
	}

	ids, remove := matchScreenshots(existing, checksums)

	for _, id := range remove {
		tflog.Debug(ctx, "Deleting screenshot", map[string]interface{}{"screenshot_id": id})
		if err := r.client.DeleteAppScreenshot(id, nil); err != nil && !media.IsNotFound(err) {
			addScreenshotWriteError(diags, setID, "delete screenshot '"+id+"' from", err)
			return false
		}
	}

	var processing []string
	for i, f := range files {
		if ids[i] != "" {
			continue
		}

		tflog.Debug(ctx, "Uploading screenshot", map[string]interface{}{"file": f.path})
		asset, err := r.client.UploadAppScreenshot(setID, filepath.Base(f.path), f.data)
		if err != nil {
			addScreenshotWriteError(diags, setID, fmt.Sprintf("upload '%s' to", f.path), err)
			return false
		}
		ids[i] = asset.ID

		if asset.DeliveryState() != models.AssetDeliveryComplete {
			processing = append(processing, f.path)
		}
	}

	// Uploads append, so the order only comes out right by accident. One
	// screenshot has no order to set.
	if len(ids) > 1 {
		if err := r.client.ReorderAppScreenshots(setID, ids, nil); err != nil {
			addScreenshotWriteError(diags, setID, "reorder the screenshots in", err)
			return false
		}
	}

	if len(processing) > 0 {
		diags.AddWarning(
			"Screenshots Still Processing",
			fmt.Sprintf("Apple accepted %s but had not finished processing them. If Apple rejects one "+
				"later, the next plan will upload it again.", strings.Join(processing, ", ")),
		)
	}

	// Record the local checksums rather than Apple's: they are what the plan
	// predicted, and an asset still processing may not report one yet.
	idList, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	checksumList, d := types.ListValueFrom(ctx, types.StringType, checksums)
	diags.Append(d...)
	model.ScreenshotIDs = idList
	model.ScreenshotChecksums = checksumList

	return true
}

// recordCurrent sets the model's IDs and checksums from what the set holds at
// Apple now.
func (r *appScreenshotSetResource) recordCurrent(ctx context.Context, model *appScreenshotSetModel, diags *diag.Diagnostics) {
	shots, err := r.client.GetAppScreenshots(model.ID.ValueString())
	if err != nil {
		diags.AddError(
			"Error Reading Screenshots",
			fmt.Sprintf("Could not list the screenshots in set '%s': %s", model.ID.ValueString(), err.Error()),
		)
		model.ScreenshotIDs = types.ListNull(types.StringType)
		model.ScreenshotChecksums = types.ListNull(types.StringType)
		return
	}

	ids := make([]string, len(shots))
	checksums := make([]string, len(shots))
	for i, shot := range shots {
		ids[i] = shot.ID
		checksums[i] = media.AssetChecksum(shot)
	}

	idList, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	checksumList, d := types.ListValueFrom(ctx, types.StringType, checksums)
	diags.Append(d...)
	model.ScreenshotIDs = idList
	model.ScreenshotChecksums = checksumList
}

func addScreenshotSetWriteError(diags *diag.Diagnostics, localizationID, displayType string, err error) {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "404"):
		diags.AddError(
			"App Store Version Localization Not Found",
			fmt.Sprintf("Could not create a %s screenshot set on localization '%s': %s", displayType, localizationID, errMsg),
		)
	case strings.Contains(errMsg, "409"):
		diags.AddError(
			"Screenshot Set Refused",
			fmt.Sprintf("Apple refused a %s screenshot set on localization '%s': %s\n\n"+
				"Apple accepts screenshot changes only while the version is being prepared, and only "+
				"for display types the app's platform has.", displayType, localizationID, errMsg),
		)
	default:
		diags.AddError(
			"Error Creating Screenshot Set",
			fmt.Sprintf("Could not create a %s screenshot set on localization '%s': %s", displayType, localizationID, errMsg),
		)
	}
}

func addScreenshotWriteError(diags *diag.Diagnostics, setID, action string, err error) {
	errMsg := err.Error()
	detail := fmt.Sprintf("Could not %s screenshot set '%s': %s", action, setID, errMsg)
	if strings.Contains(errMsg, "409") {
		detail += "\n\nApple accepts screenshot changes only while the version is being prepared."
	}
	diags.AddError("Error Writing Screenshots", detail)
}

// knownStrings returns a list's elements when the list and every element are
// known. A path computed by another resource is unknown until apply, and its
// checksum with it.
func knownStrings(list types.List) ([]string, bool) {
	if list.IsNull() || list.IsUnknown() {
		return nil, false
	}

	out := make([]string, 0, len(list.Elements()))
	for _, v := range list.Elements() {
		s, ok := v.(types.String)
		if !ok || s.IsUnknown() || s.IsNull() {
			return nil, false
		}
		out = append(out, s.ValueString())
	}

	return out, true
}
