package bundle

import (
	"context"
	"fmt"
	"terraform-provider-apple/internal/apple"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &bundleIDsDataSource{}
	_ datasource.DataSourceWithConfigure = &bundleIDsDataSource{}
)

func NewBundleIDsDataSource() datasource.DataSource {
	return &bundleIDsDataSource{}
}

// bundleIDsDataSource is the data source implementation.
type bundleIDsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *bundleIDsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bundle_ids"
}

// Schema defines the schema for the data source.
func (d *bundleIDsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about Bundle IDs from Apple App Store Connect.\n\n" +
			"This data source allows you to query and filter Bundle IDs based on various criteria such as " +
			"platform, identifier patterns, and name patterns. It supports sorting and limiting results.",

		Attributes: map[string]schema.Attribute{
			// Filter attributes
			"platform": schema.StringAttribute{
				MarkdownDescription: "Filter Bundle IDs by platform. Valid values are:\n" +
					"- `IOS` - iOS platform\n" +
					"- `MAC_OS` - macOS platform\n" +
					"- `TV_OS` - tvOS platform\n" +
					"- `WATCH_OS` - watchOS platform\n\n" +
					"Cannot be used together with `platforms`.",
				Optional:   true,
				Validators: []validator.String{PlatformValidator},
			},
			"platforms": schema.ListAttribute{
				MarkdownDescription: "Filter Bundle IDs by multiple platforms. Accepts a list of platform values.\n" +
					"Cannot be used together with `platform`.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  []validator.List{
					// TODO: Add custom validator to ensure mutual exclusivity with platform
				},
			},
			"identifier_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter Bundle IDs by identifier using a regular expression pattern. " +
					"For example, `^com\\.example\\.*` to match all Bundle IDs starting with `com.example.`.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"identifier_prefix": schema.StringAttribute{
				MarkdownDescription: "Filter Bundle IDs by identifier prefix. " +
					"For example, `com.example` to match all Bundle IDs starting with that prefix.",
				Optional: true,
				Validators: []validator.String{
					BundleIdentifierValidator,
				},
			},
			"name_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter Bundle IDs by name using a glob pattern. " +
					"For example, `*MyApp*` to match Bundle IDs containing 'MyApp' in the name.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},

			// Result control attributes
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of Bundle IDs to return. Must be between 1 and 200.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 200),
				},
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort Bundle IDs by. Valid values are:\n" +
					"- `name` - Sort by Bundle ID name\n" +
					"- `identifier` - Sort by Bundle ID identifier\n" +
					"- `platform` - Sort by platform\n\n" +
					"Defaults to `identifier` if not specified.",
				Optional:   true,
				Validators: []validator.String{SortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort order for the results. Valid values are:\n" +
					"- `asc` - Ascending order (default)\n" +
					"- `desc` - Descending order",
				Optional:   true,
				Validators: []validator.String{SortOrderValidator},
			},

			// Output attributes
			"bundle_ids": schema.ListNestedAttribute{
				MarkdownDescription: "List of Bundle IDs that match the specified filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the Bundle ID.",
							Computed:            true,
						},
						"identifier": schema.StringAttribute{
							MarkdownDescription: "The Bundle ID identifier string in reverse domain format.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The human-readable name for the Bundle ID.",
							Computed:            true,
						},
						"platform": schema.StringAttribute{
							MarkdownDescription: "The platform for the Bundle ID (IOS, MAC_OS, TV_OS, WATCH_OS).",
							Computed:            true,
						},
						"seed_id": schema.StringAttribute{
							MarkdownDescription: "The seed ID for the Bundle ID, assigned by Apple.",
							Computed:            true,
						},
					},
				},
			},

			// Computed metadata attributes
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of Bundle IDs retrieved from Apple before filtering.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of Bundle IDs after applying filters and before applying limit.",
				Computed:            true,
			},
			"last_updated": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the data was last retrieved from Apple App Store Connect.",
				Computed:            true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *bundleIDsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading Bundle IDs data source")

	var config bundleIDsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Fetching Bundle IDs from Apple API")

	// Get all Bundle IDs from Apple
	bundleIDs, err := d.client.GetBundleIDs()
	if err != nil {
		tflog.Error(ctx, "Failed to fetch Bundle IDs", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Unable to Read Bundle IDs",
			fmt.Sprintf("Could not retrieve Bundle IDs from Apple App Store Connect: %s", err.Error()),
		)
		return
	}

	totalCount := len(bundleIDs)
	tflog.Debug(ctx, "Retrieved Bundle IDs from Apple", map[string]interface{}{
		"total_count": totalCount,
	})

	// Parse filter options from configuration
	filterOptions, err := ParseFilterOptions(ctx, config)
	if err != nil {
		tflog.Error(ctx, "Failed to parse filter options", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Invalid Filter Configuration",
			fmt.Sprintf("Could not parse filter options: %s", err.Error()),
		)
		return
	}

	// Apply filters
	filteredBundleIDs, err := FilterBundleIDs(ctx, bundleIDs, filterOptions)
	if err != nil {
		tflog.Error(ctx, "Failed to filter Bundle IDs", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Filter Error",
			fmt.Sprintf("Could not apply filters to Bundle IDs: %s", err.Error()),
		)
		return
	}

	filteredCount := len(filteredBundleIDs)
	tflog.Info(ctx, "Applied filters to Bundle IDs", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"final_count":    len(filteredBundleIDs), // After limit
	})

	// Map Bundle IDs to model
	state := bundleIDsDataSourceModel{
		BundleIDs:     make([]bundleIDModel, 0, len(filteredBundleIDs)),
		TotalCount:    types.Int64Value(int64(totalCount)),
		FilteredCount: types.Int64Value(int64(filteredCount)),
		LastUpdated:   types.StringValue(time.Now().UTC().Format(time.RFC3339)),
	}

	// Copy filter configuration to state for reference
	state.Platform = config.Platform
	state.Platforms = config.Platforms
	state.IdentifierPattern = config.IdentifierPattern
	state.IdentifierPrefix = config.IdentifierPrefix
	state.NamePattern = config.NamePattern
	state.Limit = config.Limit
	state.SortBy = config.SortBy
	state.SortOrder = config.SortOrder

	for _, bundleID := range filteredBundleIDs {
		bundleIDModel := bundleIDModel{
			ID:         types.StringValue(bundleID.ID),
			Identifier: types.StringValue(bundleID.Attributes.Identifier),
			Name:       types.StringValue(bundleID.Attributes.Name),
			Platform:   types.StringValue(string(bundleID.Attributes.Platform)),
			SeedID:     types.StringValue(bundleID.Attributes.SeedID),
		}
		state.BundleIDs = append(state.BundleIDs, bundleIDModel)
	}

	tflog.Debug(ctx, "Bundle IDs data source read successfully", map[string]interface{}{
		"result_count": len(state.BundleIDs),
	})

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *bundleIDsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
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
