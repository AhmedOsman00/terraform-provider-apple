// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package device

import (
	"context"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strList(values ...string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

func devices() []models.Device {
	return []models.Device{
		{ID: "1", Attributes: models.DeviceAttributes{
			Name: "Alice iPhone", UDID: "aaa111", Platform: models.DevicePlatform("IOS"),
			DeviceClass: models.DeviceClass("IPHONE"), Status: models.DeviceStatusENABLED,
		}},
		{ID: "2", Attributes: models.DeviceAttributes{
			Name: "bob iPad", UDID: "bbb222", Platform: models.DevicePlatform("IOS"),
			DeviceClass: models.DeviceClass("IPAD"), Status: models.DeviceStatusDISABLED,
		}},
		{ID: "3", Attributes: models.DeviceAttributes{
			Name: "Carol Mac", UDID: "ccc333", Platform: models.DevicePlatform("MAC_OS"),
			DeviceClass: models.DeviceClass("MAC"), Status: models.DeviceStatusENABLED,
		}},
	}
}

func udids(list []models.Device) []string {
	out := make([]string, 0, len(list))
	for _, d := range list {
		out = append(out, d.Attributes.UDID)
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// baseConfig returns a config with every filter unset.
func baseConfig() devicesDataSourceModel {
	return devicesDataSourceModel{
		Platform:      types.StringNull(),
		Platforms:     types.ListNull(types.StringType),
		DeviceClass:   types.StringNull(),
		DeviceClasses: types.ListNull(types.StringType),
		Status:        types.StringNull(),
		NamePattern:   types.StringNull(),
		UDIDPattern:   types.StringNull(),
		Limit:         types.Int64Null(),
		SortBy:        types.StringNull(),
		SortOrder:     types.StringNull(),
	}
}

func TestFilterDevices(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		mutate func(*devicesDataSourceModel)
		want   []string
	}{
		{name: "no filters", mutate: func(c *devicesDataSourceModel) {}, want: []string{"aaa111", "bbb222", "ccc333"}},
		{name: "single platform", mutate: func(c *devicesDataSourceModel) { c.Platform = types.StringValue("MAC_OS") }, want: []string{"ccc333"}},
		// The list-valued filters are decoded once up front rather than per device.
		{name: "platforms list", mutate: func(c *devicesDataSourceModel) { c.Platforms = strList("IOS") }, want: []string{"aaa111", "bbb222"}},
		{name: "device classes list", mutate: func(c *devicesDataSourceModel) { c.DeviceClasses = strList("IPAD", "MAC") }, want: []string{"bbb222", "ccc333"}},
		{name: "empty platforms list matches everything", mutate: func(c *devicesDataSourceModel) { c.Platforms = strList() }, want: []string{"aaa111", "bbb222", "ccc333"}},
		{name: "status", mutate: func(c *devicesDataSourceModel) { c.Status = types.StringValue("DISABLED") }, want: []string{"bbb222"}},
		{name: "name regex", mutate: func(c *devicesDataSourceModel) { c.NamePattern = types.StringValue("^Alice") }, want: []string{"aaa111"}},
		{name: "udid regex", mutate: func(c *devicesDataSourceModel) { c.UDIDPattern = types.StringValue("^ccc") }, want: []string{"ccc333"}},
		{name: "limit", mutate: func(c *devicesDataSourceModel) { c.Limit = types.Int64Value(2) }, want: []string{"aaa111", "bbb222"}},
		{
			name: "filters combine",
			mutate: func(c *devicesDataSourceModel) {
				c.Platforms = strList("IOS")
				c.Status = types.StringValue("ENABLED")
			},
			want: []string{"aaa111"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig()
			tt.mutate(&config)

			got, err := filterDevices(ctx, devices(), config)
			if err != nil {
				t.Fatalf("filterDevices() returned error: %v", err)
			}
			if !equalStrings(udids(got), tt.want) {
				t.Errorf("filterDevices() = %v, want %v", udids(got), tt.want)
			}
		})
	}
}

func TestFilterDevicesRejectsBadPatterns(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name   string
		mutate func(*devicesDataSourceModel)
	}{
		{"name_pattern", func(c *devicesDataSourceModel) { c.NamePattern = types.StringValue("(unclosed") }},
		{"udid_pattern", func(c *devicesDataSourceModel) { c.UDIDPattern = types.StringValue("[a-") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig()
			tt.mutate(&config)

			got, err := filterDevices(ctx, devices(), config)
			if err == nil {
				t.Fatalf("filterDevices() = %v with nil error, want an error naming the bad pattern", udids(got))
			}
			if got != nil {
				t.Errorf("filterDevices() returned %v alongside an error, want nil", udids(got))
			}
		})
	}
}

func TestSortDevices(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{"name ascending is case-insensitive", "name", "asc", []string{"aaa111", "bbb222", "ccc333"}},
		{"name descending", "name", "desc", []string{"ccc333", "bbb222", "aaa111"}},
		{"udid ascending", "udid", "asc", []string{"aaa111", "bbb222", "ccc333"}},
		{"platform ascending", "platform", "asc", []string{"aaa111", "bbb222", "ccc333"}},
		{"unknown field falls back to name", "nonsense", "asc", []string{"aaa111", "bbb222", "ccc333"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig()
			config.SortBy = types.StringValue(tt.sortBy)
			config.SortOrder = types.StringValue(tt.sortOrder)

			got, err := filterDevices(ctx, devices(), config)
			if err != nil {
				t.Fatalf("filterDevices() returned error: %v", err)
			}
			if !equalStrings(udids(got), tt.want) {
				t.Errorf("sort by %q %q = %v, want %v", tt.sortBy, tt.sortOrder, udids(got), tt.want)
			}
		})
	}
}
