// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package profile

import (
	"context"
	"fmt"
	"terraform-provider-apple/internal/apple"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &profilesDataSource{}
	_ datasource.DataSourceWithConfigure = &profilesDataSource{}
)

func NewProfilesDataSource() datasource.DataSource {
	return &profilesDataSource{}
}

// profilesDataSource is the data source implementation.
type profilesDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *profilesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profiles"
}

// Schema defines the schema for the data source.
func (d *profilesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about Provisioning Profiles from Apple App Store Connect.\n\n" +
			"This data source allows you to query and filter Provisioning Profiles based on various criteria such as " +
			"platform, name patterns, profile state, and profile type. It supports sorting and limiting results.",

		Attributes: map[string]schema.Attribute{
			// Filter attributes
			"platform": schema.StringAttribute{
				MarkdownDescription: "Filter Profiles by platform. Valid values are:\n" +
					"- `IOS` - iOS platform\n" +
					"- `MAC_OS` - macOS platform\n" +
					"- `TV_OS` - tvOS platform\n" +
					"- `WATCH_OS` - watchOS platform\n\n" +
					"Cannot be used together with `platforms`.",
				Optional:   true,
				Validators: []validator.String{PlatformValidator},
			},
			"platforms": schema.ListAttribute{
				MarkdownDescription: "Filter Profiles by multiple platforms. Accepts a list of platform values.\n" +
					"Cannot be used together with `platform`.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"name_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter Profiles by name using a regular expression pattern. " +
					"For example, `.*Development.*` to match all Profiles containing 'Development' in the name.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"profile_state": schema.StringAttribute{
				MarkdownDescription: "Filter Profiles by state. Valid values are:\n" +
					"- `ACTIVE` - Active profiles\n" +
					"- `INVALID` - Invalid profiles\n" +
					"- `EXPIRED` - Expired profiles",
				Optional:   true,
				Validators: []validator.String{ProfileStateValidator},
			},
			"profile_type": schema.StringAttribute{
				MarkdownDescription: "Filter Profiles by type. Valid values are:\n" +
					"- `IOS_APP_DEVELOPMENT` - iOS App Development\n" +
					"- `IOS_APP_ADHOC` - iOS Ad Hoc\n" +
					"- `IOS_APP_STORE` - iOS App Store\n" +
					"- `IOS_APP_INHOUSE` - iOS In House\n" +
					"- `MAC_APP_DEVELOPMENT` - macOS App Development\n" +
					"- `MAC_APP_STORE` - macOS App Store\n" +
					"- `MAC_APP_DIRECT` - macOS Direct Distribution\n" +
					"- `TVOS_APP_DEVELOPMENT` - tvOS App Development\n" +
					"- `TVOS_APP_ADHOC` - tvOS Ad Hoc\n" +
					"- `TVOS_APP_STORE` - tvOS App Store\n" +
					"- `TVOS_APP_INHOUSE` - tvOS In House",
				Optional:   true,
				Validators: []validator.String{ProfileTypeValidator},
			},
			"bundle_id": schema.StringAttribute{
				MarkdownDescription: "Filter Profiles by the associated Bundle ID.",
				Optional:            true,
			},

			// Result control attributes
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of Profiles to return. Must be between 1 and 200.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 200),
				},
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort Profiles by. Valid values are:\n" +
					"- `name` - Sort by Profile name\n" +
					"- `platform` - Sort by platform\n" +
					"- `profile_state` - Sort by profile state\n" +
					"- `profile_type` - Sort by profile type\n" +
					"- `created_date` - Sort by creation date\n" +
					"- `expiration_date` - Sort by expiration date\n\n" +
					"Defaults to `name` if not specified.",
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
			"profiles": schema.ListNestedAttribute{
				MarkdownDescription: "List of Profiles matching the filter criteria.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the Profile.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the Profile.",
							Computed:            true,
						},
						"platform": schema.StringAttribute{
							MarkdownDescription: "The platform for the Profile.",
							Computed:            true,
						},
						"bundle_id": schema.StringAttribute{
							MarkdownDescription: "The Bundle ID associated with this profile.",
							Computed:            true,
						},
						"certificates": schema.ListAttribute{
							MarkdownDescription: "List of certificate IDs included in the profile.",
							ElementType:         types.StringType,
							Computed:            true,
						},
						"devices": schema.ListAttribute{
							MarkdownDescription: "List of device IDs included in the profile.",
							ElementType:         types.StringType,
							Computed:            true,
						},
						"profile_content": schema.StringAttribute{
							MarkdownDescription: "Base64-encoded profile content.",
							Computed:            true,
						},
						"uuid": schema.StringAttribute{
							MarkdownDescription: "The UUID of the profile.",
							Computed:            true,
						},
						"profile_state": schema.StringAttribute{
							MarkdownDescription: "The current state of the profile.",
							Computed:            true,
						},
						"profile_type": schema.StringAttribute{
							MarkdownDescription: "The type of profile.",
							Computed:            true,
						},
						"created_date": schema.StringAttribute{
							MarkdownDescription: "The date and time when the profile was created.",
							Computed:            true,
						},
						"expiration_date": schema.StringAttribute{
							MarkdownDescription: "The date and time when the profile expires.",
							Computed:            true,
						},
					},
				},
			},

			// Computed metadata
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of Profiles available (before filtering).",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of Profiles after applying filters.",
				Computed:            true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *profilesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading Profiles data source")

	var config profilesDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get Profiles from Apple API
	profiles, err := d.client.GetProfiles()
	if err != nil {
		tflog.Error(ctx, "Failed to get profiles from Apple API", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Unable to Read Apple Profiles",
			fmt.Sprintf("An error occurred while retrieving Profiles from Apple: %s", err.Error()),
		)
		return
	}

	totalCount := len(profiles)
	tflog.Debug(ctx, "Retrieved profiles from Apple API", map[string]interface{}{
		"total_count": totalCount,
	})

	// Apply filters
	filteredProfiles, err := d.applyFilters(ctx, profiles, config)
	if err != nil {
		resp.Diagnostics.AddError(
			"Filter Error",
			fmt.Sprintf("An error occurred while filtering profiles: %s", err.Error()),
		)
		return
	}

	filteredCount := len(filteredProfiles)
	tflog.Debug(ctx, "Applied filters to profiles", map[string]interface{}{
		"filtered_count": filteredCount,
	})

	// Apply sorting
	sortedProfiles := d.applySorting(ctx, filteredProfiles, config)

	// Apply limit
	limitedProfiles := d.applyLimit(ctx, sortedProfiles, config)

	finalCount := len(limitedProfiles)
	tflog.Debug(ctx, "Applied sorting and limit to profiles", map[string]interface{}{
		"final_count": finalCount,
	})

	// Convert to Terraform model
	profileModels := make([]profileModel, len(limitedProfiles))
	for i, profile := range limitedProfiles {
		profileModel := profileModel{
			ID:       types.StringValue(profile.ID),
			Name:     types.StringValue(profile.Attributes.Name),
			Platform: types.StringValue(string(profile.Attributes.Platform)),
		}

		// Set optional computed attributes
		if profile.Attributes.ProfileContent != nil {
			profileModel.ProfileContent = types.StringValue(*profile.Attributes.ProfileContent)
		}
		if profile.Attributes.UUID != nil {
			profileModel.UUID = types.StringValue(*profile.Attributes.UUID)
		}
		if profile.Attributes.ProfileState != nil {
			profileModel.ProfileState = types.StringValue(string(*profile.Attributes.ProfileState))
		}
		if profile.Attributes.ProfileType != nil {
			profileModel.ProfileType = types.StringValue(string(*profile.Attributes.ProfileType))
		}
		if profile.Attributes.CreatedDate != nil {
			profileModel.CreatedDate = types.StringValue(profile.Attributes.CreatedDate.Format(time.RFC3339))
		}
		if profile.Attributes.ExpirationDate != nil {
			profileModel.ExpirationDate = types.StringValue(profile.Attributes.ExpirationDate.Format(time.RFC3339))
		}

		// For data source, we set empty lists for relationships as they're not typically included
		profileModel.Certificates, _ = types.ListValue(types.StringType, []attr.Value{})
		profileModel.Devices, _ = types.ListValue(types.StringType, []attr.Value{})
		profileModel.BundleID = types.StringValue("")

		profileModels[i] = profileModel
	}

	// Set computed metadata
	config.Profiles = profileModels
	config.TotalCount = types.Int64Value(int64(totalCount))
	config.FilteredCount = types.Int64Value(int64(finalCount))

	tflog.Info(ctx, "Profiles data source read completed successfully", map[string]interface{}{
		"total_profiles":    totalCount,
		"filtered_profiles": finalCount,
	})

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// Configure adds the provider configured client to the data source.
func (d *profilesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
