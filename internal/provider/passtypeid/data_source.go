// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package passtypeid

import (
	"context"
	"fmt"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &passTypeIDsDataSource{}
	_ datasource.DataSourceWithConfigure = &passTypeIDsDataSource{}
)

func NewPassTypeIDsDataSource() datasource.DataSource {
	return &passTypeIDsDataSource{}
}

// passTypeIDsDataSource is the data source implementation.
type passTypeIDsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *passTypeIDsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pass_type_ids"
}

// Schema defines the schema for the data source.
func (d *passTypeIDsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about Pass Type IDs from Apple App Store Connect.\n\n" +
			"This data source allows you to query and filter Pass Type IDs based on various criteria such as " +
			"identifier patterns, identifier prefixes, and name patterns. It supports sorting and limiting results.",

		Attributes: map[string]schema.Attribute{
			// Filter attributes
			"identifier_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter Pass Type IDs by identifier using a regular expression pattern. " +
					"For example, `^pass\\.com\\.example\\.*` to match all Pass Type IDs starting with `pass.com.example.`.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"identifier_prefix": schema.StringAttribute{
				MarkdownDescription: "Filter Pass Type IDs by identifier prefix. " +
					"For example, `pass.com.example` to match all Pass Type IDs starting with that prefix.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"name_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter Pass Type IDs by name using a regular expression pattern. " +
					"For example, `.*MyPass.*` to match Pass Type IDs containing 'MyPass' in the name.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			// Result control attributes
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of Pass Type IDs to return. Must be between 1 and 200.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 200),
				},
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort Pass Type IDs by. Valid values are:\n" +
					"- `name` - Sort by Pass Type ID name\n" +
					"- `identifier` - Sort by Pass Type ID identifier\n\n" +
					"Defaults to `identifier` if not specified.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("name", "identifier"),
				},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort order for the results. Valid values are:\n" +
					"- `asc` - Ascending order\n" +
					"- `desc` - Descending order\n\n" +
					"Defaults to `asc` if not specified.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("asc", "desc"),
				},
			},

			// Output attributes
			"pass_type_ids": schema.ListNestedAttribute{
				MarkdownDescription: "List of Pass Type IDs matching the filter criteria.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the Pass Type ID.",
							Computed:            true,
						},
						"identifier": schema.StringAttribute{
							MarkdownDescription: "The Pass Type ID identifier string (e.g., `pass.com.example.mypass`).",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The human-readable name for the Pass Type ID.",
							Computed:            true,
						},
					},
				},
			},

			// Computed metadata
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of Pass Type IDs available (before filtering).",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of Pass Type IDs after applying filters (before limiting).",
				Computed:            true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *passTypeIDsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read refreshes the Terraform state with the latest data.
func (d *passTypeIDsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading Pass Type IDs data source")

	var config passTypeIDsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Fetching Pass Type IDs from Apple API")

	// Get Pass Type IDs from Apple API
	passTypeIDs, err := d.client.GetPassTypeIDs()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Pass Type IDs",
			fmt.Sprintf("An error occurred while reading Pass Type IDs from Apple API: %s", err.Error()),
		)
		return
	}

	originalCount := len(passTypeIDs)
	tflog.Debug(ctx, "Retrieved Pass Type IDs from Apple API", map[string]interface{}{
		"total_count": originalCount,
	})

	// Apply filtering
	var identifierPattern, identifierPrefix, namePattern string
	if !config.IdentifierPattern.IsNull() {
		identifierPattern = config.IdentifierPattern.ValueString()
	}
	if !config.IdentifierPrefix.IsNull() {
		identifierPrefix = config.IdentifierPrefix.ValueString()
	}
	if !config.NamePattern.IsNull() {
		namePattern = config.NamePattern.ValueString()
	}

	filteredPassTypeIDs, err := FilterPassTypeIDs(passTypeIDs, identifierPattern, identifierPrefix, namePattern)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Pass Type ID Filter",
			err.Error(),
		)
		return
	}
	filteredCount := len(filteredPassTypeIDs)

	tflog.Debug(ctx, "Applied filtering to Pass Type IDs", map[string]interface{}{
		"filtered_count":     filteredCount,
		"identifier_pattern": identifierPattern,
		"identifier_prefix":  identifierPrefix,
		"name_pattern":       namePattern,
	})

	// Apply sorting
	var sortBy, sortOrder string
	if !config.SortBy.IsNull() {
		sortBy = config.SortBy.ValueString()
	}
	if !config.SortOrder.IsNull() {
		sortOrder = config.SortOrder.ValueString()
	}

	sortedPassTypeIDs := SortPassTypeIDs(filteredPassTypeIDs, sortBy, sortOrder)

	// Apply limiting
	var limit int
	if !config.Limit.IsNull() {
		limit = int(config.Limit.ValueInt64())
	}

	limitedPassTypeIDs := LimitPassTypeIDs(sortedPassTypeIDs, limit)

	tflog.Debug(ctx, "Applied sorting and limiting to Pass Type IDs", map[string]interface{}{
		"final_count": len(limitedPassTypeIDs),
		"sort_by":     sortBy,
		"sort_order":  sortOrder,
		"limit":       limit,
	})

	// Convert to Terraform model
	var passTypeIDModels []passTypeIDModel
	for _, passTypeID := range limitedPassTypeIDs {
		passTypeIDModel := passTypeIDModel{
			ID:         types.StringValue(passTypeID.ID),
			Identifier: types.StringValue(passTypeID.Attributes.Identifier),
			Name:       types.StringValue(passTypeID.Attributes.Name),
		}
		passTypeIDModels = append(passTypeIDModels, passTypeIDModel)
	}

	// Set the computed values
	state := passTypeIDsDataSourceModel{
		IdentifierPattern: config.IdentifierPattern,
		IdentifierPrefix:  config.IdentifierPrefix,
		NamePattern:       config.NamePattern,
		Limit:             config.Limit,
		SortBy:            config.SortBy,
		SortOrder:         config.SortOrder,
		PassTypeIDs:       passTypeIDModels,
		TotalCount:        types.Int64Value(int64(originalCount)),
		FilteredCount:     types.Int64Value(int64(filteredCount)),
	}

	tflog.Info(ctx, "Pass Type IDs data source read successfully", map[string]interface{}{
		"total_count":    originalCount,
		"filtered_count": filteredCount,
		"final_count":    len(limitedPassTypeIDs),
	})

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
