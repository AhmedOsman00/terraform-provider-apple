// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package profile

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

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
				MarkdownDescription: "Return only profiles issued for this Bundle ID, named either by " +
					"Apple's ID (an `apple_bundle_id`'s `id`) or by its identifier, such as " +
					"`com.example.app`. The profile record does not carry it, so this costs one request " +
					"per profile that passes the other filters.",
				Optional: true,
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
							MarkdownDescription: "The profile's name in the Developer Portal.",
							Computed:            true,
						},
						"platform": schema.StringAttribute{
							MarkdownDescription: "The platform for the Profile.",
							Computed:            true,
						},
						"bundle_id": schema.StringAttribute{
							MarkdownDescription: "Apple's ID of the Bundle ID this profile was issued for " +
								"(an `apple_bundle_id`'s `id`, not the `com.example.app` identifier). Read with " +
								"one request per profile returned.",
							Computed: true,
						},
						"certificates": schema.ListAttribute{
							MarkdownDescription: "Always empty: this data source does not read a profile's " +
								"certificates, which would cost a paginated request per profile on every plan. " +
								"Import the profile as an `apple_profile` to read them.",
							ElementType: types.StringType,
							Computed:    true,
						},
						"devices": schema.ListAttribute{
							MarkdownDescription: "Always empty: this data source does not read a profile's " +
								"devices, which would cost a paginated request per profile on every plan. " +
								"Import the profile as an `apple_profile` to read them.",
							ElementType: types.StringType,
							Computed:    true,
						},
						"profile_content": schema.StringAttribute{
							MarkdownDescription: "Base64-encoded profile content.",
							Computed:            true,
						},
						"uuid": schema.StringAttribute{
							MarkdownDescription: "The profile's UUID, which Xcode uses to refer to it.",
							Computed:            true,
						},
						"profile_state": schema.StringAttribute{
							MarkdownDescription: "The current state of the profile.",
							Computed:            true,
						},
						"profile_type": schema.StringAttribute{
							MarkdownDescription: "The profile type, such as `IOS_APP_STORE` or `IOS_APP_DEVELOPMENT`.",
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

	// The Bundle ID each profile was issued for, read at most once per profile:
	// by the filter for every candidate, and for the profiles returned.
	bundleIDs := map[string]string{}
	bundleIDOf := func(profileID string) (string, error) {
		if id, ok := bundleIDs[profileID]; ok {
			return id, nil
		}
		id, err := d.client.GetProfileBundleID(profileID)
		if err != nil {
			return "", err
		}
		bundleIDs[profileID] = id
		return id, nil
	}

	if !config.BundleID.IsNull() && !config.BundleID.IsUnknown() {
		filteredProfiles, err = d.filterByBundleID(config.BundleID.ValueString(), filteredProfiles, bundleIDOf)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Profile Bundle IDs",
				fmt.Sprintf("Could not filter profiles by bundle_id '%s': %s", config.BundleID.ValueString(), err.Error()),
			)
			return
		}
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

		// Certificates and devices are not read: each is a paginated request
		// per profile on every plan. See their descriptions.
		profileModel.Certificates, _ = types.ListValue(types.StringType, []attr.Value{})
		profileModel.Devices, _ = types.ListValue(types.StringType, []attr.Value{})

		bundleID, err := bundleIDOf(profile.ID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Profile Bundle ID",
				fmt.Sprintf("Could not read the Bundle ID of profile '%s': %s", profile.Attributes.Name, err.Error()),
			)
			return
		}
		profileModel.BundleID = types.StringValue(bundleID)

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

// filterByBundleID keeps the profiles issued for one Bundle ID, named by
// Apple's ID or by its identifier, such as com.example.app. An identifier no
// Bundle ID has matches nothing rather than failing, like any other filter.
func (d *profilesDataSource) filterByBundleID(wanted string, profiles []models.Profile, bundleIDOf func(string) (string, error)) ([]models.Profile, error) {
	if strings.Contains(wanted, ".") {
		bundle, err := d.client.GetBundleIDByIdentifier(wanted)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return []models.Profile{}, nil
			}
			return nil, err
		}
		wanted = bundle.ID
	}

	kept := make([]models.Profile, 0, len(profiles))
	for _, profile := range profiles {
		id, err := bundleIDOf(profile.ID)
		if err != nil {
			return nil, err
		}
		if id == wanted {
			kept = append(kept, profile)
		}
	}
	return kept, nil
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
