// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

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
	_ datasource.DataSource              = &subscriptionPricePointEqualizationsDataSource{}
	_ datasource.DataSourceWithConfigure = &subscriptionPricePointEqualizationsDataSource{}
)

func NewSubscriptionPricePointEqualizationsDataSource() datasource.DataSource {
	return &subscriptionPricePointEqualizationsDataSource{}
}

type subscriptionPricePointEqualizationsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *subscriptionPricePointEqualizationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_price_point_equalizations"
}

// Schema defines the schema for the data source.
func (d *subscriptionPricePointEqualizationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the price point equivalent to one base price point in every other territory.\n\n" +
			"This is how a subscription gets priced everywhere it sells. `apple_subscription_price_points` " +
			"answers *what may this subscription cost in these territories*, which leaves the caller to " +
			"decide a price per storefront: there is no single `customer_price` to filter on, because " +
			"`9.99` in the United States is neither `9.99` nor a round number anywhere else. Apple already " +
			"holds that mapping — it is what App Store Connect's own price matrix is built from — and this " +
			"data source reads it.\n\n" +
			"Pick one base price point with `apple_subscription_price_points`, pass its ID here, and feed " +
			"`price_point_ids` to an `apple_subscription_price_schedule`, which writes every territory in " +
			"a single request:\n\n" +
			"```terraform\n" +
			"resource \"apple_subscription_price_schedule\" \"plan\" {\n" +
			"  subscription_id = apple_subscription.pro_monthly.id\n" +
			"\n" +
			"  prices = [\n" +
			"    for territory, price_point in data.apple_subscription_price_point_equalizations.base.price_point_ids : {\n" +
			"      territory_id   = territory\n" +
			"      price_point_id = price_point\n" +
			"    }\n" +
			"  ]\n" +
			"\n" +
			"  depends_on = [apple_subscription_availability.pro_monthly]\n" +
			"}\n" +
			"```\n\n" +
			"The same map also works as a `for_each` over `apple_subscription_price`, one resource per " +
			"territory — but that is one `POST /v1/subscriptionPrices` per storefront on apply, and a " +
			"listing of the subscription's whole price collection per storefront on refresh.\n\n" +
			"The returned points belong to the same subscription the base point does. A price point ID " +
			"encodes its subscription, so an equalization read from one subscription is no more reusable " +
			"on another than the base point was — read this once per subscription.\n\n" +
			"A subscription still cannot be priced in a territory it is not available in, and Apple's " +
			"refusal names neither. Keep the `depends_on` above, and make sure " +
			"`apple_subscription_availability` covers every territory this fans out to.",

		Attributes: map[string]schema.Attribute{
			"price_point_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the base price point to equalize from, as returned by " +
					"`apple_subscription_price_points`. Apple derives every other territory's price from " +
					"this one, so it decides what the whole fan-out costs: choose it in the territory you " +
					"actually reason about your pricing in.",
				Required: true,
			},
			"territories": schema.ListAttribute{
				MarkdownDescription: "Three-letter Apple territory codes to restrict the result to, for " +
					"example `[\"GBR\", \"EGY\", \"DEU\"]`. Applied by Apple, not in memory. Unlike " +
					"`apple_subscription_price_points`, leaving this unset is the ordinary case: this " +
					"endpoint returns one record per territory rather than the whole catalogue, which is " +
					"the point of using it.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"price_point_ids": schema.MapAttribute{
				MarkdownDescription: "Territory code to price point ID, ready to be projected into the " +
					"`prices` of an `apple_subscription_price_schedule` or used as a `for_each` over " +
					"`apple_subscription_price`. This is the same shape as `ids` on " +
					"`apple_territories` and exists for the same reason: reaching these IDs through " +
					"`price_points` means a `flatten` and a one-element index expression in every " +
					"configuration that prices more than one storefront.\n\n" +
					"Whether Apple includes the base point's own territory here is Apple's call, so a " +
					"configuration that must price the base territory too should `merge` the base price " +
					"point in rather than assume.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"price_points": schema.ListNestedAttribute{
				MarkdownDescription: "The equalized price points, one per territory. Read these to see what " +
					"Apple's equalization actually charges before applying it; `price_point_ids` is what " +
					"a configuration normally consumes.",
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The price point ID, to be passed to " +
								"`apple_subscription_price_schedule` or `apple_subscription_price`.",
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
				MarkdownDescription: "Number of equalized price points Apple returned.",
				Computed:            true,
			},
		},
	}
}

// Read fetches the equalized price points for the base price point.
func (d *subscriptionPricePointEqualizationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading subscription price point equalizations data source")

	var config subscriptionPricePointEqualizationsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	pricePointID := config.PricePointID.ValueString()
	ctx = tflog.SetField(ctx, "price_point_id", pricePointID)

	territories := make([]string, 0, len(config.Territories))
	for _, territory := range config.Territories {
		if territory.IsNull() || territory.IsUnknown() {
			continue
		}
		territories = append(territories, territory.ValueString())
	}

	points, err := d.client.GetSubscriptionPricePointEqualizations(pricePointID, territories)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Subscription Price Point Equalizations",
			fmt.Sprintf("Could not retrieve the price points equalized from '%s': %s\n\n"+
				"The ID must be a subscription price point, as returned by the "+
				"apple_subscription_price_points data source. A price point belonging to an app or to an "+
				"in-app purchase is not addressable here -- Apple models those as separate resources with "+
				"their own catalogues.", pricePointID, err.Error()),
		)
		return
	}

	pointModels := make([]subscriptionPricePointModel, len(points))
	for i, point := range points {
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

	ids := PricePointIDsByTerritory(points)
	idModels := make(map[string]types.String, len(ids))
	for territory, id := range ids {
		idModels[territory] = types.StringValue(id)
	}

	// An empty map is still a map: a base price point whose equalizations Apple
	// reports without territory linkages would otherwise leave price_point_ids
	// null, and a for_each over null fails with a type error rather than
	// pointing at the read that came back empty.
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

	tflog.Info(ctx, "Subscription price point equalizations data source read completed", map[string]interface{}{
		"total_count":     len(points),
		"territory_count": len(idModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *subscriptionPricePointEqualizationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
