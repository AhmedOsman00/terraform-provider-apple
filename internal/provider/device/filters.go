package device

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"terraform-provider-apple/internal/apple/models"
)

// filterDevices applies all specified filters to the devices list
func filterDevices(devices []models.Device, config devicesDataSourceModel) ([]models.Device, error) {
	var filtered []models.Device

	for _, device := range devices {
		if shouldIncludeDevice(device, config) {
			filtered = append(filtered, device)
		}
	}

	// Apply sorting
	if err := sortDevices(filtered, config); err != nil {
		return nil, err
	}

	// Apply limit
	if !config.Limit.IsNull() && config.Limit.ValueInt64() > 0 {
		limit := int(config.Limit.ValueInt64())
		if limit < len(filtered) {
			filtered = filtered[:limit]
		}
	}

	return filtered, nil
}

// shouldIncludeDevice determines if a device matches all filter criteria
func shouldIncludeDevice(device models.Device, config devicesDataSourceModel) bool {
	// Platform filter (single)
	if !config.Platform.IsNull() && !config.Platform.IsUnknown() {
		if string(device.Attributes.Platform) != config.Platform.ValueString() {
			return false
		}
	}

	// Platforms filter (multiple)
	if !config.Platforms.IsNull() && !config.Platforms.IsUnknown() {
		var platforms []string
		diags := config.Platforms.ElementsAs(nil, &platforms, false)
		if diags.HasError() {
			return false
		}
		if !slices.Contains(platforms, string(device.Attributes.Platform)) {
			return false
		}
	}

	// Device class filter (single)
	if !config.DeviceClass.IsNull() && !config.DeviceClass.IsUnknown() {
		if string(device.Attributes.DeviceClass) != config.DeviceClass.ValueString() {
			return false
		}
	}

	// Device classes filter (multiple)
	if !config.DeviceClasses.IsNull() && !config.DeviceClasses.IsUnknown() {
		var deviceClasses []string
		diags := config.DeviceClasses.ElementsAs(nil, &deviceClasses, false)
		if diags.HasError() {
			return false
		}
		if !slices.Contains(deviceClasses, string(device.Attributes.DeviceClass)) {
			return false
		}
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

// sortDevices sorts the devices list based on the specified criteria
func sortDevices(devices []models.Device, config devicesDataSourceModel) error {
	if config.SortBy.IsNull() || config.SortBy.IsUnknown() {
		return nil
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

	return nil
}

// compareStrings compares two strings case-insensitively
func compareStrings(a, b string) bool {
	return strings.ToLower(a) < strings.ToLower(b)
}

// compareDates compares two time pointers, treating nil as oldest
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
