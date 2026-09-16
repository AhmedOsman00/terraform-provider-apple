// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

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
	_ datasource.DataSource              = &appStoreVersionsDataSource{}
	_ datasource.DataSourceWithConfigure = &appStoreVersionsDataSource{}
)

func NewAppStoreVersionsDataSource() datasource.DataSource {
	return &appStoreVersionsDataSource{}
}

type appStoreVersionsDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *appStoreVersionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_store_versions"
}

// Schema defines the schema for the data source.
func (d *appStoreVersionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists an app's App Store versions.\n\n" +
			"An app accumulates a version record per release for its whole life, one per platform, so " +
			"`platform` is usually worth setting — it is passed to Apple rather than applied in " +
			"memory.\n\n" +
			"The common use is finding the version currently being prepared, so that a localization or " +
			"review detail can be attached to a release this configuration did not create:\n\n" +
			"```terraform\n" +
			"data \"apple_app_store_versions\" \"editable\" {\n" +
			"  app_id            = data.apple_apps.mine.apps[0].id\n" +
			"  platform          = \"IOS\"\n" +
			"  app_version_state = \"PREPARE_FOR_SUBMISSION\"\n" +
			"}\n" +
			"```",

		Attributes: map[string]schema.Attribute{
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app whose versions to list.",
				Required:            true,
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "Return only versions for this platform: `IOS`, `MAC_OS`, `TV_OS` " +
					"or `VISION_OS`. Applied by Apple, not in memory.",
				Optional:   true,
				Validators: []validator.String{AppStorePlatformValidator},
			},
			"app_version_state": schema.StringAttribute{
				MarkdownDescription: "Return only versions in this state, for example " +
					"`PREPARE_FOR_SUBMISSION`, `IN_REVIEW` or `READY_FOR_DISTRIBUTION`. Matched exactly, " +
					"case-insensitively.",
				Optional:   true,
				Validators: []validator.String{AppVersionStateValidator},
			},
			"version_string_pattern": schema.StringAttribute{
				MarkdownDescription: "Return only versions whose version string matches this regular " +
					"expression, matched as a substring unless anchored — `^2\\.` for the 2.x releases.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of versions to return. Applied after filtering and " +
					"sorting.",
				Optional: true,
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort by: `version_string`, `created_date`, " +
					"`app_version_state` or `platform`. Sort by `created_date` with `sort_order` " +
					"`desc` for the most recent release first — `version_string` is compared as text, " +
					"so `1.10` sorts before `1.9`.",
				Optional:   true,
				Validators: []validator.String{VersionSortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort direction: `asc` or `desc`. Defaults to `asc`.",
				Optional:            true,
				Validators:          []validator.String{SortOrderValidator},
			},
			"versions": schema.ListNestedAttribute{
				MarkdownDescription: "The versions matching the filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the version.",
							Computed:            true,
						},
						"app_id": schema.StringAttribute{
							MarkdownDescription: "The Apple ID of the app this version belongs to.",
							Computed:            true,
						},
						"platform": schema.StringAttribute{
							MarkdownDescription: "The platform this version targets.",
							Computed:            true,
						},
						"version_string": schema.StringAttribute{
							MarkdownDescription: "The version number customers see.",
							Computed:            true,
						},
						"copyright": schema.StringAttribute{
							MarkdownDescription: "The copyright line shown on the product page.",
							Computed:            true,
						},
						"release_type": schema.StringAttribute{
							MarkdownDescription: "How the version is released once approved: `MANUAL`, " +
								"`AFTER_APPROVAL` or `SCHEDULED`.",
							Computed: true,
						},
						"earliest_release_date": schema.StringAttribute{
							MarkdownDescription: "When a `SCHEDULED` release goes live.",
							Computed:            true,
						},
						"review_type": schema.StringAttribute{
							MarkdownDescription: "Which review queue the version goes through.",
							Computed:            true,
						},
						"uses_idfa": schema.BoolAttribute{
							MarkdownDescription: "Whether the version uses the Advertising Identifier.",
							Computed:            true,
						},
						"app_version_state": schema.StringAttribute{
							MarkdownDescription: "Where the version stands.",
							Computed:            true,
						},
						"downloadable": schema.BoolAttribute{
							MarkdownDescription: "Whether the version is currently downloadable.",
							Computed:            true,
						},
						"created_date": schema.StringAttribute{
							MarkdownDescription: "When Apple created the version record.",
							Computed:            true,
						},
					},
				},
			},
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Number of versions Apple returned before the in-memory filters.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of versions remaining after filtering, before the limit.",
				Computed:            true,
			},
		},
	}
}

// Read fetches, filters and sorts the versions.
func (d *appStoreVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading app store versions data source")

	var config appStoreVersionsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := config.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	versions, err := d.client.GetAppStoreVersions(appID, config.Platform.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read App Store Versions",
			fmt.Sprintf("Could not retrieve versions of app '%s': %s", appID, err.Error()),
		)
		return
	}

	totalCount := len(versions)

	filtered, err := FilterAppStoreVersions(versions, AppStoreVersionFilters{
		AppVersionState:      config.AppVersionState.ValueString(),
		VersionStringPattern: config.VersionStringPattern.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid App Store Version Filter", err.Error())
		return
	}

	filteredCount := len(filtered)

	SortAppStoreVersions(filtered, config.SortBy.ValueString(), config.SortOrder.ValueString())
	filtered = LimitAppStoreVersions(filtered, int(config.Limit.ValueInt64()))

	versionModels := make([]appStoreVersionModel, len(filtered))
	for i := range filtered {
		model := appStoreVersionModel{
			AppID:               types.StringValue(appID),
			EarliestReleaseDate: stringOrNull(filtered[i].Attributes.EarliestReleaseDate),
			UsesIdfa:            boolOrNull(filtered[i].Attributes.UsesIdfa),
		}
		applyAppStoreVersion(&model, &filtered[i])
		versionModels[i] = model
	}

	config.Versions = versionModels
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Info(ctx, "App store versions data source read completed", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"returned_count": len(versionModels),
	})

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *appStoreVersionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
