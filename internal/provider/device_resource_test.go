package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccDeviceResource(t *testing.T) {
	// Note: This test requires a real iOS device UDID to test with
	// In a real testing environment, you would provide test device UDIDs
	testUDID := "00008030-000A4D8E0AB8802E" // 8-16 hex format, iPhone XS and later
	testName := "Test Device"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccDeviceResourceConfig(testName, testUDID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_device.test", "name", testName),
					resource.TestCheckResourceAttr("apple_device.test", "udid", testUDID),
					resource.TestCheckResourceAttr("apple_device.test", "platform", "IOS"),
					resource.TestCheckResourceAttrSet("apple_device.test", "id"),
					resource.TestCheckResourceAttrSet("apple_device.test", "device_class"),
					resource.TestCheckResourceAttrSet("apple_device.test", "status"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "apple_device.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccDeviceImportStateIdFunc(),
			},
			// Update testing
			{
				Config: testAccDeviceResourceConfig("Updated Test Device", testUDID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_device.test", "name", "Updated Test Device"),
					resource.TestCheckResourceAttr("apple_device.test", "udid", testUDID),
					resource.TestCheckResourceAttr("apple_device.test", "platform", "IOS"),
				),
			},
		},
	})
}

func TestAccDeviceResourceMacOS(t *testing.T) {
	// Test with macOS device (UUID format)
	testUDID := "550e8400-e29b-41d4-a716-446655440000" // UUID format for macOS
	testName := "Test Mac"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeviceResourceMacOSConfig(testName, testUDID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_device.test_mac", "name", testName),
					resource.TestCheckResourceAttr("apple_device.test_mac", "udid", testUDID),
					resource.TestCheckResourceAttr("apple_device.test_mac", "platform", "MAC_OS"),
					resource.TestCheckResourceAttrSet("apple_device.test_mac", "id"),
					resource.TestCheckResourceAttrSet("apple_device.test_mac", "device_class"),
					resource.TestCheckResourceAttrSet("apple_device.test_mac", "status"),
				),
			},
		},
	})
}

// testAccDeviceImportStateIdFunc returns a function that retrieves the device ID for import testing.
func testAccDeviceImportStateIdFunc() resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources["apple_device.test"]
		if !ok {
			return "", fmt.Errorf("not found: apple_device.test")
		}

		if rs.Primary.ID == "" {
			return "", fmt.Errorf("no ID is set")
		}

		return rs.Primary.ID, nil
	}
}

func testAccDeviceResourceConfig(name, udid string) string {
	return fmt.Sprintf(`
%s

resource "apple_device" "test" {
  name     = %[2]q
  udid     = %[3]q
  platform = "IOS"
}
`, testAccProviderConfig, name, udid)
}

func testAccDeviceResourceMacOSConfig(name, udid string) string {
	return fmt.Sprintf(`
%s

resource "apple_device" "test_mac" {
  name     = %[2]q
  udid     = %[3]q
  platform = "MAC_OS"
}
`, testAccProviderConfig, name, udid)
}
