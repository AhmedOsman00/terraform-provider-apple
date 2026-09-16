// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package territory

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
	_ datasource.DataSource              = &territoriesDataSource{}
	_ datasource.DataSourceWithConfigure = &territoriesDataSource{}
)

func NewTerritoriesDataSource() datasource.DataSource {
	return &territoriesDataSource{}
}

type territoriesDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *territoriesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_territories"
}

// Schema defines the schema for the data source.
func (d *territoriesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the territories the App Store operates in.\n\n" +
			"This is what makes \"sell everywhere\" expressible. Both " +
			"`apple_subscription_availability` and `apple_in_app_purchase_availability` require " +
			"`available_territories` — Apple's default of every territory applies only to a product " +
			"whose availability has never been set, so once Terraform owns the record the list has to " +
			"be written out in full. Feed `ids` to it rather than pasting a literal list that goes " +
			"stale the next time Apple opens a storefront:\n\n" +
			"```terraform\n" +
			"data \"apple_territories\" \"all\" {}\n\n" +
			"resource \"apple_subscription_availability\" \"example\" {\n" +
			"  subscription_id       = apple_subscription.example.id\n" +
			"  available_territories = data.apple_territories.all.ids\n" +
			"}\n" +
			"```\n\n" +
			"Note that this is not Apple's `TerritoryCode` type, which enumerates every code the API " +
			"will parse — including storefronts the App Store does not operate in. A product can only " +
			"be sold in what this returns.\n\n" +
			"Unlike the other App Store Connect data sources, this one takes no required argument: " +
			"territories belong to the App Store rather than to an app, and every account sees the " +
			"same list.",

		Attributes: map[string]schema.Attribute{
			"currency": schema.StringAttribute{
				MarkdownDescription: "Return only territories trading in this currency, as an ISO 4217 " +
					"alphabetic code such as `USD` or `EUR`. Matched case-insensitively.",
				Optional:   true,
				Validators: GetCurrencyValidator(),
			},
			"id_pattern": schema.StringAttribute{
				MarkdownDescription: "Regular expression matched against each territory's three-letter " +
					"code. Matches as a substring unless anchored with `^` and `$`.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of territories to return. Applied after filtering " +
					"and sorting. Rarely useful here — a truncated availability list is a product " +
					"withdrawn from sale in whatever fell off the end.",
				Optional: true,
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort by: `id` (default) or `currency`. Territories sharing " +
					"a currency stay ordered by ID.",
				Optional:   true,
				Validators: []validator.String{SortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort direction: `asc` (default) or `desc`.",
				Optional:            true,
				Validators:          []validator.String{SortOrderValidator},
			},
			"territories": schema.ListNestedAttribute{
				MarkdownDescription: "The territories matching the filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The three-letter Apple territory code, for example `USA`, " +
								"`GBR` or `EGY`. This is the ID itself, not an opaque identifier — which is " +
								"why a territory can be referenced without looking it up first.",
							Computed: true,
						},
						"currency": schema.StringAttribute{
							MarkdownDescription: "The ISO 4217 currency the territory's storefront trades in. " +
								"Note that the same numeric price means a different amount of money in each.",
							Computed: true,
						},
					},
				},
			},
			"ids": schema.ListAttribute{
				MarkdownDescription: "The matching territory codes alone, in the same order as " +
					"`territories`. This is the attribute to pass to an availability resource's " +
					"`available_territories`.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of territories the App Store operates in, before filtering.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of territories remaining after filtering, before the limit.",
				Computed:            true,
			},
		},
	}
}

// Read fetches and filters the territories.
func (d *territoriesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading territories data source")

	var config territoriesDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	territories, err := d.client.GetTerritories()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Apple Territories",
			fmt.Sprintf("Could not retrieve territories from the App Store Connect API: %s", err.Error()),
		)
		return
	}

	totalCount := len(territories)

	filtered, err := FilterTerritories(territories, TerritoryFilters{
		Currency:  config.Currency.ValueString(),
		IDPattern: config.IDPattern.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid Territory Filter", err.Error())
		return
	}

	filteredCount := len(filtered)

	sortBy := config.SortBy.ValueString()
	if sortBy == "" {
		sortBy = "id"
	}
	sortOrder := config.SortOrder.ValueString()
	if sortOrder == "" {
		sortOrder = "asc"
	}
	SortTerritories(filtered, sortBy, sortOrder)

	if limit := int(config.Limit.ValueInt64()); limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	territoryModels := make([]territoryModel, len(filtered))
	ids := make([]types.String, len(filtered))
	for i, t := range filtered {
		territoryModels[i] = territoryModel{
			ID:       types.StringValue(t.ID),
			Currency: types.StringValue(t.Attributes.Currency),
		}
		ids[i] = types.StringValue(t.ID)
	}

	config.Territories = territoryModels
	config.IDs = ids
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Info(ctx, "Territories data source read completed", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"returned_count": len(territoryModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *territoriesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
