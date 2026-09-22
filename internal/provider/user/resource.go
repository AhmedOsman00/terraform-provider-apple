// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                   = &userResource{}
	_ resource.ResourceWithConfigure      = &userResource{}
	_ resource.ResourceWithImportState    = &userResource{}
	_ resource.ResourceWithValidateConfig = &userResource{}
)

func NewUserResource() resource.Resource {
	return &userResource{}
}

type userResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

// Schema defines the schema for the resource.
func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages what one member of your App Store Connect team can do: their roles, " +
			"whether they can see every app or only a named few, and whether they may create signing " +
			"certificates and provisioning profiles.\n\n" +
			"~> **This resource adopts an existing team member; it cannot create one.** Apple publishes " +
			"no way to add a person directly — somebody joins by accepting an invitation, which is what " +
			"`apple_user_invitation` sends. Pointing this at an address that is not already on the team " +
			"is an error naming that resource.\n\n" +
			"!> **Destroying this removes the person from the team.** Unlike the records this provider " +
			"adopts and leaves behind, a user has a real `DELETE`: Apple revokes their access to every " +
			"app, and the only way back is a fresh invitation they have to accept again. Taking the " +
			"block out of your configuration is therefore not a no-op.\n\n" +
			"The pair works the way a new hire actually arrives:\n\n" +
			"```terraform\n" +
			"resource \"apple_user_invitation\" \"new_hire\" {\n" +
			"  email      = \"dev@example.com\"\n" +
			"  first_name = \"Ada\"\n" +
			"  last_name  = \"Lovelace\"\n" +
			"  roles      = [\"DEVELOPER\"]\n" +
			"}\n\n" +
			"# Once they have accepted, this takes over — import it, or apply it after the fact.\n" +
			"resource \"apple_user\" \"new_hire\" {\n" +
			"  username = \"dev@example.com\"\n" +
			"  roles    = [\"DEVELOPER\", \"APP_MANAGER\"]\n" +
			"}\n" +
			"```\n\n" +
			"~> The account holder cannot be managed here. `ACCOUNT_HOLDER` is a role Apple reports and " +
			"refuses to assign, so this resource rejects it at plan time rather than letting an apply " +
			"fail against the one account that must never lose access.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the team member.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "The email address of the Apple Account the person signs in with, and " +
					"the handle this resource adopts them by. It belongs to their Apple Account rather " +
					"than to your team, so no request this provider sends can change it: a different " +
					"address is a different person and this replaces the resource.",
				Required:   true,
				Validators: []validator.String{EmailValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"roles": schema.SetAttribute{
				MarkdownDescription: "The roles the member holds, as App Store Connect shows them under " +
					"Users and Access. At least one is required. Valid values: `ADMIN`, `FINANCE`, " +
					"`SALES`, `MARKETING`, `APP_MANAGER`, `DEVELOPER`, `ACCESS_TO_REPORTS`, " +
					"`CUSTOMER_SUPPORT`, `CREATE_APPS`, `CLOUD_MANAGED_DEVELOPER_ID`, " +
					"`CLOUD_MANAGED_APP_DISTRIBUTION`, `GENERATE_INDIVIDUAL_KEYS`.\n\n" +
					"`ACCOUNT_HOLDER` is reported by Apple for the person who owns the membership and " +
					"cannot be assigned, so it is rejected here.\n\n" +
					"`ADMIN` subsumes the rest: naming another role beside it changes nothing.",
				Required:    true,
				ElementType: types.StringType,
				Validators:  GetRolesValidator(),
			},
			"all_apps_visible": schema.BoolAttribute{
				MarkdownDescription: "Whether the member can see every app on the team, including ones " +
					"created after they joined. Set it to `false` and name the apps in `visible_apps` to " +
					"restrict them. Apple decides this for you when neither is configured.",
				Optional: true,
				Computed: true,
			},
			"visible_apps": schema.SetAttribute{
				MarkdownDescription: "The Apple IDs of the apps this member can see, used when " +
					"`all_apps_visible` is `false`. An empty set is a member who can see none.\n\n" +
					"Leave it unset to let Apple's existing answer stand — the list is only read back " +
					"when the configuration sets it, because a member who sees every app is reported as " +
					"seeing all of them and adopting that would write your whole app catalogue into state.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"provisioning_allowed": schema.BoolAttribute{
				MarkdownDescription: "Whether the member may create signing certificates and provisioning " +
					"profiles in the Developer Portal — the access `apple_certificate` and " +
					"`apple_profile` need. Defaults to `false`, which is Apple's own default for a new " +
					"member.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"first_name": schema.StringAttribute{
				MarkdownDescription: "The member's first name, as their Apple Account reports it. Apple's " +
					"update request carries no name, so this is Apple's alone: it is what they set on " +
					"their own account, not something a team can change.",
				Computed: true,
			},
			"last_name": schema.StringAttribute{
				MarkdownDescription: "The member's last name, as their Apple Account reports it. Apple's " +
					"alone — see `first_name`.",
				Computed: true,
			},
		},
	}
}

