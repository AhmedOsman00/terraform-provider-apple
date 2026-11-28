// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package inapppurchase

import (
	"context"
	"fmt"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &inAppPurchasePricePointEqualizationsDataSource{}
	_ datasource.DataSourceWithConfigure = &inAppPurchasePricePointEqualizationsDataSource{}
)

func NewInAppPurchasePricePointEqualizationsDataSource() datasource.DataSource {
	return &inAppPurchasePricePointEqualizationsDataSource{}
}

type inAppPurchasePricePointEqualizationsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *inAppPurchasePricePointEqualizationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_in_app_purchase_price_point_equalizations"
}

// Schema defines the schema for the data source.
func (d *inAppPurchasePricePointEqualizationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the in-app purchase price point equivalent to one base price point in every " +
			"other territory.\n\n" +
			"**You do not need this to sell a purchase everywhere.** `apple_in_app_purchase_price_schedule` " +
			"takes a `base_territory`, and Apple equalizes the base price into every other storefront " +
			"by itself. This data source reads that same mapping, which is useful for two things:\n\n" +
			"* **Seeing what the equalization charges** before applying it, through `price_points`.\n" +
			"* **Pinning the equalized prices as manual ones.** Apple adjusts equalized prices when " +
			"exchange rates or taxes change; a manual price stays where it was set. Feeding " +
			"`price_point_ids` into the schedule's `prices` fixes every territory at today's " +
			"equalization:\n\n" +
			"```terraform\n" +
			"resource \"apple_in_app_purchase_price_schedule\" \"coins\" {\n" +
			"  in_app_purchase_id = apple_in_app_purchase.coins.id\n" +
			"  base_territory     = \"USA\"\n" +
			"\n" +
			"  prices = [\n" +
			"    for point in values(merge(\n" +
			"      data.apple_in_app_purchase_price_point_equalizations.base.price_point_ids,\n" +
			"      { USA = data.apple_in_app_purchase_price_points.base.price_points[0].id },\n" +
			"    )) : { price_point_id = point }\n" +
			"  ]\n" +
			"}\n" +
			"```\n\n" +
			"The returned points belong to the same purchase the base point does. A price point ID " +
			"encodes its purchase, so an equalization read from one purchase is no more reusable on " +
			"another than the base point was. Neither a subscription's nor an app's price points can " +
			"be used here.",

		Attributes: map[string]schema.Attribute{
			"price_point_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the base price point to equalize from, as returned by " +
					"`apple_in_app_purchase_price_points`.",
				Required: true,
			},
			"territories": schema.ListAttribute{
				MarkdownDescription: "Three-letter Apple territory codes to restrict the result to, for " +
					"example `[\"GBR\", \"EGY\", \"DEU\"]`. Applied by Apple, not in memory. Unlike " +
					"`apple_in_app_purchase_price_points`, leaving this unset is the ordinary case: this " +
					"endpoint returns one record per territory rather than the whole catalogue.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"price_point_ids": schema.MapAttribute{
				MarkdownDescription: "Territory code to price point ID. This is the same shape as `ids` on " +
					"`apple_territories` and exists for the same reason: reaching these IDs through " +
					"`price_points` means a `flatten` and an index expression in every configuration.\n\n" +
					"Whether Apple includes the base point's own territory here is Apple's call, so a " +
					"configuration that needs the base territory too should `merge` the base price point " +
					"in rather than assume.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"price_points": schema.ListNestedAttribute{
				MarkdownDescription: "The equalized price points, one per territory.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The price point ID, to be passed to " +
								"`apple_in_app_purchase_price_schedule`.",
							Computed: true,
						},
						"customer_price": schema.StringAttribute{
							MarkdownDescription: "What the customer pays, in the territory's currency.",
							Computed:            true,
						},
						"proceeds": schema.StringAttribute{
							MarkdownDescription: "What the developer receives, in the territory's currency.",
							Computed:            true,
						},
						"territory_id": schema.StringAttribute{
							MarkdownDescription: "The three-letter Apple territory code this price point belongs to.",
							Computed:            true,
						},
					},
				},
			},
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Number of equalized price points Apple returned.",
				Computed:            true,
			},
		},
	}
}

// Read fetches the equalized price points for the base price point.
func (d *inAppPurchasePricePointEqualizationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading in-app purchase price point equalizations data source")

	var config inAppPurchasePricePointEqualizationsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	pricePointID := config.PricePointID.ValueString()
	ctx = tflog.SetField(ctx, "price_point_id", pricePointID)

	points, err := d.client.GetInAppPurchasePricePointEqualizations(pricePointID, stringListValues(config.Territories))
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read In-App Purchase Price Point Equalizations",
			fmt.Sprintf("Could not retrieve the price points equalized from '%s': %s\n\n"+
				"The ID must be an in-app purchase price point, as returned by the "+
				"apple_in_app_purchase_price_points data source. A price point belonging to an app or to "+
				"a subscription is not addressable here -- Apple models those as separate resources with "+
				"their own catalogues.", pricePointID, err.Error()),
		)
		return
	}

	pointModels := make([]inAppPurchasePricePointModel, len(points))
	for i, point := range points {
		model := inAppPurchasePricePointModel{
			ID:            types.StringValue(point.ID),
			CustomerPrice: types.StringValue(point.Attributes.CustomerPrice),
			Proceeds:      types.StringValue(point.Attributes.Proceeds),
			TerritoryID:   types.StringNull(),
		}
		if point.Relationships != nil {
			model.TerritoryID = relationshipID(point.Relationships.Territory)
		}
		pointModels[i] = model
	}

	ids := PricePointIDsByTerritory(points)
	idModels := make(map[string]types.String, len(ids))
	for territory, id := range ids {
		idModels[territory] = types.StringValue(id)
	}

	if len(points) > 0 && len(idModels) == 0 {
		resp.Diagnostics.AddWarning(
			"Equalized Price Points Have No Territories",
			fmt.Sprintf("Apple returned %d price points equalized from '%s' but no territory for any of "+
				"them, so price_point_ids is empty. Use price_points instead, or report this to the "+
				"provider developers.", len(points), pricePointID),
		)
	}

	config.PricePoints = pointModels
	config.PricePointIDs = idModels
	config.TotalCount = types.Int64Value(int64(len(points)))

	tflog.Info(ctx, "In-app purchase price point equalizations data source read completed", map[string]interface{}{
		"total_count":     len(points),
		"territory_count": len(idModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *inAppPurchasePricePointEqualizationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
