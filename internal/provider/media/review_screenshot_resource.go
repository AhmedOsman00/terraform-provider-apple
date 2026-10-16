// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package media

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// ReviewScreenshotConfig describes one product kind's review screenshot.
//
// A subscription and an in-app purchase each hold at most one, and the two
// Apple resources behave identically apart from their parent, so one
// implementation serves both. The parent attribute's name differs between them,
// which is why the resource reads and writes attributes by path rather than
// through a tfsdk-tagged struct.
type ReviewScreenshotConfig struct {
	// TypeNameSuffix follows the provider type name, e.g.
	// "_subscription_review_screenshot".
	TypeNameSuffix string
	// ParentAttribute is the attribute naming the product, e.g.
	// "subscription_id".
	ParentAttribute string
	// ParentResource is the resource that attribute refers to, for docs.
	ParentResource string
	// Product names the product kind in prose, e.g. "subscription".
	Product string

	Get    func(c *apple.Client, parentID string) (*models.MediaAsset, error)
	Upload func(c *apple.Client, parentID, fileName string, data []byte) (*models.MediaAsset, error)
	Delete func(c *apple.Client, screenshotID string, authToken *string) error
}

var (
	_ resource.Resource                = &reviewScreenshotResource{}
	_ resource.ResourceWithConfigure   = &reviewScreenshotResource{}
	_ resource.ResourceWithImportState = &reviewScreenshotResource{}
	_ resource.ResourceWithModifyPlan  = &reviewScreenshotResource{}
)

// NewReviewScreenshotResource builds the resource for one product kind.
func NewReviewScreenshotResource(cfg ReviewScreenshotConfig) resource.Resource {
	return &reviewScreenshotResource{cfg: cfg}
}

type reviewScreenshotResource struct {
	client *apple.Client
	cfg    ReviewScreenshotConfig
}

var (
	pathID       = path.Root("id")
	pathFile     = path.Root("file")
	pathChecksum = path.Root("source_file_checksum")
)

// Metadata returns the resource type name.
func (r *reviewScreenshotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.cfg.TypeNameSuffix
}

// Schema defines the schema for the resource.
func (r *reviewScreenshotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	product := r.cfg.Product

	resp.Schema = schema.Schema{
		MarkdownDescription: fmt.Sprintf("Manages the App Store review screenshot of a %[1]s: the image "+
			"App Review is shown of the %[1]s as a customer sees it before buying.\n\n"+
			"A %[1]s holds at most one. Apple has no request that swaps the image, so a changed file is "+
			"uploaded as a new screenshot after the old one is deleted — the `id` changes. The file is "+
			"tracked by its MD5, computed at plan time: editing the image in place is a diff, and so is a "+
			"screenshot replaced by hand in App Store Connect. When the %[1]s already has a screenshot "+
			"with the same content, it is adopted rather than uploaded again.\n\n"+
			"The file parts are sent to the presigned storage URLs Apple issues for the upload, over a "+
			"separate HTTP client that never carries the App Store Connect API token.\n\n"+
			"Apple processes the image after upload. An image it rejects — wrong dimensions, an "+
			"unsupported format — fails the apply with Apple's reason, and the rejected upload is removed.",
			product),

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the screenshot. Changes " +
					"whenever the file does.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			r.cfg.ParentAttribute: schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("The ID of the `%s` the screenshot belongs to. Cannot be "+
					"changed.", r.cfg.ParentResource),
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"file": schema.StringAttribute{
				MarkdownDescription: "Path to the image on the machine running Terraform — a PNG or JPEG. " +
					"Only its content matters: renaming or moving the file without changing it is not a " +
					"change. Null after an import until the configuration supplies it.",
				Required: true,
			},
			"source_file_checksum": schema.StringAttribute{
				MarkdownDescription: "MD5 of the uploaded file, lower-case hex, as Apple records it. " +
					"Computed from `file` at plan time. Empty when Apple failed to process the image, " +
					"which makes the next plan replace it.",
				Computed: true,
			},
		},
	}
}

