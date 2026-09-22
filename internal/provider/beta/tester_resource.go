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
	_ resource.Resource                = &betaTesterResource{}
	_ resource.ResourceWithConfigure   = &betaTesterResource{}
	_ resource.ResourceWithImportState = &betaTesterResource{}
)

func NewBetaTesterResource() resource.Resource {
	return &betaTesterResource{}
}

type betaTesterResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *betaTesterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_beta_tester"
}

// Schema defines the schema for the resource.
func (r *betaTesterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Puts one tester in one TestFlight group — the membership half of " +
			"`fastlane pilot add`, and what an `apple_beta_group` needs before it distributes anything.\n\n" +
			"Works for both kinds of group, but they take different people:\n\n" +
			"- **Internal group** — the address must belong to a user of your App Store Connect team. " +
			"Apple rejects anything else, and the tester sees builds as soon as they finish processing.\n" +
			"- **External group** — any address at all, up to 10,000 per app. The tester sees nothing " +
			"until Apple's beta review has approved a build, which is what `apple_beta_app_review_detail` " +
			"is for.\n\n" +
			"!> **Creating this sends a real invitation email.** Apple emails the address as soon as the " +
			"membership exists — for an external group, a stranger receives a TestFlight invitation to " +
			"your app. Point it only at addresses you are entitled to invite.\n\n" +
			"A tester record is the **account's**, not the group's: one record per email address, shared " +
			"by every group and every app it belongs to. So a tester in three groups is three of these " +
			"resources carrying the same `id`, and the first one to be applied is the one that creates " +
			"the record. What each resource owns is the membership — destroying it removes the tester " +
			"from that group and leaves the record, and every other group, alone.\n\n" +
			"~> Apple publishes no update for a tester. `first_name` and `last_name` are sent only when " +
			"the provider creates the record; if the address is already in your account, Apple keeps the " +
			"name it has and the provider warns rather than pretending otherwise.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the **tester record**, " +
					"which is shared with every other group this tester belongs to — it does not identify " +
					"the membership.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the `apple_beta_group` to put the tester in. " +
					"Membership is per group, so moving a tester to a different group is a new membership " +
					"and this replaces the resource.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The tester's email address — their identity across the whole " +
					"account, and where Apple sends the TestFlight invitation. For an internal group it " +
					"must be the address of an App Store Connect user on your team. Apple has no way to " +
					"change it, so a different address is a different tester and this replaces the " +
					"resource.",
				Required:   true,
				Validators: []validator.String{EmailValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"first_name": schema.StringAttribute{
				MarkdownDescription: "The tester's first name, shown beside the address in App Store " +
					"Connect. Sent **only when the provider creates the tester record** — Apple publishes " +
					"no update for one, so an address already in your account keeps the name it has.",
				Optional:   true,
				Validators: GetBetaTesterNameValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"last_name": schema.StringAttribute{
				MarkdownDescription: "The tester's last name. Sent only when the provider creates the " +
					"record — see `first_name`.",
				Optional:   true,
				Validators: GetBetaTesterNameValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"invite_type": schema.StringAttribute{
				MarkdownDescription: "How the tester came to be in TestFlight, as Apple reports it: " +
					"`EMAIL` for one invited by address, `PUBLIC_LINK` for one who joined through a " +
					"group's public link.",
				Computed: true,
			},
		},
	}
}

// Create puts the tester in the group, creating the tester record if the
// account does not already hold one for the address.
//
// The account is asked first because a beta tester is one record per email
// address for the whole team: POSTing an address that already exists is
// refused, and the existing record has to be linked to the group instead. A
// tester new to the account is the common case and costs one lookup that finds
// nothing plus the create.
func (r *betaTesterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating beta tester resource")

	var plan betaTesterModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := plan.GroupID.ValueString()
	email := plan.Email.ValueString()
	ctx = tflog.SetField(ctx, "beta_group_id", groupID)
	ctx = tflog.SetField(ctx, "beta_tester_email", email)

	existing, lookupErr := r.client.GetBetaTesterByEmail(email)
	switch {
	case lookupErr == nil:
		if !r.adoptExistingTester(ctx, &plan, existing, resp) {
			return
		}
	case isNotFound(lookupErr):
		tflog.Debug(ctx, "Creating a new beta tester record")

		tester, err := r.client.CreateBetaTester(groupID, models.BetaTesterCreateAttributes{
			Email:     email,
			FirstName: stringOrNil(plan.FirstName),
			LastName:  stringOrNil(plan.LastName),
		}, nil)

		// A duplicate here means the record appeared between the lookup and the
		// create. That is the ordinary shape of putting one person in two groups:
		// nothing connects the two resources, so Terraform applies them at once
		// and whichever loses the race finds an address it was told did not
		// exist. Linking the record that now exists is what it would have done a
		// moment earlier, and saves every such configuration a depends_on.
		if err != nil && isDuplicate(err) {
			tflog.Debug(ctx, "Beta tester record was created concurrently, linking it instead")

			raced, raceErr := r.client.GetBetaTesterByEmail(email)
			if raceErr != nil {
				r.addWriteError(&resp.Diagnostics, groupID, email, err)

				return
			}

			if !r.adoptExistingTester(ctx, &plan, raced, resp) {
				return
			}

			break
		}

		if err != nil {
			r.addWriteError(&resp.Diagnostics, groupID, email, err)
			tflog.Error(ctx, "Failed to create beta tester", map[string]interface{}{"error": err.Error()})

			return
		}

		applyBetaTester(&plan, tester)
	default:
		resp.Diagnostics.AddError(
			"Error Reading Beta Testers",
			fmt.Sprintf("Could not check whether '%s' is already a tester in this account: %s",
				email, lookupErr.Error()),
		)

		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// adoptExistingTester links a tester the account already holds to the group,
// reporting false when the membership could not be established.
//
// A tester who is already in the group is an error rather than an adoption: the
// membership exists without Terraform knowing about it, and destroying this
// resource would then remove somebody a configuration never added. The import
// path is named instead, which is what the rest of this provider does with an
// "already exists".
func (r *betaTesterResource) adoptExistingTester(
	ctx context.Context,
	plan *betaTesterModel,
	existing *models.BetaTester,
	resp *resource.CreateResponse,
) bool {
	groupID := plan.GroupID.ValueString()
	email := plan.Email.ValueString()

	if _, err := r.client.GetBetaGroupTesterByEmail(groupID, email); err == nil {
		resp.Diagnostics.AddError(
			"Beta Tester Already In Group",
			fmt.Sprintf("'%s' is already a tester in beta group '%s'.\n\n"+
				"Import the membership instead of creating it:\n\n"+
				"  terraform import <address> %s/%s", email, groupID, groupID, email),
		)

		return false
	} else if !isNotFound(err) {
		resp.Diagnostics.AddError(
			"Error Reading Beta Group Testers",
			fmt.Sprintf("Could not check whether '%s' is already in beta group '%s': %s",
				email, groupID, err.Error()),
		)

		return false
	}

	r.warnAboutUnappliedName(&resp.Diagnostics, plan, existing)

	tflog.Debug(ctx, "Adding an existing beta tester to the group",
		map[string]interface{}{"beta_tester_id": existing.ID})

	if err := r.client.AddBetaTesterToGroup(groupID, existing.ID, nil); err != nil {
		r.addWriteError(&resp.Diagnostics, groupID, email, err)
		tflog.Error(ctx, "Failed to add beta tester to group", map[string]interface{}{"error": err.Error()})

		return false
	}

	applyBetaTester(plan, existing)

	return true
}

// Read refreshes the Terraform state with the latest data.
//
// The group's own tester collection is what is asked, because the resource is
// the membership: a tester who exists in the account but has been taken out of
// this group has to read as gone, and a collection scoped to the group cannot
// report otherwise.
//
// email, first_name and last_name are not refreshed. The address is the
// configured identity and Apple echoes it back with whatever capitalisation the
// record was created with; the names are inputs Apple accepts only at creation,
// so adopting either would overwrite what the configuration says with something
// it cannot change.
func (r *betaTesterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading beta tester resource")

	var state betaTesterModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := state.GroupID.ValueString()
	email := state.Email.ValueString()
	ctx = tflog.SetField(ctx, "beta_group_id", groupID)
	ctx = tflog.SetField(ctx, "beta_tester_email", email)

	tester, err := r.client.GetBetaGroupTesterByEmail(groupID, email)
	if err != nil {
		if isNotFound(err) {
			tflog.Info(ctx, "Beta tester no longer in the group, removing from state")
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Beta Tester",
			fmt.Sprintf("Could not read tester '%s' in beta group '%s': %s", email, groupID, err.Error()),
		)

		return
	}

	applyBetaTester(&state, tester)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update is unreachable: every configurable attribute forces replacement,
// because Apple publishes no PATCH /v1/betaTesters and a membership is a
// linkage rather than a record with fields.
func (r *betaTesterResource) Update(ctx context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Error(ctx, "Update called on a Beta Tester, which Apple does not support")

	resp.Diagnostics.AddError(
		"Beta Tester Update Not Supported",
		"App Store Connect cannot update a beta tester or move one between groups, so this resource "+
			"replaces the membership instead. Reaching this method means an attribute changed without "+
			"forcing replacement, which is a bug in the provider: please report it.",
	)
}

// Delete removes the tester from the group.
//
// Not DELETE /v1/betaTesters/{id}, which would remove the person from every app
// and group in the account: this resource owns one membership, so it withdraws
// one membership. The tester stays in the account's tester list, which is also
// what App Store Connect does when a group is removed from under them.
func (r *betaTesterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state betaTesterModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := state.GroupID.ValueString()
	testerID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "beta_group_id", groupID)
	ctx = tflog.SetField(ctx, "beta_tester_id", testerID)

	if err := r.client.RemoveBetaTesterFromGroup(groupID, testerID, nil); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError(
				"Error Removing Beta Tester",
				fmt.Sprintf("Could not remove tester '%s' from beta group '%s': %s",
					state.Email.ValueString(), groupID, err.Error()),
			)

			return
		}

		tflog.Info(ctx, "Beta tester or group already gone")
	}

	tflog.Info(ctx, "Beta tester removed from the group")
}

// Configure adds the provider configured client to the resource.
func (r *betaTesterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports one group membership.
//
// The import ID is composite and there is no bare-ID form, for a reason no
// other resource here has: an Apple tester ID names the person, not the
// membership, and the same ID belongs to every group they are in -- so a bare
// ID cannot say which membership was meant.
//
// first_name and last_name are left null. Apple reports both, but they are
// inputs it accepts only at creation, and adopting a name into a state the
// configuration does not set would plan a replacement that could not change it.
func (r *betaTesterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing beta tester resource", map[string]interface{}{"import_id": req.ID})

	groupID, email, ok := strings.Cut(req.ID, "/")
	if !ok || groupID == "" || email == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Could not parse import ID '%s'. The accepted form is '<group_id>/<email>'.\n\n"+
				"A bare tester ID is not accepted: it names the tester record, which is shared by every "+
				"group that tester belongs to, so it cannot say which membership to import.", req.ID),
		)

		return
	}

	tester, err := r.client.GetBetaGroupTesterByEmail(groupID, email)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Beta Tester",
			fmt.Sprintf("Could not read tester '%s' in beta group '%s': %s", email, groupID, err.Error()),
		)

		return
	}

	state := betaTesterModel{
		GroupID: types.StringValue(groupID),
		Email:   types.StringValue(email),
	}

	applyBetaTester(&state, tester)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// warnAboutUnappliedName reports a configured name that Apple will not apply.
