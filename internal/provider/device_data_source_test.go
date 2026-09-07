package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDevicesDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccDevicesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_devices.test", "devices.#"),
					resource.TestCheckResourceAttrSet("data.apple_devices.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_devices.test", "filtered_count"),
				),
			},
		},
	})
}

func TestAccDevicesDataSourceFiltered(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDevicesDataSourceFilteredConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_devices.ios_only", "devices.#"),
					resource.TestCheckResourceAttr("data.apple_devices.ios_only", "platform", "IOS"),
					resource.TestCheckResourceAttrSet("data.apple_devices.ios_only", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_devices.ios_only", "filtered_count"),
				),
			},
		},
	})
}

func TestAccDevicesDataSourceWithLimit(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDevicesDataSourceWithLimitConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.apple_devices.limited", "limit", "5"),
					resource.TestCheckResourceAttrSet("data.apple_devices.limited", "devices.#"),
				),
			},
		},
	})
}

func testAccDevicesDataSourceConfig() string {
	return fmt.Sprintf(`
%s

data "apple_devices" "test" {}
`, testAccProviderConfig)
}

func testAccDevicesDataSourceFilteredConfig() string {
	return fmt.Sprintf(`
%s

data "apple_devices" "ios_only" {
  platform = "IOS"
  sort_by = "name"
  sort_order = "asc"
}
`, testAccProviderConfig)
}

func testAccDevicesDataSourceWithLimitConfig() string {
	return fmt.Sprintf(`
%s

data "apple_devices" "limited" {
  limit = 5
  sort_by = "name"
}
`, testAccProviderConfig)
}
