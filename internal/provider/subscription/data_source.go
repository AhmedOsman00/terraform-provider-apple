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
	_ datasource.DataSource              = &subscriptionsDataSource{}
	_ datasource.DataSourceWithConfigure = &subscriptionsDataSource{}
)

func NewSubscriptionsDataSource() datasource.DataSource {
	return &subscriptionsDataSource{}
}

type subscriptionsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *subscriptionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscriptions"
}

// Schema defines the schema for the data source.
func (d *subscriptionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the auto-renewable subscriptions in a subscription group.\n\n" +
			"`group_id` is required because Apple publishes no top-level subscription collection — a " +
			"subscription is reachable only through its group.",

		Attributes: map[string]schema.Attribute{
			"group_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the subscription group whose subscriptions to list.",
				Required:            true,
			},
			"name_pattern": schema.StringAttribute{
				MarkdownDescription: "Regular expression matched against each subscription's reference name. " +
					"Matches as a substring unless anchored with `^` and `$`.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"product_id_pattern": schema.StringAttribute{
				MarkdownDescription: "Regular expression matched against each subscription's product ID. " +
					"Matches as a substring unless anchored with `^` and `$`.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Return only subscriptions in this state. Exact match on one of " +
					"`MISSING_METADATA`, `READY_TO_SUBMIT`, `WAITING_FOR_REVIEW`, `IN_REVIEW`, " +
					"`DEVELOPER_ACTION_NEEDED`, `PENDING_BINARY_APPROVAL`, `APPROVED`, " +
					"`DEVELOPER_REMOVED_FROM_SALE`, `REMOVED_FROM_SALE` or `REJECTED`.",
				Optional:   true,
				Validators: []validator.String{StateValidator},
			},
			"subscription_period": schema.StringAttribute{
				MarkdownDescription: "Return only subscriptions with this renewal period. Exact match on one of " +
					"`ONE_WEEK`, `ONE_MONTH`, `TWO_MONTHS`, `THREE_MONTHS`, `SIX_MONTHS` or `ONE_YEAR`.",
				Optional:   true,
				Validators: []validator.String{SubscriptionPeriodValidator},
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of subscriptions to return. Applied after filtering and sorting.",
				Optional:            true,
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort by: `product_id` (default), `name`, `state` or `group_level`.",
				Optional:            true,
				Validators:          []validator.String{SubscriptionSortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort direction: `asc` (default) or `desc`.",
				Optional:            true,
				Validators:          []validator.String{SortOrderValidator},
			},
			"subscriptions": schema.ListNestedAttribute{
				MarkdownDescription: "The subscriptions matching the filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the subscription.",
							Computed:            true,
						},
						"group_id": schema.StringAttribute{
							MarkdownDescription: "The subscription group the subscription belongs to.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The internal reference name of the subscription.",
							Computed:            true,
						},
						"product_id": schema.StringAttribute{
							MarkdownDescription: "The product identifier the app passes to StoreKit.",
							Computed:            true,
						},
						"subscription_period": schema.StringAttribute{
							MarkdownDescription: "How often the subscription renews.",
							Computed:            true,
						},
						"family_sharable": schema.BoolAttribute{
							MarkdownDescription: "Whether the subscription can be shared through Family Sharing.",
							Computed:            true,
						},
						"group_level": schema.Int64Attribute{
							MarkdownDescription: "The rank of the subscription within its group, where 1 is the highest.",
							Computed:            true,
						},
						"review_note": schema.StringAttribute{
							MarkdownDescription: "The note supplied to App Review.",
							Computed:            true,
						},
						"state": schema.StringAttribute{
							MarkdownDescription: "Apple's review state for the subscription.",
							Computed:            true,
						},
					},
				},
			},
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of subscriptions in the group before filtering.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of subscriptions remaining after filtering, before the limit.",
				Computed:            true,
			},
		},
	}
}

// Read fetches and filters the subscriptions.
func (d *subscriptionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading subscriptions data source")

	var config subscriptionsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := config.GroupID.ValueString()
	ctx = tflog.SetField(ctx, "group_id", groupID)

	subscriptions, err := d.client.GetSubscriptions(groupID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Subscriptions",
			fmt.Sprintf("Could not retrieve subscriptions for group '%s': %s", groupID, err.Error()),
		)
		return
	}

	totalCount := len(subscriptions)

	filtered, err := FilterSubscriptions(subscriptions, SubscriptionFilters{
		NamePattern:      config.NamePattern.ValueString(),
		ProductIDPattern: config.ProductIDPattern.ValueString(),
		State:            config.State.ValueString(),
		Period:           config.Period.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid Subscription Filter", err.Error())
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
	SortSubscriptions(filtered, sortBy, sortOrder)

	if limit := int(config.Limit.ValueInt64()); limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	subscriptionModels := make([]subscriptionModel, len(filtered))
	for i, subscription := range filtered {
		model := subscriptionModel{
			ID: types.StringValue(subscription.ID),
			// Echoed from the argument: the collection is already scoped to
			// this group, and Apple does not repeat the linkage per record.
			GroupID:    types.StringValue(groupID),
			ReviewNote: stringOrNull(subscription.Attributes.ReviewNote),
		}
		// applySubscriptionAttributes leaves family_sharable and group_level
		// untouched when Apple omits them, which is correct here: the zero
		// value of a framework type is null, and null is what a Computed
		// attribute should carry for something Apple did not report.
		applySubscriptionAttributes(&model, &filtered[i])

		subscriptionModels[i] = model
	}

	config.Subscriptions = subscriptionModels
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Info(ctx, "Subscriptions data source read completed", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"returned_count": len(subscriptionModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *subscriptionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
