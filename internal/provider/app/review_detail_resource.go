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
	_ resource.Resource                = &appStoreReviewDetailResource{}
	_ resource.ResourceWithConfigure   = &appStoreReviewDetailResource{}
	_ resource.ResourceWithImportState = &appStoreReviewDetailResource{}
)

func NewAppStoreReviewDetailResource() resource.Resource {
	return &appStoreReviewDetailResource{}
}

type appStoreReviewDetailResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *appStoreReviewDetailResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_store_review_detail"
}

// Schema defines the schema for the resource.
func (r *appStoreReviewDetailResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages what App Review is told about a version: who to contact, how to " +
			"sign in, and anything a reviewer needs to know.\n\n" +
			"A version has exactly one review detail. Apple creates one for some versions and not " +
			"others, so this resource adopts an existing record when it finds one and creates it " +
			"otherwise — either way, `terraform apply` ends with the configured values in place.\n\n" +
			"~> **This resource cannot be destroyed.** Apple publishes no `DELETE` for a review detail. " +
			"`terraform destroy` drops it from state and warns; the contact information stays on the " +
			"version.\n\n" +
			"~> `demo_account_password` is stored in Terraform state in plain text. Supply it from a " +
			"secret store rather than writing it into the configuration, and prefer an account created " +
			"for App Review alone.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the review detail.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_store_version_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_app_store_version` this review information " +
					"belongs to. Cannot be changed.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"contact_first_name": schema.StringAttribute{
				MarkdownDescription: "First name of the person App Review should contact.",
				Optional:            true,
			},
			"contact_last_name": schema.StringAttribute{
				MarkdownDescription: "Last name of the person App Review should contact.",
				Optional:            true,
			},
			"contact_phone": schema.StringAttribute{
				MarkdownDescription: "Phone number App Review can reach that person on, including the " +
					"country code — `+20 109 255 8423`.",
				Optional: true,
			},
			"contact_email": schema.StringAttribute{
				MarkdownDescription: "Email address App Review should write to. This is where a rejection " +
					"or a question about the build arrives.",
				Optional: true,
				Validators: []validator.String{
					EmailValidator,
				},
			},
			"demo_account_required": schema.BoolAttribute{
				MarkdownDescription: "Whether the app needs an account to be reviewed. Set it to `false` " +
					"for an app that works without signing in, and say so in `notes` — a reviewer who " +
					"cannot find the sign-in screen rejects the build.",
				Optional: true,
			},
			"demo_account_name": schema.StringAttribute{
				MarkdownDescription: "Username of the account App Review should sign in with. Required " +
					"when `demo_account_required` is true.",
				Optional: true,
			},
			"demo_account_password": schema.StringAttribute{
				MarkdownDescription: "Password for the demo account. **Stored in Terraform state in " +
					"plain text** — supply it from a secret store, and use an account created for App " +
					"Review alone.",
				Optional:  true,
				Sensitive: true,
			},
			"notes": schema.StringAttribute{
				MarkdownDescription: "Anything a reviewer needs to know: how to reach a feature behind a " +
					"paywall, why a permission is asked for, what hardware a feature needs. At most " +
					"4000 characters.",
				Optional:   true,
				Validators: GetReviewNotesValidator(),
			},
		},
	}
}