// ValidateConfig rejects the two combinations Apple refuses, at plan time and
// with the reason rather than the attribute name.
func (r *userResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config userModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.AllAppsVisible.IsNull() && !config.AllAppsVisible.IsUnknown() && config.AllAppsVisible.ValueBool() &&
		!config.VisibleApps.IsNull() && !config.VisibleApps.IsUnknown() && len(config.VisibleApps.Elements()) > 0 {
		resp.Diagnostics.AddError(
			"Conflicting App Visibility",
			"all_apps_visible is true and visible_apps names apps. A member who can see every app has "+
				"no list of apps, and Apple rejects a request carrying both.\n\n"+
				"Set all_apps_visible = false to restrict the member to visible_apps, or drop "+
				"visible_apps to let them see everything.",
		)
	}

	validateAssignableRoles(ctx, config.Roles, &resp.Diagnostics)
}

// Create adopts the team member and applies the configuration to them.
//
// There is no create path to add here: Apple publishes none, and a user record
// comes into existence only when somebody accepts an invitation. Adoption is
// therefore the whole of it -- a lookup that must succeed, then the same PATCH
// Update sends.
func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating user resource")

	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	username := plan.Username.ValueString()
	ctx = tflog.SetField(ctx, "username", username)

	existing, err := r.client.GetUserByUsername(username)
	if err != nil {
		if isNotFound(err) {
			resp.Diagnostics.AddError(
				"User Not On The Team",
				fmt.Sprintf("'%s' is not a member of this App Store Connect team.\n\n"+
					"Apple publishes no way to add one directly — a person joins by accepting an "+
					"invitation. Send one with apple_user_invitation and apply this once they have "+
					"accepted:\n\n"+
					"  resource \"apple_user_invitation\" \"example\" {\n"+
					"    email      = %q\n"+
					"    first_name = \"…\"\n"+
					"    last_name  = \"…\"\n"+
					"    roles      = [\"DEVELOPER\"]\n"+
					"  }", username, username),
			)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Users",
			fmt.Sprintf("Could not check whether '%s' is a member of this team: %s", username, err.Error()),
		)

		return
	}

	plan.ID = types.StringValue(existing.ID)

	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with the latest data.
