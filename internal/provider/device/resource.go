// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package device

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"terraform-provider-apple/internal/apple"
	"terraform-provider-apple/internal/apple/models"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &DeviceResource{}
	_ resource.ResourceWithConfigure   = &DeviceResource{}
	_ resource.ResourceWithImportState = &DeviceResource{}
)

// DeviceResource defines the resource implementation.
type DeviceResource struct {
	client *apple.Client
}

// NewDeviceResource creates a new DeviceResource.
func NewDeviceResource() resource.Resource {
	return &DeviceResource{}
}

// Metadata returns the resource type name.
func (r *DeviceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_device"
}

// Schema defines the schema for the resource.
func (r *DeviceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a device registered with Apple App Store Connect. " +
			"Devices represent iOS, macOS, tvOS, and visionOS devices that can be used for development and testing. " +
			"Each device is identified by its UDID (Unique Device Identifier).\n\n" +
			"~> **Note:** The App Store Connect API cannot delete devices. Destroying this resource disables the " +
			"device instead and removes it from Terraform state; it remains listed in your Apple Developer account.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the device assigned by Apple",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name for the device",
				Required:            true,
				Validators:          GetNameValidator(),
			},
			"udid": schema.StringAttribute{
				MarkdownDescription: "Device UDID (Unique Device Identifier). This uniquely identifies the device and cannot be changed. " +
					"The format varies by device -- a 40-character hex string on iPhone X and earlier, " +
					"an 8-16 hex string such as `00008030-000A4D8E0AB8802E` on iPhone XS and later, " +
					"a UUID on Mac, Apple TV and Vision Pro -- so the exact shape is validated by Apple rather than here. " +
					"Report the UDID exactly as the device gives it.",
				Required:   true,
				Validators: GetUDIDValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "Device platform. Valid values: IOS, MAC_OS, TV_OS, VISION_OS. Cannot be changed after creation.",
				Required:            true,
				Validators:          []validator.String{PlatformValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"device_class": schema.StringAttribute{
				MarkdownDescription: "Device class (automatically determined by Apple)",
				Computed:            true,
			},
			"model": schema.StringAttribute{
				MarkdownDescription: "Device model (automatically determined by Apple)",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Device status (automatically determined by Apple)",
				Computed:            true,
			},
			"added_date": schema.StringAttribute{
				MarkdownDescription: "Date when the device was added (ISO 8601 format)",
				Computed:            true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *DeviceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apple.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *apple.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create creates a new device resource.
func (r *DeviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating device resource")

	var data deviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	udid := data.UDID.ValueString()
	platform := models.DevicePlatform(data.Platform.ValueString())

	tflog.Debug(ctx, "Creating device with API", map[string]interface{}{
		"name":     name,
		"udid":     udid,
		"platform": string(platform),
	})

	// Create the device
	device, err := r.client.CreateDevice(name, udid, platform, nil)
	if err != nil {
		if strings.Contains(err.Error(), "409") {
			resp.Diagnostics.AddError(
				"Device Already Exists",
				fmt.Sprintf("A device with UDID '%s' already exists. Device UDIDs must be unique.", udid),
			)
			return
		}
		if strings.Contains(err.Error(), "400") {
			resp.Diagnostics.AddError(
				"Invalid Device Data",
				fmt.Sprintf("The provided device data is invalid. Please check the UDID format and platform. Error: %s", err),
			)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create device: %s", err))
		return
	}

	tflog.Debug(ctx, "Successfully created device", map[string]interface{}{
		"device_id": device.ID,
		"name":      device.Attributes.Name,
		"udid":      device.Attributes.UDID,
		"platform":  string(device.Attributes.Platform),
	})

	// Map response back to schema
	r.mapDeviceToModel(device, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *DeviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading device resource")

	var data deviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deviceID := data.ID.ValueString()
	tflog.Debug(ctx, "Reading device from API", map[string]interface{}{
		"device_id": deviceID,
	})

	device, err := r.client.GetDevice(deviceID)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			tflog.Warn(ctx, "Device not found, removing from state", map[string]interface{}{
				"device_id": deviceID,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read device: %s", err))
		return
	}

	tflog.Debug(ctx, "Successfully read device", map[string]interface{}{
		"device_id": device.ID,
		"name":      device.Attributes.Name,
		"udid":      device.Attributes.UDID,
		"status":    string(device.Attributes.Status),
	})

	// Map response back to schema
	r.mapDeviceToModel(device, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the device resource.
func (r *DeviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Debug(ctx, "Updating device resource")

	var data deviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state deviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deviceID := state.ID.ValueString()
	name := data.Name.ValueString()

	tflog.Debug(ctx, "Updating device with API", map[string]interface{}{
		"device_id": deviceID,
		"name":      name,
	})

	// Update the device
	device, err := r.client.UpdateDevice(deviceID, name, nil, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			resp.Diagnostics.AddError(
				"Device Not Found",
				fmt.Sprintf("Device with ID '%s' not found. It may have been deleted outside of Terraform.", deviceID),
			)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update device: %s", err))
		return
	}

	tflog.Debug(ctx, "Successfully updated device", map[string]interface{}{
		"device_id": device.ID,
		"name":      device.Attributes.Name,
	})

	// Map response back to schema
	r.mapDeviceToModel(device, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the device resource.
func (r *DeviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "Deleting device resource")

	var data deviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The App Store Connect API has no endpoint for deleting a device. The
	// supported equivalent is disabling it, which releases the device from the
	// team's provisioning profiles. Erroring here instead would make the
	// resource impossible to destroy and permanently stuck in state.
	deviceID := data.ID.ValueString()
	disabled := models.DeviceStatusDISABLED

	tflog.Debug(ctx, "Disabling device in place of deletion", map[string]interface{}{
		"device_id": deviceID,
		"udid":      data.UDID.ValueString(),
	})

	if _, err := r.client.UpdateDevice(deviceID, data.Name.ValueString(), &disabled, nil); err != nil {
		// Already gone from Apple's side: let the delete succeed so Terraform
		// drops it from state rather than wedging the resource.
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, "Device not found while disabling, treating as already removed", map[string]interface{}{
				"device_id": deviceID,
			})
			return
		}

		resp.Diagnostics.AddError(
			"Unable to Disable Device",
			fmt.Sprintf("Apple's API cannot delete devices, so Terraform disables them instead. Disabling device '%s' failed: %s", deviceID, err),
		)
		return
	}

	resp.Diagnostics.AddWarning(
		"Device Disabled Rather Than Deleted",
		fmt.Sprintf("The App Store Connect API cannot delete devices. Device '%s' (UDID %s) has been disabled and removed from Terraform state, "+
			"but it remains listed in your Apple Developer account.", data.Name.ValueString(), data.UDID.ValueString()),
	)

	tflog.Info(ctx, "Device disabled successfully", map[string]interface{}{
		"device_id": deviceID,
		"udid":      data.UDID.ValueString(),
	})
}

// ImportState imports an existing device into Terraform state.
func (r *DeviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "Importing device resource", map[string]interface{}{
		"import_id": req.ID,
	})

	// Support importing by both device ID and UDID
	importID := strings.TrimSpace(req.ID)
	if importID == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Device import ID cannot be empty. Provide either the Apple device ID or UDID.",
		)
		return
	}

	var device *models.Device
	var err error

	// First try as device ID (Apple's format)
	if device, err = r.client.GetDevice(importID); err != nil {
		// If that fails, try as UDID
		if device, err = r.client.GetDeviceByUDID(importID); err != nil {
			resp.Diagnostics.AddError(
				"Device Not Found",
				fmt.Sprintf("Unable to find device with ID or UDID '%s': %s", importID, err),
			)
			return
		}
	}

	tflog.Debug(ctx, "Successfully found device for import", map[string]interface{}{
		"device_id": device.ID,
		"udid":      device.Attributes.UDID,
		"name":      device.Attributes.Name,
	})

	// Set the ID in state so subsequent Read operation works
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), device.ID)...)
}

// mapDeviceToModel maps an Apple API device response to the Terraform model.
func (r *DeviceResource) mapDeviceToModel(device *models.Device, data *deviceModel) {
	data.ID = types.StringValue(device.ID)
	data.Name = types.StringValue(device.Attributes.Name)
	data.UDID = types.StringValue(device.Attributes.UDID)
	data.Platform = types.StringValue(string(device.Attributes.Platform))
	data.DeviceClass = types.StringValue(string(device.Attributes.DeviceClass))
	data.Status = types.StringValue(string(device.Attributes.Status))

	// Set optional model field
	if device.Attributes.Model != nil {
		data.Model = types.StringValue(*device.Attributes.Model)
	} else {
		data.Model = types.StringNull()
	}

	// Set optional added date field
	if device.Attributes.AddedDate != nil {
		data.AddedDate = types.StringValue(device.Attributes.AddedDate.Format(time.RFC3339))
	} else {
		data.AddedDate = types.StringNull()
	}
}
