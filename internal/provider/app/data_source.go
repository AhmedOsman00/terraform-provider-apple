// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

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
	_ datasource.DataSource              = &appsDataSource{}
	_ datasource.DataSourceWithConfigure = &appsDataSource{}
)

func NewAppsDataSource() datasource.DataSource {
	return &appsDataSource{}
}

type appsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *appsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apps"
}

// Schema defines the schema for the data source.
func (d *appsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads App Store Connect app records.\n\n" +
			"There is no `apple_app` resource, and there will not be one: Apple's API documentation says " +
			"to create new apps on the App Store Connect website, and publishes no endpoint to create or " +
			"delete one. This data source exists because subscription groups hang off an app, so its ID " +
			"has to come from somewhere.\n\n" +
			"Note that an app record is distinct from an `apple_bundle_id`. The Bundle ID is a Developer " +
			"Portal identifier that carries capabilities and signs builds; the app is the App Store " +
			"Connect record that carries metadata, pricing and subscriptions. They share an identifier " +
			"string but are separate objects in separate systems, and creating one does not create the other.",

		Attributes: map[string]schema.Attribute{
			"bundle_id": schema.StringAttribute{
				MarkdownDescription: "Return only the app with this bundle identifier, for example " +
					"`com.example.app`. Exact match.",
				Optional:   true,
				Validators: GetBundleIDValidator(),
			},
			"name_pattern": schema.StringAttribute{
				MarkdownDescription: "Regular expression matched against each app's name. Matches as a " +
					"substring unless anchored with `^` and `$`.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"sku": schema.StringAttribute{
				MarkdownDescription: "Return only the app with this SKU. Exact match.",
				Optional:            true,
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of apps to return. Applied after filtering and sorting.",
				Optional:            true,
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort by: `name` (default), `bundle_id` or `sku`.",
				Optional:            true,
				Validators:          []validator.String{SortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort direction: `asc` (default) or `desc`.",
				Optional:            true,
				Validators:          []validator.String{SortOrderValidator},
			},
			"apps": schema.ListNestedAttribute{
				MarkdownDescription: "The apps matching the filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Apple's identifier for the app, the one to pass to " +
								"`apple_subscription_group`. This is the numeric ID that appears in the " +
								"App Store Connect URL, not the bundle identifier.",
							Computed: true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The app's name in App Store Connect.",
							Computed:            true,
						},
						"bundle_id": schema.StringAttribute{
							MarkdownDescription: "The app's bundle identifier.",
							Computed:            true,
						},
						"sku": schema.StringAttribute{
							MarkdownDescription: "The app's SKU.",
							Computed:            true,
						},
						"primary_locale": schema.StringAttribute{
							MarkdownDescription: "The app's primary locale, which is the locale a " +
								"subscription must at minimum be localized into.",
							Computed: true,
						},
					},
				},
			},
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of apps on the account before filtering.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of apps remaining after filtering, before the limit.",
				Computed:            true,
			},
		},
	}
}

// Read fetches and filters the apps.
func (d *appsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading apps data source")

	var config appsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apps, err := d.client.GetApps()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Apple Apps",
			fmt.Sprintf("Could not retrieve apps from the App Store Connect API: %s", err.Error()),
		)
		return
	}

	totalCount := len(apps)

	filtered, err := FilterApps(apps, AppFilters{
		BundleID:    config.BundleID.ValueString(),
		NamePattern: config.NamePattern.ValueString(),
		SKU:         config.SKU.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid App Filter", err.Error())
		return
	}

	filteredCount := len(filtered)

	sortBy := config.SortBy.ValueString()
	if sortBy == "" {
		sortBy = "name"
	}
	sortOrder := config.SortOrder.ValueString()
	if sortOrder == "" {
		sortOrder = "asc"
	}
	SortApps(filtered, sortBy, sortOrder)

	if limit := int(config.Limit.ValueInt64()); limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	appModels := make([]appModel, len(filtered))
	for i, a := range filtered {
		appModels[i] = appModel{
			ID:            types.StringValue(a.ID),
			Name:          types.StringValue(a.Attributes.Name),
			BundleID:      types.StringValue(a.Attributes.BundleID),
			SKU:           types.StringValue(a.Attributes.SKU),
			PrimaryLocale: types.StringValue(a.Attributes.PrimaryLocale),
		}
	}

	config.Apps = appModels
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Info(ctx, "Apps data source read completed", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"returned_count": len(appModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *appsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