//
// The member is looked up by Apple's identifier rather than by username: the
// address is the configuration's identity for them, and reading by it would
// make a person who left and rejoined look like the same resource.
//
// visible_apps is refreshed only when state holds a list. It is a second
// request, and for a member who can see every app Apple answers it with every
// app on the team -- so adopting it for a configuration that never set the
// attribute would write the whole catalogue into state and show a diff for
// something nobody configured. This is the rule apple_app_store_version follows
// for the build it does not manage.
func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading user resource")

	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "user_id", userID)

	user, err := r.client.GetUser(userID)
	if err != nil {
		if isNotFound(err) {
			tflog.Info(ctx, "User is no longer on the team, removing from state")
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError(
			"Error Reading User",
			fmt.Sprintf("Could not read user '%s': %s", state.Username.ValueString(), err.Error()),
		)

		return
	}

	state.Username = stringOrNull(user.Attributes.Username)
	state.FirstName = stringOrNull(user.Attributes.FirstName)
	state.LastName = stringOrNull(user.Attributes.LastName)
	state.Roles = rolesToTerraform(ctx, user.Attributes.Roles, &resp.Diagnostics)
	state.AllAppsVisible = boolOrNull(user.Attributes.AllAppsVisible)
	state.ProvisioningAllowed = boolOrNull(user.Attributes.ProvisioningAllowed)

	if !state.VisibleApps.IsNull() {
		apps, err := r.client.GetUserVisibleApps(userID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Visible Apps",
				fmt.Sprintf("Could not read the apps visible to '%s': %s",
					state.Username.ValueString(), err.Error()),
			)

			return
		}

		state.VisibleApps = appIDsToTerraform(ctx, apps, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update applies a changed set of roles, app visibility or provisioning access.
func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating user resource")

	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID
	ctx = tflog.SetField(ctx, "user_id", plan.ID.ValueString())

	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete removes the person from the App Store Connect team.
//
// This is a real deletion rather than the state-only drop the adopted records
// use, because leaving a member in place would be the wrong answer to a
// configuration that no longer names them: they would keep access to every app
// on the team. Apple refuses to remove the account holder, which is reported as
// itself.
func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := state.ID.ValueString()
	username := state.Username.ValueString()
	ctx = tflog.SetField(ctx, "user_id", userID)

	if err := r.client.DeleteUser(userID, nil); err != nil {
		if isNotFound(err) {
			tflog.Info(ctx, "User is already off the team")

			return
		}

		if isAccountHolder(err) {
			resp.Diagnostics.AddError(
				"Cannot Remove The Account Holder",
				fmt.Sprintf("Apple refused to remove '%s' from the team: %s\n\n"+
					"The account holder owns the Apple Developer Program membership and cannot be "+
					"removed through the API. Transfer the membership in App Store Connect first, then "+
					"remove this resource from state with `terraform state rm`.", username, err.Error()),
			)

			return
		}

		resp.Diagnostics.AddError(
			"Error Removing User",
			fmt.Sprintf("Could not remove '%s' from the team: %s", username, err.Error()),
		)

		return
	}

	tflog.Info(ctx, "User removed from the team")
}

// Configure adds the provider configured client to the resource.
func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports one team member by Apple's identifier or by the address
// they sign in with.
//
// The two are told apart by shape: an Apple user ID is a UUID and a username is
// an email address, so the "@" decides. Both are unique within a team.
//
// visible_apps is imported only for a member who is restricted to a list, which
// is the same rule Read follows and for the same reason.
func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing user resource", map[string]interface{}{"import_id": req.ID})

	var (
		user *models.User
		err  error
	)

	if strings.Contains(req.ID, "@") {
		user, err = r.client.GetUserByUsername(req.ID)
	} else {
		user, err = r.client.GetUser(req.ID)
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing User",
			fmt.Sprintf("Could not read user '%s': %s\n\n"+
				"The accepted import IDs are the Apple user ID and the address the person signs in "+
				"with. Both are listed by the apple_users data source.", req.ID, err.Error()),
		)

		return
	}

	state := userModel{
		ID:                  types.StringValue(user.ID),
		Username:            stringOrNull(user.Attributes.Username),
		FirstName:           stringOrNull(user.Attributes.FirstName),
		LastName:            stringOrNull(user.Attributes.LastName),
		Roles:               rolesToTerraform(ctx, user.Attributes.Roles, &resp.Diagnostics),
		AllAppsVisible:      boolOrNull(user.Attributes.AllAppsVisible),
		ProvisioningAllowed: boolOrNull(user.Attributes.ProvisioningAllowed),
		VisibleApps:         types.SetNull(types.StringType),
	}

	if user.Attributes.AllAppsVisible != nil && !*user.Attributes.AllAppsVisible {
		apps, appsErr := r.client.GetUserVisibleApps(user.ID)
		if appsErr != nil {
			resp.Diagnostics.AddError(
				"Error Importing Visible Apps",
				fmt.Sprintf("Could not read the apps visible to '%s': %s", req.ID, appsErr.Error()),
			)

			return
		}

		state.VisibleApps = appIDsToTerraform(ctx, apps, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// write sends the plan to Apple and settles the computed attributes from the
// response.
//
// visible_apps is passed through only when the configuration holds a list: a
// nil slice leaves Apple's existing answer alone, where an empty one is a
// member who can see no apps and has to be sent as an explicit empty
// relationship.
func (r *userResource) write(ctx context.Context, plan *userModel, diags *diag.Diagnostics) {
	roles := rolesToAPI(ctx, plan.Roles, diags)
	visibleApps := appIDsToAPI(ctx, plan.VisibleApps, diags)
	if diags.HasError() {
		return
	}

	attributes := models.UserUpdateAttributes{
		Roles:               roles,
		AllAppsVisible:      boolOrNil(plan.AllAppsVisible),
		ProvisioningAllowed: boolOrNil(plan.ProvisioningAllowed),
	}

	// A named list of apps is only meaningful for a member who is not seeing
	// everything, and Apple rejects a request that says both. Saying so here
	// rather than relying on the configuration keeps an unset all_apps_visible
	// from quietly contradicting a list that was set.
	if visibleApps != nil && attributes.AllAppsVisible == nil {
		allAppsVisible := false
		attributes.AllAppsVisible = &allAppsVisible
		plan.AllAppsVisible = types.BoolValue(false)
	}

	tflog.Debug(ctx, "Updating user", map[string]interface{}{"roles": roles})

	user, err := r.client.UpdateUser(plan.ID.ValueString(), attributes, visibleApps, nil)
	if err != nil {
		r.addWriteError(diags, plan.Username.ValueString(), err)

		return
	}

	plan.FirstName = stringOrNull(user.Attributes.FirstName)
	plan.LastName = stringOrNull(user.Attributes.LastName)
	plan.AllAppsVisible = adoptBool(plan.AllAppsVisible, user.Attributes.AllAppsVisible)
	plan.ProvisioningAllowed = adoptBool(plan.ProvisioningAllowed, user.Attributes.ProvisioningAllowed)
}

// addWriteError maps Apple's refusals onto diagnostics that name the cause.
func (r *userResource) addWriteError(diags *diag.Diagnostics, username string, err error) {
	errMsg := err.Error()

	switch {
	case isAccountHolder(err):
		diags.AddError(
			"Cannot Change The Account Holder",
			fmt.Sprintf("Apple refused to update '%s': %s\n\n"+
				"The account holder's roles cannot be changed through the API, and ACCOUNT_HOLDER "+
				"cannot be assigned to anybody else. Transfer the membership in App Store Connect "+
				"instead.", username, errMsg),
		)
	case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
		diags.AddError(
			"User Or App Not Found",
			fmt.Sprintf("Apple could not find something the request named while updating '%s': %s\n\n"+
				"Either the member has been removed from the team, or an Apple ID in visible_apps "+
				"names an app this team does not hold.", username, errMsg),
		)
	case strings.Contains(errMsg, "FORBIDDEN") || strings.Contains(errMsg, "403"):
		diags.AddError(
			"Not Permitted To Manage Users",
			fmt.Sprintf("Apple refused to update '%s': %s\n\n"+
				"Managing team members needs an App Store Connect API key with the Admin role. A key "+
				"with Developer or App Manager access can read /v1/users and not write to it.",
				username, errMsg),
		)
	default:
		diags.AddError(
			"Error Updating User",
			fmt.Sprintf("Could not update '%s': %s", username, errMsg),
		)
	}
}

// validateAssignableRoles rejects a role Apple reports but will not accept.
//
// ACCOUNT_HOLDER is the only one: it names the person who owns the membership,
// and a configuration carrying it would fail on every apply rather than on the
// plan that introduced it.
func validateAssignableRoles(ctx context.Context, roles types.Set, diags *diag.Diagnostics) {
	if roles.IsNull() || roles.IsUnknown() {
		return
	}

	var values []string
	diags.Append(roles.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		return
	}

	for _, role := range values {
		if role == models.UserRoleAccountHolder {
			diags.AddError(
				"Account Holder Role Cannot Be Assigned",
				"roles contains ACCOUNT_HOLDER. Apple reports that role for the person who owns the "+
					"Apple Developer Program membership and refuses every request that tries to grant "+
					"it, so a configuration carrying it can never apply.\n\n"+
					"The account holder cannot be managed by this provider at all: their roles are "+
					"fixed and they cannot be removed from the team. Transfer the membership in App "+
					"Store Connect if it belongs to somebody else.",
			)

			return
		}
	}
}

// isNotFound reports whether an error is Apple saying the record is not there,
// including the miss reported by the client's own lookups.
func isNotFound(err error) bool {
	errMsg := err.Error()

	return strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404")
}

// isAccountHolder reports whether Apple refused a write because it named the
// account holder.
func isAccountHolder(err error) bool {
	errMsg := strings.ToUpper(err.Error())

	return strings.Contains(errMsg, "ACCOUNT_HOLDER") || strings.Contains(errMsg, "ACCOUNT HOLDER")
}
