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
	_ resource.Resource                = &betaAppReviewDetailResource{}
	_ resource.ResourceWithConfigure   = &betaAppReviewDetailResource{}
	_ resource.ResourceWithImportState = &betaAppReviewDetailResource{}
)

func NewBetaAppReviewDetailResource() resource.Resource {
	return &betaAppReviewDetailResource{}
}

type betaAppReviewDetailResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *betaAppReviewDetailResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_beta_app_review_detail"
}

// Schema defines the schema for the resource.
func (r *betaAppReviewDetailResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages what Apple's **beta** reviewers are told about an app: who to " +
			"contact, how to sign in, and anything they need to know before testing a build bound for " +
			"external testers.\n\n" +
			"This is a different Apple record from `apple_app_store_review_detail`, and the two do not " +
			"substitute for each other: this one is per app and gates external TestFlight " +
			"distribution, that one is per version and gates the App Store listing. An app that " +
			"distributes only to internal groups never needs this — internal testers skip beta review " +
			"entirely.\n\n" +
			"~> **Apple publishes no `POST` for this record** — it comes into existence with the app. " +
			"The resource therefore **adopts on create**: `terraform apply` patches what is already " +
			"there, and `terraform destroy` drops it from state and warns rather than deleting " +
			"anything. This is the same shape `apple_app_settings` and `apple_app_info` are in.\n\n" +
			"~> `demo_account_password` is stored in Terraform state in plain text. Supply it from a " +
			"secret store rather than writing it into the configuration, and prefer an account created " +
			"for review alone.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the beta review detail.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app this beta review information belongs to. " +
					"Read it from the `apple_apps` data source. Cannot be changed.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"contact_first_name": schema.StringAttribute{
				MarkdownDescription: "First name of the person beta review should contact.",
				Optional:            true,
			},
			"contact_last_name": schema.StringAttribute{
				MarkdownDescription: "Last name of the person beta review should contact.",
				Optional:            true,
			},
			"contact_phone": schema.StringAttribute{
				MarkdownDescription: "Phone number beta review can reach that person on, including the " +
					"country code — `+20 109 255 8423`.",
				Optional: true,
			},
			"contact_email": schema.StringAttribute{
				MarkdownDescription: "Email address beta review should write to. This is where a rejection " +
					"or a question about the build arrives.",
				Optional:   true,
				Validators: []validator.String{EmailValidator},
			},
			"demo_account_required": schema.BoolAttribute{
				MarkdownDescription: "Whether the app needs an account to be reviewed. Set it to `false` " +
					"for an app that works without signing in, and say so in `notes` — a reviewer who " +
					"cannot find the sign-in screen rejects the build.",
				Optional: true,
			},
			"demo_account_name": schema.StringAttribute{
				MarkdownDescription: "Username of the account beta review should sign in with. Required " +
					"when `demo_account_required` is true.",
				Optional: true,
			},
			"demo_account_password": schema.StringAttribute{
				MarkdownDescription: "Password for the demo account. **Stored in Terraform state in " +
					"plain text** — supply it from a secret store, and use an account created for review " +
					"alone.",
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

// Create adopts the record Apple created with the app.
//
// There is nothing to create: Apple publishes no POST /v1/betaAppReviewDetails.
func (r *betaAppReviewDetailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Adopting beta app review detail resource")

	var plan betaAppReviewDetailModel
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
func (r *betaAppReviewDetailResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading beta app review detail resource")

	var state betaAppReviewDetailModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := state.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	detail, err := r.client.GetBetaAppReviewDetail(appID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Beta app review detail not found, removing from state")
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Beta App Review Detail",
			fmt.Sprintf("Could not read the beta review detail of app '%s': %s", appID, err.Error()),
		)

		return
	}

	applyBetaAppReviewDetail(&state, detail)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update patches the beta review detail in place.
func (r *betaAppReviewDetailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating beta app review detail resource")

	var plan betaAppReviewDetailModel
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

// Delete drops the beta review detail from state.
//
// Apple publishes no DELETE for one, and the record belongs to the app rather
// than to Terraform.
func (r *betaAppReviewDetailResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state betaAppReviewDetailModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Beta App Review Detail Not Deleted",
		fmt.Sprintf("Apple publishes no DELETE endpoint for a beta review detail — the record comes into "+
			"existence with the app — so the contact information on app '%s' remains in App Store "+
			"Connect. Terraform has removed the resource from state only.\n\nNote that the demo account "+
			"password stays on Apple's record as well as in the state file you are discarding.",
			state.AppID.ValueString()),
	)

	tflog.Info(ctx, "Beta app review detail removed from state", map[string]interface{}{
		"app_id": state.AppID.ValueString(),
	})
}

// Configure adds the provider configured client to the resource.
func (r *betaAppReviewDetailResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an app's beta review detail.
//
// The import ID is the app ID, not the detail ID: an app has exactly one, and
// Apple publishes no collection of them to find one in.
func (r *betaAppReviewDetailResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing beta app review detail resource", map[string]interface{}{"import_id": req.ID})

	detail, err := r.client.GetBetaAppReviewDetail(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Beta App Review Detail",
			fmt.Sprintf("Could not read the beta review detail of app '%s': %s\n\n"+
				"The import ID is the app ID — an app has exactly one beta review detail, and Apple "+
				"publishes no collection of them.", req.ID, err.Error()),
		)

		return
	}

	state := betaAppReviewDetailModel{AppID: types.StringValue(req.ID)}
	applyBetaAppReviewDetail(&state, detail)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write patches the record Apple holds for the app.
func (r *betaAppReviewDetailResource) write(ctx context.Context, plan *betaAppReviewDetailModel, diags *diag.Diagnostics) {
	appID := plan.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	existing, err := r.client.GetBetaAppReviewDetail(appID)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404") {
			diags.AddError(
				"Beta App Review Detail Not Found",
				fmt.Sprintf("App '%s' has no beta review detail to adopt: %s\n\n"+
					"Apple creates the record with the app and publishes no endpoint to create one, so "+
					"there is nothing this resource can create. Check that app_id names an app that "+
					"exists and that the API key can read it.", appID, errMsg),
			)
		} else {
			diags.AddError(
				"Error Reading Beta App Review Detail",
				fmt.Sprintf("Could not read the beta review detail of app '%s': %s", appID, errMsg),
			)
		}

		return
	}

	detail, err := r.client.UpdateBetaAppReviewDetail(existing.ID, models.BetaAppReviewDetailAttributes{
		ContactFirstName:    stringOrNil(plan.ContactFirstName),
		ContactLastName:     stringOrNil(plan.ContactLastName),
		ContactPhone:        stringOrNil(plan.ContactPhone),
		ContactEmail:        stringOrNil(plan.ContactEmail),
		DemoAccountName:     stringOrNil(plan.DemoAccountName),
		DemoAccountPassword: stringOrNil(plan.DemoAccountPassword),
		DemoAccountRequired: boolOrNil(plan.DemoAccountRequired),
		Notes:               stringOrNil(plan.Notes),
	}, nil)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "demoAccount") {
			diags.AddError(
				"Demo Account Incomplete",
				fmt.Sprintf("Apple refused the beta review detail on app '%s': %s\n\n"+
					"When demo_account_required is true, both demo_account_name and "+
					"demo_account_password are required.", appID, errMsg),
			)
		} else {
			diags.AddError(
				"Error Writing Beta App Review Detail",
				fmt.Sprintf("Could not write the beta review detail of app '%s': %s", appID, errMsg),
			)
		}
		tflog.Error(ctx, "Failed to write beta app review detail", map[string]interface{}{"error": errMsg})

		return
	}

	applyBetaAppReviewDetail(plan, detail)
}

// applyBetaAppReviewDetail copies Apple's view of the record into the model.
//
// demo_account_password is skipped on purpose -- see Read.
func applyBetaAppReviewDetail(model *betaAppReviewDetailModel, detail *models.BetaAppReviewDetail) {
	model.ID = types.StringValue(detail.ID)
	model.ContactFirstName = stringOrNull(detail.Attributes.ContactFirstName)
	model.ContactLastName = stringOrNull(detail.Attributes.ContactLastName)
	model.ContactPhone = stringOrNull(detail.Attributes.ContactPhone)
	model.ContactEmail = stringOrNull(detail.Attributes.ContactEmail)
	model.DemoAccountName = stringOrNull(detail.Attributes.DemoAccountName)
	model.DemoAccountRequired = boolOrNull(detail.Attributes.DemoAccountRequired)
	model.Notes = stringOrNull(detail.Attributes.Notes)
}
