// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

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
	_ datasource.DataSource              = &appCategoriesDataSource{}
	_ datasource.DataSourceWithConfigure = &appCategoriesDataSource{}
)

func NewAppCategoriesDataSource() datasource.DataSource {
	return &appCategoriesDataSource{}
}

type appCategoriesDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *appCategoriesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_categories"
}

// Schema defines the schema for the data source.
func (d *appCategoriesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads Apple's App Store category catalogue.\n\n" +
			"Categories are fixed and the same for every account, like territories. A category's ID is " +
			"the constant itself — `FINANCE`, `PRODUCTIVITY`, `GAMES_PUZZLE` — which is what " +
			"`apple_app_info` takes, so this data source exists to discover the valid values and which " +
			"platforms accept them rather than to look an ID up at apply time.\n\n" +
			"Apple returns categories and subcategories in one collection; a subcategory is the one " +
			"with a `parent_id`. Only a top-level category can be an app's primary or secondary " +
			"category, so set `top_level_only` when that is what you are choosing.",

		Attributes: map[string]schema.Attribute{
			"platforms": schema.ListAttribute{
				MarkdownDescription: "Restrict the catalogue to categories available on these platforms: " +
					"`IOS`, `MAC_OS`, `TV_OS` or `VISION_OS`. Applied by Apple, not in memory.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"top_level_only": schema.BoolAttribute{
				MarkdownDescription: "Return only categories that can be assigned as a primary or " +
					"secondary category, dropping the subcategories.",
				Optional: true,
			},
			"id_pattern": schema.StringAttribute{
				MarkdownDescription: "Return only categories whose ID matches this regular expression, " +
					"matched as a substring unless anchored — `^GAMES` for the games categories.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of categories to return. Applied after filtering.",
				Optional:            true,
			},
			"categories": schema.ListNestedAttribute{
				MarkdownDescription: "The categories matching the filters, ordered by ID.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The category constant, to be passed to `apple_app_info`.",
							Computed:            true,
						},
						"platforms": schema.ListAttribute{
							MarkdownDescription: "The platforms this category is offered on.",
							Computed:            true,
							ElementType:         types.StringType,
						},
						"parent_id": schema.StringAttribute{
							MarkdownDescription: "The category this one is a subcategory of, or null for " +
								"a top-level category.",
							Computed: true,
						},
						"subcategories": schema.ListAttribute{
							MarkdownDescription: "The IDs of this category's subcategories. Most " +
								"categories have none.",
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Number of categories Apple returned before the in-memory filters.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of categories remaining after filtering, before the limit.",
				Computed:            true,
			},
		},
	}
}

// Read fetches and filters the category catalogue.
func (d *appCategoriesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading app categories data source")

	var config appCategoriesDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	categories, err := d.client.GetAppCategories(stringListValues(config.Platforms))
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read App Categories",
			fmt.Sprintf("Could not retrieve Apple's category catalogue: %s", err.Error()),
		)
		return
	}

	totalCount := len(categories)

	filtered, err := FilterAppCategories(categories, AppCategoryFilters{
		TopLevelOnly: config.TopLevelOnly.ValueBool(),
		IDPattern:    config.IDPattern.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid App Category Filter", err.Error())
		return
	}

	SortAppCategories(filtered)
	filteredCount := len(filtered)

	if limit := int(config.Limit.ValueInt64()); limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	categoryModels := make([]appCategoryModel, len(filtered))
	for i, category := range filtered {
		model := appCategoryModel{
			ID:       types.StringValue(category.ID),
			ParentID: types.StringNull(),
		}

		model.Platforms = make([]types.String, 0, len(category.Attributes.Platforms))
		for _, platform := range category.Attributes.Platforms {
			model.Platforms = append(model.Platforms, types.StringValue(platform))
		}

		if category.Relationships != nil {
			model.ParentID = relationshipID(category.Relationships.Parent)

			if category.Relationships.Subcategories != nil {
				model.Subcategories = make([]types.String, 0, len(category.Relationships.Subcategories.Data))
				for _, sub := range category.Relationships.Subcategories.Data {
					model.Subcategories = append(model.Subcategories, types.StringValue(sub.ID))
				}
			}
		}

		categoryModels[i] = model
	}

	config.Categories = categoryModels
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Info(ctx, "App categories data source read completed", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"returned_count": len(categoryModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *appCategoriesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
