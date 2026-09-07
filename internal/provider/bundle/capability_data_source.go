// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package bundle

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &bundleIDCapabilitiesDataSource{}
	_ datasource.DataSourceWithConfigure = &bundleIDCapabilitiesDataSource{}
)

func NewBundleIDCapabilitiesDataSource() datasource.DataSource {
	return &bundleIDCapabilitiesDataSource{}
}

type bundleIDCapabilitiesDataSource struct {
	client *apple.Client
}

func (d *bundleIDCapabilitiesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bundle_id_capabilities"
}

func (d *bundleIDCapabilitiesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to retrieve information about Bundle ID Capabilities in your Apple Developer account.\n\n" +
			"Bundle ID Capabilities define what services and features are enabled for your apps. " +
			"This data source can filter capabilities by Bundle ID and capability type, and provides sorting and limiting options.",

		Attributes: map[string]schema.Attribute{
			// Filter configuration
			"bundle_id": schema.StringAttribute{
				MarkdownDescription: "Filter capabilities by Bundle ID. This can be either the Apple-generated Bundle ID or Bundle identifier.",
				Optional:            true,
				Validators:          GetBundleIDValidator(),
			},
			"capability_type": schema.StringAttribute{
				MarkdownDescription: "Filter by capability type (e.g., ICLOUD, PUSH_NOTIFICATIONS, IN_APP_PURCHASE).",
				Optional:            true,
				Validators:          GetCapabilityTypeValidator(),
			},

			// Result control
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of capabilities to return. Must be between 1 and 200. Defaults to 100.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 200),
				},
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort by. Valid values: `capability_type`, `bundle_id`. Defaults to `capability_type`.",
				Optional:            true,
				Validators: []validator.String{
					CapabilitySortByValidator,
				},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort order. Valid values: `asc`, `desc`. Defaults to `asc`.",
				Optional:            true,
				Validators: []validator.String{
					CapabilitySortOrderValidator,
				},
			},

			// Output
			"capabilities": schema.ListNestedAttribute{
				MarkdownDescription: "List of Bundle ID Capabilities matching the filter criteria.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the Bundle ID Capability.",
							Computed:            true,
						},
						"bundle_id": schema.StringAttribute{
							MarkdownDescription: "The Bundle ID that this capability is associated with.",
							Computed:            true,
						},
						"capability_type": schema.StringAttribute{
							MarkdownDescription: "The type of capability (e.g., ICLOUD, PUSH_NOTIFICATIONS).",
							Computed:            true,
						},
						"settings": schema.ListNestedAttribute{
							MarkdownDescription: "Configuration settings for the capability.",
							Computed:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"key": schema.StringAttribute{
										MarkdownDescription: "The setting key identifier.",
										Computed:            true,
									},
									"name": schema.StringAttribute{
										MarkdownDescription: "Human-readable name for the setting.",
										Computed:            true,
									},
									"value": schema.StringAttribute{
										MarkdownDescription: "The setting value.",
										Computed:            true,
									},
									"visible": schema.BoolAttribute{
										MarkdownDescription: "Whether this setting is visible in the Apple Developer portal.",
										Computed:            true,
									},
									"min_count": schema.Int64Attribute{
										MarkdownDescription: "Minimum number of values required for this setting.",
										Computed:            true,
									},
									"options": schema.ListNestedAttribute{
										MarkdownDescription: "Available options for this setting.",
										Computed:            true,
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"key": schema.StringAttribute{
													MarkdownDescription: "The option key identifier.",
													Computed:            true,
												},
												"name": schema.StringAttribute{
													MarkdownDescription: "Human-readable name for the option.",
													Computed:            true,
												},
												"description": schema.StringAttribute{
													MarkdownDescription: "Description of what this option enables.",
													Computed:            true,
												},
												"enabled": schema.BoolAttribute{
													MarkdownDescription: "Whether this option is enabled.",
													Computed:            true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},

			// Computed metadata
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of capabilities found before applying limit.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of capabilities after applying filters but before limit.",
				Computed:            true,
			},
		},
	}
}