//
// There is no PATCH for a tester, so a record the account already holds keeps
// the name it was created with. Saying so is the honest thing: the alternative
// is a configuration that reads as though it set a name it did not.
func (r *betaTesterResource) warnAboutUnappliedName(diags *diag.Diagnostics, plan *betaTesterModel, existing *models.BetaTester) {
	for attribute, configured := range map[string]types.String{
		"first_name": plan.FirstName,
		"last_name":  plan.LastName,
	} {
		if configured.IsNull() || configured.IsUnknown() {
			continue
		}

		reported := existing.Attributes.FirstName
		if attribute == "last_name" {
			reported = existing.Attributes.LastName
		}

		if reported != nil && *reported == configured.ValueString() {
			continue
		}

		held := "no name"
		if reported != nil {
			held = fmt.Sprintf("'%s'", *reported)
		}

		diags.AddWarning(
			"Beta Tester Name Not Applied",
			fmt.Sprintf("'%s' is already a tester in this App Store Connect account, so the record was "+
				"added to the group rather than created, and %s was not applied: Apple publishes no "+
				"update for a beta tester. Apple holds %s.",
				plan.Email.ValueString(), attribute, held),
		)
	}
}

// addWriteError maps Apple's refusals onto diagnostics that name the cause.
func (r *betaTesterResource) addWriteError(diags *diag.Diagnostics, groupID, email string, err error) {
	errMsg := err.Error()

	switch {
	case isDuplicate(err):
		diags.AddError(
			"Beta Tester Already Exists",
			fmt.Sprintf("Apple refused to add '%s' to beta group '%s': %s\n\n"+
				"A tester record is the account's rather than one group's, so the address may already "+
				"exist without being in this group. If it is in this group, import the membership "+
				"instead of creating it:\n\n"+
				"  terraform import <address> %s/%s", email, groupID, errMsg, groupID, email),
		)
	case strings.Contains(errMsg, "INTERNAL") || strings.Contains(strings.ToLower(errMsg), "internal group") ||
		strings.Contains(errMsg, "not a member"):
		diags.AddError(
			"Not An App Store Connect User",
			fmt.Sprintf("Apple refused to add '%s' to beta group '%s': %s\n\n"+
				"An internal group draws its testers from the users of your App Store Connect team, so "+
				"the address must belong to one of them — add the person under Users and Access first, "+
				"or put them in an external group instead.", email, groupID, errMsg),
		)
	case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
		diags.AddError(
			"Beta Group Not Found",
			fmt.Sprintf("Could not add '%s' to beta group '%s': %s\n\n"+
				"The group must exist before testers can be put in it.", email, groupID, errMsg),
		)
	case strings.Contains(errMsg, "MAXIMUM") || strings.Contains(errMsg, "maximum") ||
		strings.Contains(errMsg, "limit"):
		diags.AddError(
			"Beta Tester Limit Reached",
			fmt.Sprintf("Apple refused to add '%s' to beta group '%s': %s\n\n"+
				"TestFlight allows 10,000 external testers per app and 100 internal ones.",
				email, groupID, errMsg),
		)
	default:
		diags.AddError(
			"Error Adding Beta Tester",
			fmt.Sprintf("Could not add '%s' to beta group '%s': %s", email, groupID, errMsg),
		)
	}
}

// isDuplicate reports whether an error is Apple refusing a record that already
// exists -- for a beta tester, an email address the account already holds.
func isDuplicate(err error) bool {
	errMsg := err.Error()

	return strings.Contains(errMsg, "already exist") ||
		strings.Contains(errMsg, "already in use") ||
		strings.Contains(errMsg, "DUPLICATE")
}

// isNotFound reports whether an error is Apple saying the record is not there,
// including the miss reported by the client's own by-email lookups.
func isNotFound(err error) bool {
	errMsg := err.Error()

	return strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404")
}

// applyBetaTester copies Apple's view of the tester into the model.
//
// Only the record's identifier and Apple's own inviteType are taken: see Read
// for why the address and the names are left as the configuration has them.
func applyBetaTester(model *betaTesterModel, tester *models.BetaTester) {
	model.ID = types.StringValue(tester.ID)
	model.InviteType = stringOrNull(tester.Attributes.InviteType)
}
