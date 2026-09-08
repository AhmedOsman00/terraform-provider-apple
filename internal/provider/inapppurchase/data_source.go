// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package inapppurchase

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
	_ datasource.DataSource              = &inAppPurchasesDataSource{}
	_ datasource.DataSourceWithConfigure = &inAppPurchasesDataSource{}
)

func NewInAppPurchasesDataSource() datasource.DataSource {
	return &inAppPurchasesDataSource{}
}

type inAppPurchasesDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *inAppPurchasesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_in_app_purchases"
}

// Schema defines the schema for the data source.
func (d *inAppPurchasesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the one-time in-app purchases of an app: consumables, non-consumables " +
			"and non-renewing subscriptions.\n\n" +
			"Auto-renewable subscriptions are not here — they are a different Apple resource, listed by " +
			"`apple_subscriptions`.\n\n" +
			"`app_id` is required because Apple publishes no top-level in-app purchase collection: a " +
			"purchase is reachable only through the app that owns it.",

		Attributes: map[string]schema.Attribute{
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app whose in-app purchases to list. " +
					"Read it from the `apple_apps` data source.",
				Required: true,
			},
			"name_pattern": schema.StringAttribute{
				MarkdownDescription: "Regular expression matched against each purchase's reference name. " +
					"Matches as a substring unless anchored with `^` and `$`.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"product_id_pattern": schema.StringAttribute{
				MarkdownDescription: "Regular expression matched against each purchase's product ID. " +
					"Matches as a substring unless anchored with `^` and `$`.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"in_app_purchase_type": schema.StringAttribute{
				MarkdownDescription: "Return only purchases of this kind. Exact match on `CONSUMABLE`, " +
					"`NON_CONSUMABLE` or `NON_RENEWING_SUBSCRIPTION`.",
				Optional:   true,
				Validators: []validator.String{TypeValidator},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Return only purchases in this state. Exact match on one of " +
					"`MISSING_METADATA`, `WAITING_FOR_UPLOAD`, `PROCESSING_CONTENT`, `READY_TO_SUBMIT`, " +
					"`WAITING_FOR_REVIEW`, `IN_REVIEW`, `DEVELOPER_ACTION_NEEDED`, " +
					"`PENDING_BINARY_APPROVAL`, `APPROVED`, `DEVELOPER_REMOVED_FROM_SALE`, " +
					"`REMOVED_FROM_SALE` or `REJECTED`.",
				Optional:   true,
				Validators: []validator.String{StateValidator},
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of purchases to return. Applied after filtering and sorting.",
				Optional:            true,
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort by: `product_id` (default), `name`, `state` or " +
					"`in_app_purchase_type`.",
				Optional:   true,
				Validators: []validator.String{SortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort direction: `asc` (default) or `desc`.",
				Optional:            true,
				Validators:          []validator.String{SortOrderValidator},
			},
			"in_app_purchases": schema.ListNestedAttribute{
				MarkdownDescription: "The in-app purchases matching the filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the purchase.",
							Computed:            true,
						},
						"app_id": schema.StringAttribute{
							MarkdownDescription: "The app the purchase belongs to. Echoed from the " +
								"argument — Apple's in-app purchase resource has no app relationship, " +
								"so this cannot be read back from the record itself.",
							Computed: true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The internal reference name of the purchase.",
							Computed:            true,
						},
						"product_id": schema.StringAttribute{
							MarkdownDescription: "The product identifier the app passes to StoreKit.",
							Computed:            true,
						},
						"in_app_purchase_type": schema.StringAttribute{
							MarkdownDescription: "Whether the purchase is `CONSUMABLE`, `NON_CONSUMABLE` " +
								"or a `NON_RENEWING_SUBSCRIPTION`.",
							Computed: true,
						},
						"family_sharable": schema.BoolAttribute{
							MarkdownDescription: "Whether the purchase can be shared through Family Sharing.",
							Computed:            true,
						},
						"review_note": schema.StringAttribute{
							MarkdownDescription: "The note supplied to App Review.",
							Computed:            true,
						},
						"state": schema.StringAttribute{
							MarkdownDescription: "Apple's review state for the purchase.",
							Computed:            true,
						},
						"content_hosting": schema.BoolAttribute{
							MarkdownDescription: "Whether Apple hosts the downloadable content for the purchase.",
							Computed:            true,
						},
					},
				},
			},
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of in-app purchases on the app before filtering.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of purchases remaining after filtering, before the limit.",
				Computed:            true,
			},
		},
	}
}

// Read fetches and filters the in-app purchases.
func (d *inAppPurchasesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading in-app purchases data source")

	var config inAppPurchasesDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := config.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	purchases, err := d.client.GetInAppPurchases(appID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read In-App Purchases",
			fmt.Sprintf("Could not retrieve in-app purchases for app '%s': %s", appID, err.Error()),
		)
		return
	}

	totalCount := len(purchases)

	filtered, err := FilterInAppPurchases(purchases, InAppPurchaseFilters{
		NamePattern:       config.NamePattern.ValueString(),
		ProductIDPattern:  config.ProductIDPattern.ValueString(),
		InAppPurchaseType: config.InAppPurchaseType.ValueString(),
		State:             config.State.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid In-App Purchase Filter", err.Error())
		return
	}

	filteredCount := len(filtered)

	sortBy := config.SortBy.ValueString()
	if sortBy == "" {
		sortBy = "product_id"
	}
	sortOrder := config.SortOrder.ValueString()
	if sortOrder == "" {
		sortOrder = "asc"
	}
	SortInAppPurchases(filtered, sortBy, sortOrder)

	if limit := int(config.Limit.ValueInt64()); limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	purchaseModels := make([]inAppPurchaseModel, len(filtered))
	for i, purchase := range filtered {
		model := inAppPurchaseModel{
			ID: types.StringValue(purchase.ID),
			// Echoed from the argument: the collection is already scoped to
			// this app, and the purchase itself never names one.
			AppID:      types.StringValue(appID),
			ReviewNote: stringOrNull(purchase.Attributes.ReviewNote),
		}
		applyPurchaseAttributes(&model, &filtered[i])

		purchaseModels[i] = model
	}

	config.InAppPurchases = purchaseModels
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Info(ctx, "In-app purchases data source read completed", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"returned_count": len(purchaseModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *inAppPurchasesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
