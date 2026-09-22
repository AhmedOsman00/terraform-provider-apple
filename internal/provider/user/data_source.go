// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"fmt"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &UsersDataSource{}
	_ datasource.DataSourceWithConfigure = &UsersDataSource{}
)

// UsersDataSource lists the members of the App Store Connect team.
type UsersDataSource struct {
	client *apple.Client
}

// NewUsersDataSource creates a new UsersDataSource.
func NewUsersDataSource() datasource.DataSource {
	return &UsersDataSource{}
}

// Metadata returns the data source type name.
func (d *UsersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

// Schema defines the schema for the data source.
func (d *UsersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the members of your App Store Connect team — everybody under Users and " +
			"Access — with their roles, the apps they can see and whether they may create signing " +
			"certificates.\n\n" +
			"Pending invitations are not here. Apple keeps them in a separate collection until they are " +
			"accepted, at which point the invitation is destroyed and the person appears in this one; " +
			"`apple_user_invitation` is what tracks them in between.\n\n" +
			"~> `visible_apps` costs one extra request per member who is restricted to a list, because " +
			"Apple publishes it as a collection of its own rather than as a field. Members who can see " +
			"every app report it as null rather than as your entire catalogue.",

		Attributes: map[string]schema.Attribute{
			// Filter attributes
			"username_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter members by the address they sign in with, using a " +
					"case-insensitive regular expression. Unanchored, so `@example\\.com$` matches a " +
					"domain and `ada` matches anywhere in the address.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"name_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter members by name, using a case-insensitive regular " +
					"expression. Matched against the first and last name joined by a space, so a " +
					"pattern can span both.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"role": schema.StringAttribute{
				MarkdownDescription: "Filter members to those holding this role. Valid values: `ADMIN`, " +
					"`FINANCE`, `ACCOUNT_HOLDER`, `SALES`, `MARKETING`, `APP_MANAGER`, `DEVELOPER`, " +
					"`ACCESS_TO_REPORTS`, `CUSTOMER_SUPPORT`, `CREATE_APPS`, " +
					"`CLOUD_MANAGED_DEVELOPER_ID`, `CLOUD_MANAGED_APP_DISTRIBUTION`, " +
					"`GENERATE_INDIVIDUAL_KEYS`.",
				Optional:   true,
				Validators: []validator.String{RoleValidator},
			},
			"roles": schema.ListAttribute{
				MarkdownDescription: "Filter members to those holding **any** of these roles. Most people " +
					"carry several, so this is an any-of rather than an exact match.",
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(RoleValidator),
					listvalidator.SizeAtMost(len(models.ValidUserRoles)),
				},
			},
			"all_apps_visible": schema.BoolAttribute{
				MarkdownDescription: "Filter members by whether they can see every app on the team.",
				Optional:            true,
			},
			"provisioning_allowed": schema.BoolAttribute{
				MarkdownDescription: "Filter members by whether they may create signing certificates and " +
					"provisioning profiles.",
				Optional: true,
			},

			// Result control attributes
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of members to return (1-200). Defaults to no limit.",
				Optional:            true,
				Validators:          GetLimitValidator(),
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort by. Valid values: `username`, `first_name`, " +
					"`last_name`. Sorting is case-insensitive and ties break on the username.",
				Optional:   true,
				Validators: []validator.String{SortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort order. Valid values: `asc`, `desc`. Defaults to `asc`.",
				Optional:            true,
				Validators:          []validator.String{SortOrderValidator},
			},

			// Output attributes
			"users": schema.ListNestedAttribute{
				MarkdownDescription: "The team members matching the specified filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the member.",
							Computed:            true,
						},
						"username": schema.StringAttribute{
							MarkdownDescription: "The email address of the Apple Account the member signs in with.",
							Computed:            true,
						},
						"first_name": schema.StringAttribute{
							MarkdownDescription: "The member's first name, as their Apple Account reports it.",
							Computed:            true,
						},
						"last_name": schema.StringAttribute{
							MarkdownDescription: "The member's last name, as their Apple Account reports it.",
							Computed:            true,
						},
						"roles": schema.ListAttribute{
							MarkdownDescription: "The roles the member holds.",
							Computed:            true,
							ElementType:         types.StringType,
						},
						"all_apps_visible": schema.BoolAttribute{
							MarkdownDescription: "Whether the member can see every app on the team.",
							Computed:            true,
						},
						"provisioning_allowed": schema.BoolAttribute{
							MarkdownDescription: "Whether the member may create signing certificates and " +
								"provisioning profiles.",
							Computed: true,
						},
						"visible_apps": schema.ListAttribute{
							MarkdownDescription: "The Apple IDs of the apps this member can see, for a " +
								"member restricted to a list. Null when `all_apps_visible` is true — " +
								"Apple would answer with every app on the team, which says nothing.",
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},

			// Metadata attributes
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of members on the team.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of members after applying filters.",
				Computed:            true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *UsersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apple.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *apple.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

// Read refreshes the Terraform state with the latest data.
func (d *UsersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Debug(ctx, "Reading users data source")

	var data usersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	users, err := d.client.GetUsers()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read users: %s", err))

		return
	}

	totalCount := len(users)

	filters, ok := d.buildFilters(ctx, data, &resp.Diagnostics)
	if !ok {
		return
	}

	filtered, err := FilterUsers(users, filters)
	if err != nil {
		resp.Diagnostics.AddError("Filter Error", fmt.Sprintf("Unable to filter users: %s", err))

		return
	}

	SortUsers(filtered, data.SortBy.ValueString(), data.SortOrder.ValueString())

	if !data.Limit.IsNull() {
		filtered = LimitUsers(filtered, int(data.Limit.ValueInt64()))
	}

	tflog.Debug(ctx, "Applied filters to users", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": len(filtered),
	})

	userModels := make([]userDataModel, len(filtered))
	for i, user := range filtered {
		userModels[i] = userDataModel{
			ID:                  types.StringValue(user.ID),
			Username:            stringOrNull(user.Attributes.Username),
			FirstName:           stringOrNull(user.Attributes.FirstName),
			LastName:            stringOrNull(user.Attributes.LastName),
			Roles:               rolesToStrings(user.Attributes.Roles),
			AllAppsVisible:      boolOrNull(user.Attributes.AllAppsVisible),
			ProvisioningAllowed: boolOrNull(user.Attributes.ProvisioningAllowed),
		}

		// Only a member restricted to a list has a list worth reporting: Apple
		// answers the same collection with every app on the team for anybody
		// else, so asking costs a request to learn nothing.
		if boolValue(user.Attributes.AllAppsVisible) {
			continue
		}

		apps, appsErr := d.client.GetUserVisibleApps(user.ID)
		if appsErr != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to read the apps visible to '%s': %s", username(user), appsErr),
			)

			return
		}

		visible := make([]types.String, 0, len(apps))
		for _, app := range apps {
			visible = append(visible, types.StringValue(app.ID))
		}
		userModels[i].VisibleApps = visible
	}

	data.Users = userModels
	data.TotalCount = types.Int64Value(int64(totalCount))
	data.FilteredCount = types.Int64Value(int64(len(filtered)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// buildFilters reads the configured filters into the pure filtering type.
//
// role and roles are one filter: naming both is the union, which is what a
// configuration that sets each of them separately means.
func (d *UsersDataSource) buildFilters(ctx context.Context, data usersDataSourceModel, diags *diag.Diagnostics) (UserFilters, bool) {
	filters := UserFilters{
		UsernamePattern: data.UsernamePattern.ValueString(),
		NamePattern:     data.NamePattern.ValueString(),
	}

	if !data.Role.IsNull() {
		filters.Roles = append(filters.Roles, data.Role.ValueString())
	}

	if !data.Roles.IsNull() && !data.Roles.IsUnknown() {
		var roles []string
		diags.Append(data.Roles.ElementsAs(ctx, &roles, false)...)
		if diags.HasError() {
			return filters, false
		}

		filters.Roles = append(filters.Roles, roles...)
	}

	if !data.AllAppsVisible.IsNull() {
		value := data.AllAppsVisible.ValueBool()
		filters.AllAppsVisible = &value
	}

	if !data.ProvisioningAllowed.IsNull() {
		value := data.ProvisioningAllowed.ValueBool()
		filters.ProvisioningAllowed = &value
	}

	return filters, true
}

// rolesToStrings converts Apple's roles into the data source's element type.
func rolesToStrings(roles []string) []types.String {
	if roles == nil {
		return nil
	}

	values := make([]types.String, 0, len(roles))
	for _, role := range roles {
		values = append(values, types.StringValue(role))
	}

	return values
}