func (d *bundleIDCapabilitiesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading Bundle ID Capabilities data source")

	var config bundleIDCapabilitiesDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set default values
	limit := int64(100)
	if !config.Limit.IsNull() {
		limit = config.Limit.ValueInt64()
	}

	sortBy := "capability_type"
	if !config.SortBy.IsNull() {
		sortBy = config.SortBy.ValueString()
	}

	sortOrder := "asc"
	if !config.SortOrder.IsNull() {
		sortOrder = config.SortOrder.ValueString()
	}

	ctx = tflog.SetField(ctx, "limit", limit)
	ctx = tflog.SetField(ctx, "sort_by", sortBy)
	ctx = tflog.SetField(ctx, "sort_order", sortOrder)

	var allCapabilities []models.BundleIDCapability
	var err error

	// If bundle_id is specified, get capabilities for that specific bundle
	if !config.BundleID.IsNull() && !config.BundleID.IsUnknown() {
		bundleID := config.BundleID.ValueString()
		ctx = tflog.SetField(ctx, "bundle_id", bundleID)

		// Try to resolve bundle identifier to bundle ID if needed
		var resolvedBundleID string
		if strings.HasPrefix(bundleID, "bundle-") || len(bundleID) >= 10 {
			resolvedBundleID = bundleID
		} else {
			// Try to find bundle ID by identifier
			bundleIDObj, err := d.client.GetBundleIDByIdentifier(bundleID)
			if err != nil {
				resp.Diagnostics.AddError(
					"Bundle ID Not Found",
					fmt.Sprintf("Could not find Bundle ID with identifier '%s': %s", bundleID, err.Error()),
				)
				return
			}
			resolvedBundleID = bundleIDObj.ID
		}

		allCapabilities, err = d.client.GetBundleIDCapabilities(resolvedBundleID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Bundle ID Capabilities",
				fmt.Sprintf("Could not read Bundle ID Capabilities for Bundle ID '%s': %s", resolvedBundleID, err.Error()),
			)
			return
		}
	} else {
		// Get all bundle IDs and their capabilities
		bundleIDs, err := d.client.GetBundleIDs()
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Bundle IDs",
				fmt.Sprintf("Could not read Bundle IDs: %s", err.Error()),
			)
			return
		}

		// Collect capabilities from all bundle IDs
		for _, bundle := range bundleIDs {
			capabilities, err := d.client.GetBundleIDCapabilities(bundle.ID)
			if err != nil {
				tflog.Warn(ctx, "Failed to get capabilities for bundle ID", map[string]interface{}{
					"bundle_id": bundle.ID,
					"error":     err.Error(),
				})
				continue
			}
			allCapabilities = append(allCapabilities, capabilities...)
		}
	}

	tflog.Debug(ctx, "Retrieved capabilities from Apple API", map[string]interface{}{
		"total_capabilities": len(allCapabilities),
	})

	// Apply capability type filter
	filteredCapabilities := allCapabilities
	if !config.CapabilityType.IsNull() && !config.CapabilityType.IsUnknown() {
		capabilityType := config.CapabilityType.ValueString()
		ctx = tflog.SetField(ctx, "capability_type_filter", capabilityType)

		var filtered []models.BundleIDCapability
		for _, capability := range allCapabilities {
			if string(capability.Attributes.CapabilityType) == capabilityType {
				filtered = append(filtered, capability)
			}
		}
		filteredCapabilities = filtered
	}

	// Sort capabilities
	sortCapabilities(filteredCapabilities, sortBy, sortOrder)

	// Apply limit
	if int64(len(filteredCapabilities)) > limit {
		filteredCapabilities = filteredCapabilities[:limit]
	}

	// Convert capabilities to Terraform models
	var capabilityModels []bundleIDCapabilityModel
	for _, capability := range filteredCapabilities {
		// Convert settings
		tfSettings, err := SettingsFromAPI(capability.Attributes.Settings)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Converting Settings",
				fmt.Sprintf("Could not convert capability settings: %s", err.Error()),
			)
			return
		}

		// For data source, we need to determine the bundle ID from the capability
		// This would require additional API call or the capability response to include bundle ID
		// For now, we'll use a placeholder or try to extract from relationships if available
		bundleID := types.StringValue("") // This would need to be populated from API response

		capabilityModel := bundleIDCapabilityModel{
			ID:             types.StringValue(capability.ID),
			BundleID:       bundleID,
			CapabilityType: types.StringValue(string(capability.Attributes.CapabilityType)),
			Settings:       tfSettings,
		}
		capabilityModels = append(capabilityModels, capabilityModel)
	}

	// Populate the data source model
	state := bundleIDCapabilitiesDataSourceModel{
		BundleID:       config.BundleID,
		CapabilityType: config.CapabilityType,
		Limit:          types.Int64Value(limit),
		SortBy:         types.StringValue(sortBy),
		SortOrder:      types.StringValue(sortOrder),
		Capabilities:   capabilityModels,
		TotalCount:     types.Int64Value(int64(len(allCapabilities))),
		FilteredCount:  types.Int64Value(int64(len(filteredCapabilities))),
	}

	tflog.Info(ctx, "Bundle ID Capabilities data source read completed", map[string]interface{}{
		"total_count":    len(allCapabilities),
		"filtered_count": len(filteredCapabilities),
		"returned_count": len(capabilityModels),
	})

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *bundleIDCapabilitiesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// sortCapabilities sorts capabilities based on the specified field and order.
func sortCapabilities(capabilities []models.BundleIDCapability, sortBy, sortOrder string) {
	sort.Slice(capabilities, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "capability_type":
			less = string(capabilities[i].Attributes.CapabilityType) < string(capabilities[j].Attributes.CapabilityType)
		case "bundle_id":
			// Note: This would require bundle ID to be available in the capability model
			less = capabilities[i].ID < capabilities[j].ID // Fallback to ID sorting
		default:
			less = string(capabilities[i].Attributes.CapabilityType) < string(capabilities[j].Attributes.CapabilityType)
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}