// Create adopts an existing review detail or creates one.
//
// Apple attaches a review detail to some versions on its own and not to others,
// so a plain POST would fail intermittently with a 409 on exactly the versions
// that already had one. Reading first and patching when a record exists makes
// the outcome the same either way.
func (r *appStoreReviewDetailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating app store review detail resource")

	var plan appStoreReviewDetailModel
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
// demo_account_password is deliberately not refreshed: Apple returns it masked
// on some accounts and omitted on others, and adopting either would overwrite
// the real value with something that is not it.
func (r *appStoreReviewDetailResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading app store review detail resource")

	var state appStoreReviewDetailModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	versionID := state.AppStoreVersionID.ValueString()
	ctx = tflog.SetField(ctx, "app_store_version_id", versionID)

	detail, err := r.client.GetAppStoreReviewDetail(versionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "App store review detail not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading App Store Review Detail",
			fmt.Sprintf("Could not read the review detail of version '%s': %s", versionID, err.Error()),
		)
		return
	}

	applyReviewDetail(&state, detail)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update patches the review detail in place.
func (r *appStoreReviewDetailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating app store review detail resource")

	var plan appStoreReviewDetailModel
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

// Delete drops the review detail from state.
//
// Apple publishes no DELETE for one.
func (r *appStoreReviewDetailResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appStoreReviewDetailModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"App Store Review Detail Not Deleted",
		fmt.Sprintf("Apple publishes no DELETE endpoint for a review detail, so the contact information "+
			"on version '%s' remains in App Store Connect. Terraform has removed the resource from "+
			"state only.\n\nNote that the demo account password stays on Apple's record as well as in "+
			"the state file you are discarding.", state.AppStoreVersionID.ValueString()),
	)

	tflog.Info(ctx, "App store review detail removed from state", map[string]interface{}{
		"app_store_version_id": state.AppStoreVersionID.ValueString(),
	})
}

// Configure adds the provider configured client to the resource.
func (r *appStoreReviewDetailResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports a version's review detail.
//
// The import ID is the App Store version ID, not the detail ID: a version has
// exactly one, and Apple publishes no collection of them to find one in.
func (r *appStoreReviewDetailResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing app store review detail resource", map[string]interface{}{"import_id": req.ID})

	detail, err := r.client.GetAppStoreReviewDetail(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing App Store Review Detail",
			fmt.Sprintf("Could not read the review detail of version '%s': %s\n\n"+
				"The import ID is the App Store version ID — a version has exactly one review detail, "+
				"and Apple publishes no collection of them.", req.ID, err.Error()),
		)
		return
	}

	state := appStoreReviewDetailModel{AppStoreVersionID: types.StringValue(req.ID)}
	applyReviewDetail(&state, detail)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write creates the review detail or patches the one already there.
func (r *appStoreReviewDetailResource) write(ctx context.Context, plan *appStoreReviewDetailModel, diags *diag.Diagnostics) {
	versionID := plan.AppStoreVersionID.ValueString()
	ctx = tflog.SetField(ctx, "app_store_version_id", versionID)

	attributes := models.AppStoreReviewDetailAttributes{
		ContactFirstName:    stringOrNil(plan.ContactFirstName),
		ContactLastName:     stringOrNil(plan.ContactLastName),
		ContactPhone:        stringOrNil(plan.ContactPhone),
		ContactEmail:        stringOrNil(plan.ContactEmail),
		DemoAccountName:     stringOrNil(plan.DemoAccountName),
		DemoAccountPassword: stringOrNil(plan.DemoAccountPassword),
		DemoAccountRequired: boolOrNil(plan.DemoAccountRequired),
		Notes:               stringOrNil(plan.Notes),
	}

	var (
		detail *models.AppStoreReviewDetail
		err    error
	)

	existing, readErr := r.client.GetAppStoreReviewDetail(versionID)
	switch {
	case readErr == nil:
		tflog.Debug(ctx, "Patching existing review detail", map[string]interface{}{"detail_id": existing.ID})
		detail, err = r.client.UpdateAppStoreReviewDetail(existing.ID, attributes, nil)
	case strings.Contains(readErr.Error(), "not found") || strings.Contains(readErr.Error(), "404"):
		tflog.Debug(ctx, "Creating review detail")
		detail, err = r.client.CreateAppStoreReviewDetail(versionID, attributes, nil)
	default:
		diags.AddError(
			"Error Reading App Store Review Detail",
			fmt.Sprintf("Could not check whether version '%s' already has a review detail: %s",
				versionID, readErr.Error()),
		)

		return
	}

	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "demoAccount"):
			diags.AddError(
				"Demo Account Incomplete",
				fmt.Sprintf("Apple refused the review detail on version '%s': %s\n\n"+
					"When demo_account_required is true, both demo_account_name and "+
					"demo_account_password are required.", versionID, errMsg),
			)
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
			diags.AddError(
				"App Store Version Not Found",
				fmt.Sprintf("Could not write the review detail of version '%s': %s", versionID, errMsg),
			)
		default:
			diags.AddError(
				"Error Writing App Store Review Detail",
				fmt.Sprintf("Could not write the review detail of version '%s': %s", versionID, errMsg),
			)
		}
		tflog.Error(ctx, "Failed to write app store review detail", map[string]interface{}{"error": errMsg})

		return
	}

	applyReviewDetail(plan, detail)
}

// applyReviewDetail copies Apple's view of the record into the model.
//
// demo_account_password is skipped on purpose -- see Read.
func applyReviewDetail(model *appStoreReviewDetailModel, detail *models.AppStoreReviewDetail) {
	model.ID = types.StringValue(detail.ID)
	model.ContactFirstName = stringOrNull(detail.Attributes.ContactFirstName)
	model.ContactLastName = stringOrNull(detail.Attributes.ContactLastName)
	model.ContactPhone = stringOrNull(detail.Attributes.ContactPhone)
	model.ContactEmail = stringOrNull(detail.Attributes.ContactEmail)
	model.DemoAccountName = stringOrNull(detail.Attributes.DemoAccountName)
	model.DemoAccountRequired = boolOrNull(detail.Attributes.DemoAccountRequired)
	model.Notes = stringOrNull(detail.Attributes.Notes)
}
