package merchant

import (
	"context"
	"fmt"
	"terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &merchantIDsDataSource{}
	_ datasource.DataSourceWithConfigure = &merchantIDsDataSource{}
)

func NewMerchantIDsDataSource() datasource.DataSource {
	return &merchantIDsDataSource{}
}

// merchantIDsDataSource is the data source implementation.
type merchantIDsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *merchantIDsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_merchant_ids"
}

// Schema defines the schema for the data source.
func (d *merchantIDsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about Merchant IDs from Apple App Store Connect.\n\n" +
			"This data source allows you to query and filter Merchant IDs based on various criteria such as " +
			"identifier patterns, display name patterns. It supports sorting and limiting results.",

		Attributes: map[string]schema.Attribute{
			// Filter attributes
			"identifier_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter Merchant IDs by identifier using a regular expression pattern. " +
					"For example, `merchant\\.com\\.example\\..*` to match all identifiers starting with 'merchant.com.example.'.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"identifier_prefix": schema.StringAttribute{
				MarkdownDescription: "Filter Merchant IDs by identifier prefix. " +
					"For example, `merchant.com.example` to match all identifiers starting with that prefix.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"display_name_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter Merchant IDs by display name using a regular expression pattern. " +
					"Case-insensitive matching is performed.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},

			// Result control attributes
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Limit the number of Merchant IDs returned. Must be between 1 and 200. Defaults to returning all matching Merchant IDs.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 200),
				},
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Sort Merchant IDs by the specified field. Valid values are:\n" +
					"- `display_name` - Sort by display name\n" +
					"- `identifier` - Sort by identifier\n\n" +
					"Defaults to `identifier`.",
				Optional:   true,
				Validators: []validator.String{SortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort order for the results. Valid values are:\n" +
					"- `asc` - Ascending order\n" +
					"- `desc` - Descending order\n\n" +
					"Defaults to `asc`.",
				Optional:   true,
				Validators: []validator.String{SortOrderValidator},
			},

			// Output attributes
			"merchant_ids": schema.ListNestedAttribute{
				MarkdownDescription: "List of Merchant IDs matching the specified criteria.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the Merchant ID.",
							Computed:            true,
						},
						"identifier": schema.StringAttribute{
							MarkdownDescription: "The Merchant ID identifier string.",
							Computed:            true,
						},
						"display_name": schema.StringAttribute{
							MarkdownDescription: "The display name for the Merchant ID.",
							Computed:            true,
						},
					},
				},
			},

			// Computed metadata attributes
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of Merchant IDs in your Apple Developer account.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of Merchant IDs matching the specified filters (before applying limit).",
				Computed:            true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *merchantIDsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading Merchant IDs data source")

	var config merchantIDsDataSourceModel

	// Read Terraform configuration data into the model
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Log filter configuration for debugging
	tflog.Debug(ctx, "Merchant IDs data source configuration", map[string]interface{}{
		"identifier_pattern":   config.IdentifierPattern.ValueString(),
		"identifier_prefix":    config.IdentifierPrefix.ValueString(),
		"display_name_pattern": config.DisplayNamePattern.ValueString(),
		"limit":                config.Limit.ValueInt64(),
		"sort_by":              config.SortBy.ValueString(),
		"sort_order":           config.SortOrder.ValueString(),
	})

	// Get Merchant IDs from Apple API
	merchantIDs, err := d.client.GetMerchantIDs()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Apple Merchant IDs",
			fmt.Sprintf("Could not retrieve Merchant IDs from Apple API: %s", err.Error()),
		)
		return
	}

	totalCount := len(merchantIDs)
	tflog.Debug(ctx, "Retrieved Merchant IDs from Apple API", map[string]interface{}{
		"total_count": totalCount,
	})

	// Apply filters
	filteredMerchantIDs, err := FilterMerchantIDs(merchantIDs, MerchantIDFilters{
		IdentifierPattern:  config.IdentifierPattern.ValueString(),
		IdentifierPrefix:   config.IdentifierPrefix.ValueString(),
		DisplayNamePattern: config.DisplayNamePattern.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Merchant ID Filter",
			err.Error(),
		)
		return
	}

	filteredCount := len(filteredMerchantIDs)
	tflog.Debug(ctx, "Applied filters", map[string]interface{}{
		"filtered_count": filteredCount,
	})

	// Apply sorting
	sortBy := config.SortBy.ValueString()
	if sortBy == "" {
		sortBy = "identifier" // Default sort
	}
	sortOrder := config.SortOrder.ValueString()
	if sortOrder == "" {
		sortOrder = "asc" // Default sort order
	}

	SortMerchantIDs(filteredMerchantIDs, sortBy, sortOrder)
	tflog.Debug(ctx, "Applied sorting", map[string]interface{}{
		"sort_by":    sortBy,
		"sort_order": sortOrder,
	})

	// Apply limit if specified
	limit := int(config.Limit.ValueInt64())
	if limit > 0 && limit < len(filteredMerchantIDs) {
		filteredMerchantIDs = filteredMerchantIDs[:limit]
		tflog.Debug(ctx, "Applied limit", map[string]interface{}{
			"limit": limit,
		})
	}

	// Map Apple API response to Terraform data model
	merchantIDsModels := make([]merchantIDModel, len(filteredMerchantIDs))
	for i, merchantID := range filteredMerchantIDs {
		merchantIDsModels[i] = merchantIDModel{
			ID:          types.StringValue(merchantID.ID),
			Identifier:  types.StringValue(merchantID.Attributes.Identifier),
			DisplayName: types.StringValue(merchantID.Attributes.DisplayName),
		}
	}

	// Set the data source attributes
	config.MerchantIDs = merchantIDsModels
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Info(ctx, "Merchant IDs data source read completed", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"returned_count": len(merchantIDsModels),
	})

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to set state for Merchant IDs data source")
		return
	}

	tflog.Debug(ctx, "Merchant IDs data source read operation completed successfully")
}

// Configure adds the provider configured client to the data source.
func (d *merchantIDsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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
