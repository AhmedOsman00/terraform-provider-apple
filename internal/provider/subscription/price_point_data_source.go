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
	_ datasource.DataSource              = &subscriptionPricePointsDataSource{}
	_ datasource.DataSourceWithConfigure = &subscriptionPricePointsDataSource{}
)

func NewSubscriptionPricePointsDataSource() datasource.DataSource {
	return &subscriptionPricePointsDataSource{}
}

type subscriptionPricePointsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *subscriptionPricePointsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_price_points"
}

// Schema defines the schema for the data source.
func (d *subscriptionPricePointsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads Apple's catalogue of permitted prices for a subscription.\n\n" +
			"A subscription price is never a number: `apple_subscription_price` references a price point, " +
			"and each price point fixes a customer price and the developer proceeds for one territory. " +
			"This data source is how you find the ID of the one you want.\n\n" +
			"**Always set `territories`.** The unfiltered catalogue covers every territory the App Store " +
			"sells in and runs to tens of thousands of records; the filter is passed to Apple rather than " +
			"applied in memory, so it is the difference between one request and hundreds. Price point IDs " +
			"are scoped to the subscription they were read from and cannot be reused on another.",

		Attributes: map[string]schema.Attribute{
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the subscription whose price catalogue to read. " +
					"Price points differ per subscription because they encode the subscription's own period.",
				Required: true,
			},
			"territories": schema.ListAttribute{
				MarkdownDescription: "Three-letter Apple territory codes to restrict the catalogue to, for " +
					"example `[\"USA\", \"GBR\", \"EGY\"]`. Applied by Apple, not in memory. Omitting this " +
					"returns every territory, which is rarely what you want.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"customer_price": schema.StringAttribute{
				MarkdownDescription: "Return only price points whose customer price matches this string exactly, " +
					"for example `9.99`. Applied in memory after Apple's territory filter. Note that the " +
					"same numeric price means a different amount of money in each territory's currency.",
				Optional:   true,
				Validators: []validator.String{PriceValidator},
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of price points to return. Applied after filtering.",
				Optional:            true,
			},
			"price_points": schema.ListNestedAttribute{
				MarkdownDescription: "The price points matching the filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The price point ID, to be passed to `apple_subscription_price`.",
							Computed:            true,
						},
						"customer_price": schema.StringAttribute{
							MarkdownDescription: "What the customer pays, in the territory's currency.",
							Computed:            true,
						},
						"proceeds": schema.StringAttribute{
							MarkdownDescription: "What the developer receives, in the territory's currency.",
							Computed:            true,
						},
						"proceeds_year_2": schema.StringAttribute{
							MarkdownDescription: "What the developer receives from the second year onward, when " +
								"Apple's reduced commission for long-term subscribers applies.",
							Computed: true,
						},
						"territory_id": schema.StringAttribute{
							MarkdownDescription: "The three-letter Apple territory code this price point belongs to.",
							Computed:            true,
						},
					},
				},
			},
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Number of price points Apple returned before the in-memory filter.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of price points remaining after filtering, before the limit.",
				Computed:            true,
			},
		},
	}
}

// Read fetches and filters the price points.
func (d *subscriptionPricePointsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading subscription price points data source")

	var config subscriptionPricePointsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := config.SubscriptionID.ValueString()
	ctx = tflog.SetField(ctx, "subscription_id", subscriptionID)

	territories := make([]string, 0, len(config.Territories))
	for _, territory := range config.Territories {
		if territory.IsNull() || territory.IsUnknown() {
			continue
		}
		territories = append(territories, territory.ValueString())
	}

	if len(territories) == 0 {
		tflog.Warn(ctx, "Reading the whole price point catalogue without a territory filter")
	}

	points, err := d.client.GetSubscriptionPricePoints(subscriptionID, territories)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Subscription Price Points",
			fmt.Sprintf("Could not retrieve price points for subscription '%s': %s", subscriptionID, err.Error()),
		)
		return
	}

	totalCount := len(points)

	filtered := FilterSubscriptionPricePoints(points, SubscriptionPricePointFilters{
		CustomerPrice: config.CustomerPrice.ValueString(),
	})
	filteredCount := len(filtered)

	if limit := int(config.Limit.ValueInt64()); limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	pointModels := make([]subscriptionPricePointModel, len(filtered))
	for i, point := range filtered {
		model := subscriptionPricePointModel{
			ID:            types.StringValue(point.ID),
			CustomerPrice: types.StringValue(point.Attributes.CustomerPrice),
			Proceeds:      types.StringValue(point.Attributes.Proceeds),
			ProceedsYear2: types.StringValue(point.Attributes.ProceedsYear2),
			TerritoryID:   types.StringNull(),
		}
		if point.Relationships != nil {
			model.TerritoryID = territoryID(point.Relationships.Territory)
		}
		pointModels[i] = model
	}

	config.PricePoints = pointModels
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Info(ctx, "Subscription price points data source read completed", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"returned_count": len(pointModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *subscriptionPricePointsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
