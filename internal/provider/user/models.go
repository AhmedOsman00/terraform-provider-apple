// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package user contains the App Store Connect team membership resources and
// data source for the Apple Terraform provider.
//
// Two Apple records with two lifetimes live here, and folding them together is
// the mistake to avoid. A UserInvitation is an offer: Apple creates it, emails
// it, and destroys it the moment the person accepts. A User is the membership
// that replaces it, with a different identifier and a different set of things
// that can be changed. apple_user_invitation owns the first and
// apple_user owns the second.
package user

import (
	"context"
	"regexp"
	"sort"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// userModel maps one member of the App Store Connect team.
//
// username identifies the person and is what the resource is adopted by:
// Apple's own identifier is opaque and nobody has it to hand. first_name and
// last_name are the person's own -- they come from their Apple Account and no
// request this provider sends can change them -- so both are computed.
type userModel struct {
	ID                  types.String `tfsdk:"id"`
	Username            types.String `tfsdk:"username"`
	FirstName           types.String `tfsdk:"first_name"`
	LastName            types.String `tfsdk:"last_name"`
	Roles               types.Set    `tfsdk:"roles"`
	AllAppsVisible      types.Bool   `tfsdk:"all_apps_visible"`
	ProvisioningAllowed types.Bool   `tfsdk:"provisioning_allowed"`
	VisibleApps         types.Set    `tfsdk:"visible_apps"`
}

// userInvitationModel maps one pending invitation to join the team.
//
// accepted and user_id are how a configuration finds out that the record it
// created is gone for the good reason: Apple destroys an invitation when it is
// accepted, so "no longer pending" and "never joined" look identical from the
// invitation alone.
type userInvitationModel struct {
	ID                  types.String `tfsdk:"id"`
	Email               types.String `tfsdk:"email"`
	FirstName           types.String `tfsdk:"first_name"`
	LastName            types.String `tfsdk:"last_name"`
	Roles               types.Set    `tfsdk:"roles"`
	AllAppsVisible      types.Bool   `tfsdk:"all_apps_visible"`
	ProvisioningAllowed types.Bool   `tfsdk:"provisioning_allowed"`
	VisibleApps         types.Set    `tfsdk:"visible_apps"`
	ExpirationDate      types.String `tfsdk:"expiration_date"`
	Accepted            types.Bool   `tfsdk:"accepted"`
	UserID              types.String `tfsdk:"user_id"`
}

// usersDataSourceModel maps the data source schema for listing team members.
type usersDataSourceModel struct {
	// Filter configuration
	UsernamePattern     types.String `tfsdk:"username_pattern"`
	NamePattern         types.String `tfsdk:"name_pattern"`
	Role                types.String `tfsdk:"role"`
	Roles               types.List   `tfsdk:"roles"`
	AllAppsVisible      types.Bool   `tfsdk:"all_apps_visible"`
	ProvisioningAllowed types.Bool   `tfsdk:"provisioning_allowed"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	Users []userDataModel `tfsdk:"users"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// userDataModel is one team member as the data source reports them.
//
// It is a separate struct from userModel rather than the shared shape the other
// packages use, because the two describe a user differently on purpose: the
// listing reports the name Apple holds and never writes anything, so it has no
// use for the resource's plan modifiers and the resource has no use for a
// visible_apps list it did not configure.
type userDataModel struct {
	ID                  types.String   `tfsdk:"id"`
	Username            types.String   `tfsdk:"username"`
	FirstName           types.String   `tfsdk:"first_name"`
	LastName            types.String   `tfsdk:"last_name"`
	Roles               []types.String `tfsdk:"roles"`
	AllAppsVisible      types.Bool     `tfsdk:"all_apps_visible"`
	ProvisioningAllowed types.Bool     `tfsdk:"provisioning_allowed"`
	VisibleApps         []types.String `tfsdk:"visible_apps"`
}

var (
	// RoleValidator validates one App Store Connect user role.
	RoleValidator = stringvalidator.OneOf(models.ValidUserRoles...)

	// EmailValidator validates the address a person signs in with.
	//
	// Deliberately loose, for the reason the TestFlight one is: rejecting an
	// address Apple would have accepted is worse than passing it through.
	EmailValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`),
		"Username must look like an email address, such as developer@example.com",
	)

	// SortByValidator validates the sort field options.
	SortByValidator = stringvalidator.OneOf("username", "first_name", "last_name")

	// SortOrderValidator validates sort order options.
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetRolesValidator returns validators for a set of roles.
//
// At least one role is required: a member with none has no access at all, and
// Apple rejects the request rather than creating one.
func GetRolesValidator() []validator.Set {
	return []validator.Set{
		setvalidator.SizeAtLeast(1),
		setvalidator.SizeAtMost(len(models.ValidUserRoles)),
		setvalidator.ValueStringsAre(RoleValidator),
	}
}

// GetNameValidator returns validators for the name carried in an invitation.
//
// Counted in characters rather than bytes, like every other name this provider
// sends to Apple: LengthBetween counts bytes and would reject a legal name in
// any non-Latin script at roughly half its length.
func GetNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 255),
	}
}

// GetPatternValidator returns validators for the data source's regex filters.
func GetPatternValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 255),
	}
}

// GetLimitValidator returns validators for the data source's result cap.
func GetLimitValidator() []validator.Int64 {
	return []validator.Int64{
		int64validator.Between(1, 200),
	}
}

// rolesToAPI reads a Terraform set of roles into the slice the API models use.
//
// The result is sorted so that two configurations naming the same roles send
// the same request: a set has no order, and Go's map iteration would otherwise
// churn the body between runs.
func rolesToAPI(ctx context.Context, roles types.Set, diags *diag.Diagnostics) []string {
	if roles.IsNull() || roles.IsUnknown() {
		return nil
	}

	var values []string
	diags.Append(roles.ElementsAs(ctx, &values, false)...)
	sort.Strings(values)

	return values
}

// rolesToTerraform converts Apple's roles back into a Terraform set.
func rolesToTerraform(ctx context.Context, roles []string, diags *diag.Diagnostics) types.Set {
	if roles == nil {
		return types.SetNull(types.StringType)
	}

	set, d := types.SetValueFrom(ctx, types.StringType, roles)
	diags.Append(d...)

	return set
}

// appIDsToAPI reads a Terraform set of app IDs into a slice.
//
// A null set returns nil, which is how the client tells "leave the visible apps
// alone" from "make this list exact": an empty but non-nil slice is a user who
// can see no apps, and is sent as an explicit empty relationship.
func appIDsToAPI(ctx context.Context, apps types.Set, diags *diag.Diagnostics) []string {
	if apps.IsNull() || apps.IsUnknown() {
		return nil
	}

	values := []string{}
	diags.Append(apps.ElementsAs(ctx, &values, false)...)
	sort.Strings(values)

	return values
}

// appIDsToTerraform converts a collection of apps into a set of their IDs.
func appIDsToTerraform(ctx context.Context, apps []models.App, diags *diag.Diagnostics) types.Set {
	ids := make([]string, 0, len(apps))
	for _, app := range apps {
		ids = append(ids, app.ID)
	}
	sort.Strings(ids)

	set, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)

	return set
}

// boolOrNil converts an optional Terraform bool to a pointer.
func boolOrNil(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()

	return &b
}

// stringOrNull converts an optional API string pointer back to Terraform,
// mapping a missing value to null rather than to "".
func stringOrNull(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}

	return types.StringValue(*s)
}

// boolOrNull converts an optional API bool pointer back to Terraform.
func boolOrNull(b *bool) types.Bool {
	if b == nil {
		return types.BoolNull()
	}

	return types.BoolValue(*b)
}

// adoptBool settles a computed boolean after a write.
//
// A planned value is kept whenever there is one -- the configuration's answer
// when the attribute is set, and the prior state's when it is not -- because
// writing Apple's answer over it would be an inconsistent result the framework
// rejects. Apple's value is taken only for an attribute nothing has yet
// decided, which is a create that left it unset. Drift between the two is Read's
// to report, not Create's to paper over.
func adoptBool(planned types.Bool, reported *bool) types.Bool {
	if !planned.IsUnknown() {
		return planned
	}

	if reported != nil {
		return types.BoolValue(*reported)
	}

	return types.BoolNull()
}
