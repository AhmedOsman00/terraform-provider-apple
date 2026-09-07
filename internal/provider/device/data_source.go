// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package device

import (
	"context"
	"fmt"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &DevicesDataSource{}
	_ datasource.DataSourceWithConfigure = &DevicesDataSource{}
)

// DevicesDataSource defines the data source implementation.
type DevicesDataSource struct {
	client *apple.Client
}

// NewDevicesDataSource creates a new DevicesDataSource.
func NewDevicesDataSource() datasource.DataSource {
	return &DevicesDataSource{}
}

// Metadata returns the data source type name.
func (d *DevicesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_devices"
}

// Schema defines the schema for the data source.
func (d *DevicesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to retrieve information about Apple App Store Connect devices. " +
			"Devices represent iOS, macOS, tvOS, and visionOS devices registered for development and testing. " +
			"You can filter devices by platform, device class, status, and other attributes.",

		Attributes: map[string]schema.Attribute{
			// Filter attributes
			"platform": schema.StringAttribute{
				MarkdownDescription: "Filter devices by platform. Valid values: IOS, MAC_OS, TV_OS, VISION_OS",
				Optional:            true,
				Validators:          []validator.String{PlatformValidator},
			},
			"platforms": schema.ListAttribute{
				MarkdownDescription: "Filter devices by multiple platforms. Valid values: IOS, MAC_OS, TV_OS, VISION_OS",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(PlatformValidator),
					listvalidator.SizeAtMost(len(ValidPlatforms)),
				},
			},
			"device_class": schema.StringAttribute{
				MarkdownDescription: "Filter devices by device class. Valid values: IPHONE, IPAD, IPOD, APPLE_TV, APPLE_WATCH, MAC, APPLE_VISION_PRO",
				Optional:            true,
				Validators:          []validator.String{DeviceClassValidator},
			},
			"device_classes": schema.ListAttribute{
				MarkdownDescription: "Filter devices by multiple device classes. Valid values: IPHONE, IPAD, IPOD, APPLE_TV, APPLE_WATCH, MAC, APPLE_VISION_PRO",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(DeviceClassValidator),
					listvalidator.SizeAtMost(len(ValidDeviceClasses)),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Filter devices by status. Valid values: ENABLED, PROCESSING, INELIGIBLE",
				Optional:            true,
				Validators:          []validator.String{StatusValidator},
			},
			"name_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter devices by name using a regular expression pattern",
				Optional:            true,
				Validators:          GetPatternValidator(),
			},
			"udid_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter devices by UDID using a regular expression pattern",
				Optional:            true,
				Validators:          GetPatternValidator(),
			},

			// Result control attributes
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of devices to return (1-200). Defaults to no limit.",
				Optional:            true,
				Validators:          []validator.Int64{int64validator.Between(1, 200)},
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort by. Valid values: name, udid, platform, device_class, status, added_date",
				Optional:            true,
				Validators:          []validator.String{SortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort order. Valid values: asc, desc. Defaults to asc.",
				Optional:            true,
				Validators:          []validator.String{SortOrderValidator},
			},

			// Output attributes
			"devices": schema.ListNestedAttribute{
				MarkdownDescription: "List of devices matching the specified filters",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Unique identifier for the device",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Device name",
							Computed:            true,
						},
						"udid": schema.StringAttribute{
							MarkdownDescription: "Device UDID (Unique Device Identifier)",
							Computed:            true,
						},
						"platform": schema.StringAttribute{
							MarkdownDescription: "Device platform",
							Computed:            true,
						},
						"device_class": schema.StringAttribute{
							MarkdownDescription: "Device class",
							Computed:            true,
						},
						"model": schema.StringAttribute{
							MarkdownDescription: "Device model",
							Computed:            true,
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "Device status",
							Computed:            true,
						},
						"added_date": schema.StringAttribute{
							MarkdownDescription: "Date when the device was added (ISO 8601 format)",
							Computed:            true,
						},
					},
				},
			},

			// Metadata attributes
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of devices in the team",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of devices after applying filters",
				Computed:            true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *DevicesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *DevicesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Debug(ctx, "Reading devices data source")

	var data devicesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get all devices from the API
	devices, err := d.client.GetDevices()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read devices: %s", err))
		return
	}

	totalCount := len(devices)
	tflog.Debug(ctx, "Retrieved devices from API", map[string]interface{}{
		"total_count": totalCount,
	})

	// Apply filters and sorting
	filteredDevices, err := filterDevices(ctx, devices, data)
	if err != nil {
		resp.Diagnostics.AddError("Filter Error", fmt.Sprintf("Unable to filter devices: %s", err))
		return
	}

	filteredCount := len(filteredDevices)
	tflog.Debug(ctx, "Applied filters to devices", map[string]interface{}{
		"filtered_count": filteredCount,
	})

	// Convert to Terraform model
	deviceModels := make([]deviceModel, len(filteredDevices))
	for i, device := range filteredDevices {
		deviceModels[i] = deviceModel{
			ID:          types.StringValue(device.ID),
			Name:        types.StringValue(device.Attributes.Name),
			UDID:        types.StringValue(device.Attributes.UDID),
			Platform:    types.StringValue(string(device.Attributes.Platform)),
			DeviceClass: types.StringValue(string(device.Attributes.DeviceClass)),
			Status:      types.StringValue(string(device.Attributes.Status)),
		}

		// Set optional model field
		if device.Attributes.Model != nil {
			deviceModels[i].Model = types.StringValue(*device.Attributes.Model)
		} else {
			deviceModels[i].Model = types.StringNull()
		}

		// Set optional added date field
		if device.Attributes.AddedDate != nil {
			deviceModels[i].AddedDate = types.StringValue(device.Attributes.AddedDate.Format(time.RFC3339))
		} else {
			deviceModels[i].AddedDate = types.StringNull()
		}
	}

	// Update state
	data.Devices = deviceModels
	data.TotalCount = types.Int64Value(int64(totalCount))
	data.FilteredCount = types.Int64Value(int64(filteredCount))

	tflog.Debug(ctx, "Successfully populated devices data source", map[string]interface{}{
		"total_devices":    totalCount,
		"filtered_devices": filteredCount,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
