// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"fmt"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &subscriptionGroupsDataSource{}
	_ datasource.DataSourceWithConfigure = &subscriptionGroupsDataSource{}
)

func NewSubscriptionGroupsDataSource() datasource.DataSource {
	return &subscriptionGroupsDataSource{}
}

type subscriptionGroupsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *subscriptionGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_groups"
}

// Schema defines the schema for the data source.
func (d *subscriptionGroupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the subscription groups belonging to an app.\n\n" +
			"`app_id` is required because Apple publishes no top-level subscription group collection — " +
			"a group is reachable only through the app that owns it.",

		Attributes: map[string]schema.Attribute{
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app whose subscription groups to list. " +
					"Obtain it from the `apple_apps` data source.",
				Required: true,
			},
			"reference_name_pattern": schema.StringAttribute{
				MarkdownDescription: "Regular expression matched against each group's reference name. " +
					"Matches as a substring unless anchored with `^` and `$`.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of subscription groups to return. Applied after filtering and sorting.",
				Optional:            true,
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort by: `reference_name` (default) or `id`.",
				Optional:            true,
				Validators:          []validator.String{GroupSortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort direction: `asc` (default) or `desc`.",
				Optional:            true,
				Validators:          []validator.String{SortOrderValidator},
			},
			"subscription_groups": schema.ListNestedAttribute{
				MarkdownDescription: "The subscription groups matching the filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the subscription group.",
							Computed:            true,
						},
						"app_id": schema.StringAttribute{
							MarkdownDescription: "The app the group belongs to. Echoed from the `app_id` argument: " +
								"Apple's group response carries no app linkage.",
							Computed: true,
						},
						"reference_name": schema.StringAttribute{
							MarkdownDescription: "The internal name of the subscription group.",
							Computed:            true,
						},
					},
				},
			},
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of subscription groups on the app before filtering.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of subscription groups remaining after filtering, before the limit.",
				Computed:            true,
			},
		},
	}
}

// Read fetches and filters the subscription groups.
func (d *subscriptionGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading subscription groups data source")

	var config subscriptionGroupsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := config.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	groups, err := d.client.GetSubscriptionGroups(appID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Subscription Groups",
			fmt.Sprintf("Could not retrieve subscription groups for app '%s': %s", appID, err.Error()),
		)
		return
	}

	totalCount := len(groups)

	filtered, err := FilterSubscriptionGroups(groups, SubscriptionGroupFilters{
		ReferenceNamePattern: config.ReferenceNamePattern.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid Subscription Group Filter", err.Error())
		return
	}

	filteredCount := len(filtered)

	sortBy := config.SortBy.ValueString()
	if sortBy == "" {
		sortBy = "reference_name"
	}
	sortOrder := config.SortOrder.ValueString()
	if sortOrder == "" {
		sortOrder = "asc"
	}
	SortSubscriptionGroups(filtered, sortBy, sortOrder)

	if limit := int(config.Limit.ValueInt64()); limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	groupModels := make([]subscriptionGroupModel, len(filtered))
	for i, group := range filtered {
		groupModels[i] = subscriptionGroupModel{
			ID: types.StringValue(group.ID),
			// Echoed from the argument: Apple's response has no app linkage,
			// and a null here would be indistinguishable from "unknown app".
			AppID:         types.StringValue(appID),
			ReferenceName: types.StringValue(group.Attributes.ReferenceName),
		}
	}

	config.SubscriptionGroups = groupModels
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Info(ctx, "Subscription groups data source read completed", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"returned_count": len(groupModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *subscriptionGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
