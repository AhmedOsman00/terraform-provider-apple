// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package device

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// filterDevices applies all specified filters to the devices list.
func filterDevices(ctx context.Context, devices []models.Device, config devicesDataSourceModel) ([]models.Device, error) {
	// Reject malformed patterns up front rather than silently matching nothing.
	for _, p := range []struct {
		name  string
		value types.String
	}{
		{"name_pattern", config.NamePattern},
		{"udid_pattern", config.UDIDPattern},
	} {
		if p.value.IsNull() || p.value.IsUnknown() {
			continue
		}
		if _, err := regexp.Compile(p.value.ValueString()); err != nil {
			return nil, fmt.Errorf("invalid %s %q: %w", p.name, p.value.ValueString(), err)
		}
	}

	// Decode the list-valued filters once. Doing this per device repeated the
	// same conversion for every record and, lacking a Context, could only
	// report a failure by silently excluding the device.
	var platforms []string
	if !config.Platforms.IsNull() && !config.Platforms.IsUnknown() {
		if diags := config.Platforms.ElementsAs(ctx, &platforms, false); diags.HasError() {
			return nil, fmt.Errorf("could not read the platforms filter: %s", firstError(diags))
		}
	}

	var deviceClasses []string
	if !config.DeviceClasses.IsNull() && !config.DeviceClasses.IsUnknown() {
		if diags := config.DeviceClasses.ElementsAs(ctx, &deviceClasses, false); diags.HasError() {
			return nil, fmt.Errorf("could not read the device_classes filter: %s", firstError(diags))
		}
	}

	var filtered []models.Device

	for _, device := range devices {
		if shouldIncludeDevice(device, config, platforms, deviceClasses) {
			filtered = append(filtered, device)
		}
	}

	sortDevices(filtered, config)

	// Apply limit
	if !config.Limit.IsNull() && config.Limit.ValueInt64() > 0 {
		limit := int(config.Limit.ValueInt64())
		if limit < len(filtered) {
			filtered = filtered[:limit]
		}
	}

	return filtered, nil
}

// shouldIncludeDevice determines if a device matches all filter criteria.
func shouldIncludeDevice(device models.Device, config devicesDataSourceModel, platforms, deviceClasses []string) bool {
	// Platform filter (single)
	if !config.Platform.IsNull() && !config.Platform.IsUnknown() {
		if string(device.Attributes.Platform) != config.Platform.ValueString() {
			return false
		}
	}

	// Platforms filter (multiple)
	if len(platforms) > 0 && !slices.Contains(platforms, string(device.Attributes.Platform)) {
		return false
	}

	// Device class filter (single)
	if !config.DeviceClass.IsNull() && !config.DeviceClass.IsUnknown() {
		if string(device.Attributes.DeviceClass) != config.DeviceClass.ValueString() {
			return false
		}
	}

	// Device classes filter (multiple)
	if len(deviceClasses) > 0 && !slices.Contains(deviceClasses, string(device.Attributes.DeviceClass)) {
		return false
	}

	// Status filter
	if !config.Status.IsNull() && !config.Status.IsUnknown() {
		if string(device.Attributes.Status) != config.Status.ValueString() {
			return false
		}
	}

	// Name pattern filter
	if !config.NamePattern.IsNull() && !config.NamePattern.IsUnknown() {
		pattern := config.NamePattern.ValueString()
		matched, err := regexp.MatchString(pattern, device.Attributes.Name)
		if err != nil || !matched {
			return false
		}
	}

	// UDID pattern filter
	if !config.UDIDPattern.IsNull() && !config.UDIDPattern.IsUnknown() {
		pattern := config.UDIDPattern.ValueString()
		matched, err := regexp.MatchString(pattern, device.Attributes.UDID)
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// sortDevices sorts the devices list based on the specified criteria.
func sortDevices(devices []models.Device, config devicesDataSourceModel) {
	if config.SortBy.IsNull() || config.SortBy.IsUnknown() {
		return
	}

	sortBy := config.SortBy.ValueString()
	ascending := true
	if !config.SortOrder.IsNull() && !config.SortOrder.IsUnknown() {
		ascending = config.SortOrder.ValueString() == "asc"
	}

	sort.Slice(devices, func(i, j int) bool {
		var result bool
		switch sortBy {
		case "name":
			result = compareStrings(devices[i].Attributes.Name, devices[j].Attributes.Name)
		case "udid":
			result = compareStrings(devices[i].Attributes.UDID, devices[j].Attributes.UDID)
		case "platform":
			result = compareStrings(string(devices[i].Attributes.Platform), string(devices[j].Attributes.Platform))
		case "device_class":
			result = compareStrings(string(devices[i].Attributes.DeviceClass), string(devices[j].Attributes.DeviceClass))
		case "status":
			result = compareStrings(string(devices[i].Attributes.Status), string(devices[j].Attributes.Status))
		case "added_date":
			result = compareDates(devices[i].Attributes.AddedDate, devices[j].Attributes.AddedDate)
		default:
			result = compareStrings(devices[i].Attributes.Name, devices[j].Attributes.Name)
		}

		if ascending {
			return result
		}
		return !result
	})
}

// compareStrings compares two strings case-insensitively.
func compareStrings(a, b string) bool {
	return strings.ToLower(a) < strings.ToLower(b)
}

// compareDates compares two time pointers, treating nil as oldest.
func compareDates(a, b *time.Time) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil {
		return true
	}
	if b == nil {
		return false
	}
	return a.Before(*b)
}

// firstError renders the first error diagnostic as a string.
func firstError(diags diag.Diagnostics) string {
	errs := diags.Errors()
	if len(errs) == 0 {
		return "unknown error"
	}
	return fmt.Sprintf("%s: %s", errs[0].Summary(), errs[0].Detail())
}