// ModifyPlan computes the configured file's checksum, so that a change of
// content is a planned change, and marks the ID unknown when the content
// differs from what Apple holds.
func (r *reviewScreenshotResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var file types.String
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, pathFile, &file)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if file.IsUnknown() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, pathChecksum, types.StringUnknown())...)
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, pathID, types.StringUnknown())...)
		return
	}

	_, checksum, err := ReadAsset(file.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(pathFile, "Cannot Read Screenshot", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, pathChecksum, types.StringValue(checksum))...)

	if req.State.Raw.IsNull() {
		return
	}

	var prior types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, pathChecksum, &prior)...)
	if prior.ValueString() != checksum {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, pathID, types.StringUnknown())...)
	}
}

// Create uploads the screenshot, or adopts one with the same content.
func (r *reviewScreenshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating review screenshot resource", map[string]interface{}{"product": r.cfg.Product})

	resp.State.Raw = req.Plan.Raw
	r.write(ctx, &resp.State, &resp.Diagnostics)

	// A failed write leaves the plan's unknowns in place, which state cannot
	// hold. Nothing was left at Apple to track: a failed upload removes its
	// own reservation.
	if resp.Diagnostics.HasError() {
		resp.State.RemoveResource(ctx)
	}
}

// Read refreshes the Terraform state with the screenshot Apple holds now.
//
// The lookup goes through the product rather than the stored ID, so a
// screenshot replaced by hand reads as the replacement -- with its own checksum,
// which the next plan sees differ from the file.
func (r *reviewScreenshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading review screenshot resource")

	var parentID types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(r.cfg.ParentAttribute), &parentID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asset, err := r.cfg.Get(r.client, parentID.ValueString())
	if err != nil {
		if IsNotFound(err) {
			tflog.Info(ctx, "Review screenshot not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Review Screenshot",
			fmt.Sprintf("Could not read the review screenshot of %s '%s': %s",
				r.cfg.Product, parentID.ValueString(), err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathID, types.StringValue(asset.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathChecksum, types.StringValue(AssetChecksum(*asset)))...)
}

// Update replaces the screenshot when its content changed. A change of path
// alone finds the same content already in place and changes nothing at Apple.
func (r *reviewScreenshotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating review screenshot resource", map[string]interface{}{"product": r.cfg.Product})

	resp.State.Raw = req.Plan.Raw
	r.write(ctx, &resp.State, &resp.Diagnostics)

	// Keep the prior state rather than the plan's unknowns; the next refresh
	// reads whatever Apple holds after the partial write.
	if resp.Diagnostics.HasError() {
		resp.State.Raw = req.State.Raw
	}
}

// Delete removes the screenshot. One already gone is not an error.
func (r *reviewScreenshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, pathID, &id)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.cfg.Delete(r.client, id.ValueString(), nil); err != nil {
		if IsNotFound(err) {
			tflog.Info(ctx, "Review screenshot already deleted")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Review Screenshot",
			fmt.Sprintf("Could not delete review screenshot '%s': %s", id.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Review screenshot deleted", map[string]interface{}{"screenshot_id": id.ValueString()})
}

// Configure adds the provider configured client to the resource.
func (r *reviewScreenshotResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports a product's review screenshot.
//
// The import ID is the product's ID, not the screenshot's: a product has at
// most one, and the screenshot's own ID changes every time the file does.
// file stays null until the configuration supplies it; the first plan after
// import then compares checksums and uploads nothing when they agree.
func (r *reviewScreenshotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing review screenshot resource", map[string]interface{}{"import_id": req.ID})

	asset, err := r.cfg.Get(r.client, req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Review Screenshot",
			fmt.Sprintf("Could not read the review screenshot of %s '%s': %s\n\n"+
				"The import ID is the %s ID — a %s holds at most one review screenshot.",
				r.cfg.Product, req.ID, err.Error(), r.cfg.Product, r.cfg.Product),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(r.cfg.ParentAttribute), types.StringValue(req.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathID, types.StringValue(asset.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathChecksum, types.StringValue(AssetChecksum(*asset)))...)
}

// write makes Apple hold the configured file and records the result in state,
// which already holds the plan.
//
// A product holds one review screenshot and Apple refuses a second, so an
// existing one with different content is deleted before the upload. One with
// the same content is adopted: uploading it again would change nothing but the
// ID.
func (r *reviewScreenshotResource) write(ctx context.Context, state *tfsdk.State, diags *diag.Diagnostics) {
	var parentID, file types.String
	diags.Append(state.GetAttribute(ctx, path.Root(r.cfg.ParentAttribute), &parentID)...)
	diags.Append(state.GetAttribute(ctx, pathFile, &file)...)
	if diags.HasError() {
		return
	}

	parent := parentID.ValueString()
	ctx = tflog.SetField(ctx, r.cfg.ParentAttribute, parent)

	data, checksum, err := ReadAsset(file.ValueString())
	if err != nil {
		diags.AddAttributeError(pathFile, "Cannot Read Screenshot", err.Error())
		return
	}

	existing, err := r.cfg.Get(r.client, parent)
	switch {
	case err == nil && AssetChecksum(*existing) == checksum:
		tflog.Debug(ctx, "Adopting review screenshot with identical content", map[string]interface{}{"screenshot_id": existing.ID})
		r.record(ctx, state, existing, diags)
		return
	case err == nil:
		tflog.Debug(ctx, "Deleting review screenshot with different content", map[string]interface{}{"screenshot_id": existing.ID})
		if delErr := r.cfg.Delete(r.client, existing.ID, nil); delErr != nil && !IsNotFound(delErr) {
			diags.AddError(
				"Error Replacing Review Screenshot",
				fmt.Sprintf("Could not delete the current review screenshot of %s '%s' to make room for "+
					"the new one: %s", r.cfg.Product, parent, delErr.Error()),
			)
			return
		}
	case !IsNotFound(err):
		diags.AddError(
			"Error Reading Review Screenshot",
			fmt.Sprintf("Could not check whether %s '%s' already has a review screenshot: %s",
				r.cfg.Product, parent, err.Error()),
		)
		return
	}

	asset, err := r.cfg.Upload(r.client, parent, filepath.Base(file.ValueString()), data)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "404"):
			diags.AddError(
				"Product Not Found",
				fmt.Sprintf("Could not upload a review screenshot to %s '%s': %s", r.cfg.Product, parent, errMsg),
			)
		case strings.Contains(errMsg, "409"):
			diags.AddError(
				"Review Screenshot Refused",
				fmt.Sprintf("Apple refused the review screenshot of %s '%s': %s\n\n"+
					"Apple accepts metadata changes only while a product is not in review.",
					r.cfg.Product, parent, errMsg),
			)
		default:
			diags.AddError(
				"Error Uploading Review Screenshot",
				fmt.Sprintf("Could not upload the review screenshot of %s '%s': %s", r.cfg.Product, parent, errMsg),
			)
		}
		tflog.Error(ctx, "Failed to upload review screenshot", map[string]interface{}{"error": errMsg})
		return
	}

	if asset.DeliveryState() != models.AssetDeliveryComplete {
		diags.AddWarning(
			"Review Screenshot Still Processing",
			fmt.Sprintf("Apple accepted the review screenshot of %s '%s' but had not finished processing it "+
				"(state %q). If Apple rejects it later, the next plan will upload it again.",
				r.cfg.Product, parent, asset.DeliveryState()),
		)
	}

	// The upload is what was planned; record the local checksum rather than
	// Apple's, which a still-processing asset may not report yet.
	diags.Append(state.SetAttribute(ctx, pathID, types.StringValue(asset.ID))...)
	diags.Append(state.SetAttribute(ctx, pathChecksum, types.StringValue(checksum))...)
}

func (r *reviewScreenshotResource) record(ctx context.Context, state *tfsdk.State, asset *models.MediaAsset, diags *diag.Diagnostics) {
	diags.Append(state.SetAttribute(ctx, pathID, types.StringValue(asset.ID))...)
	diags.Append(state.SetAttribute(ctx, pathChecksum, types.StringValue(AssetChecksum(*asset)))...)
}
