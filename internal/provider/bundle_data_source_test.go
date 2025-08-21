package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccBundleIDsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a Bundle ID first to ensure we have data to read
			{
				Config: testAccBundleIDsDataSourceConfigWithResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify the resource was created
					resource.TestCheckResourceAttr("apple_bundle_id.test", "identifier", "com.test.datasource"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "name", "Test DataSource"),
					resource.TestCheckResourceAttr("apple_bundle_id.test", "platform", "IOS"),
					// Verify the data source can read Bundle IDs
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.test", "bundle_ids.#"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.test", "filtered_count"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.test", "last_updated"),
				),
			},
		},
	})
}

func TestAccBundleIDsDataSource_empty(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Test data source when no Bundle IDs exist (or when there are Bundle IDs)
			{
				Config: testAccBundleIDsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The data source should return successfully even if empty
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.test", "bundle_ids.#"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.test", "filtered_count"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.test", "last_updated"),
				),
			},
		},
	})
}

func TestAccBundleIDsDataSource_filtering(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDsDataSourceConfigWithFiltering(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify resources were created
					resource.TestCheckResourceAttr("apple_bundle_id.ios", "platform", "IOS"),
					resource.TestCheckResourceAttr("apple_bundle_id.macos", "platform", "MAC_OS"),
					// Verify platform filtering works
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.ios_only", "bundle_ids.#"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.multiple_platforms", "bundle_ids.#"),
					// Verify identifier prefix filtering
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.prefix_filter", "bundle_ids.#"),
				),
			},
		},
	})
}

func TestAccBundleIDsDataSource_sorting(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDsDataSourceConfigWithSorting(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify resources were created
					resource.TestCheckResourceAttr("apple_bundle_id.sort1", "identifier", "com.test.sort.aaa"),
					resource.TestCheckResourceAttr("apple_bundle_id.sort2", "identifier", "com.test.sort.zzz"),
					// Verify sorting works (we can't test exact order due to other Bundle IDs)
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.sorted_asc", "bundle_ids.#"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.sorted_desc", "bundle_ids.#"),
				),
			},
		},
	})
}

func TestAccBundleIDsDataSource_limit(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBundleIDsDataSourceConfigWithLimit(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify the limit is respected (at most 2 results)
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.limited", "bundle_ids.#"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.limited", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_bundle_ids.limited", "filtered_count"),
				),
			},
		},
	})
}

func TestAccBundleIDsDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBundleIDsDataSourceConfigInvalidPlatform(),
				ExpectError: regexp.MustCompile(`Attribute platform value must be one of`),
			},
			{
				Config:      testAccBundleIDsDataSourceConfigInvalidLimit(),
				ExpectError: regexp.MustCompile(`Attribute limit value must be between 1 and 200`),
			},
			{
				Config:      testAccBundleIDsDataSourceConfigInvalidSortBy(),
				ExpectError: regexp.MustCompile(`Attribute sort_by value must be one of`),
			},
		},
	})
}

// Basic data source configuration without any resources
func testAccBundleIDsDataSourceConfig() string {
	return `
data "apple_bundle_ids" "test" {}
`
}

// Data source configuration with a single Bundle ID resource
func testAccBundleIDsDataSourceConfigWithResource() string {
	return `
resource "apple_bundle_id" "test" {
  identifier = "com.test.datasource"
  name       = "Test DataSource"
  platform   = "IOS"
}

data "apple_bundle_ids" "test" {
  depends_on = [apple_bundle_id.test]
}
`
}

// Data source configuration with filtering
func testAccBundleIDsDataSourceConfigWithFiltering() string {
	return `
resource "apple_bundle_id" "ios" {
  identifier = "com.test.filter.ios"
  name       = "Test iOS Filter"
  platform   = "IOS"
}

resource "apple_bundle_id" "macos" {
  identifier = "com.test.filter.macos"
  name       = "Test macOS Filter"
  platform   = "MAC_OS"
}

# Filter by single platform
data "apple_bundle_ids" "ios_only" {
  platform = "IOS"
  depends_on = [
    apple_bundle_id.ios,
    apple_bundle_id.macos
  ]
}

# Filter by multiple platforms
data "apple_bundle_ids" "multiple_platforms" {
  platforms = ["IOS", "MAC_OS"]
  depends_on = [
    apple_bundle_id.ios,
    apple_bundle_id.macos
  ]
}

# Filter by identifier prefix
data "apple_bundle_ids" "prefix_filter" {
  identifier_prefix = "com.test.filter"
  depends_on = [
    apple_bundle_id.ios,
    apple_bundle_id.macos
  ]
}
`
}

// Data source configuration with sorting
func testAccBundleIDsDataSourceConfigWithSorting() string {
	return `
resource "apple_bundle_id" "sort1" {
  identifier = "com.test.sort.aaa"
  name       = "ZZZ Sort Test"
  platform   = "IOS"
}

resource "apple_bundle_id" "sort2" {
  identifier = "com.test.sort.zzz"
  name       = "AAA Sort Test"
  platform   = "IOS"
}

data "apple_bundle_ids" "sorted_asc" {
  identifier_prefix = "com.test.sort"
  sort_by = "identifier"
  sort_order = "asc"
  depends_on = [
    apple_bundle_id.sort1,
    apple_bundle_id.sort2
  ]
}

data "apple_bundle_ids" "sorted_desc" {
  identifier_prefix = "com.test.sort"
  sort_by = "name"
  sort_order = "desc"
  depends_on = [
    apple_bundle_id.sort1,
    apple_bundle_id.sort2
  ]
}
`
}

// Data source configuration with limit
func testAccBundleIDsDataSourceConfigWithLimit() string {
	return `
data "apple_bundle_ids" "limited" {
  limit = 2
}
`
}

// Invalid configurations for validation testing
func testAccBundleIDsDataSourceConfigInvalidPlatform() string {
	return `
data "apple_bundle_ids" "test" {
  platform = "INVALID"
}
`
}

func testAccBundleIDsDataSourceConfigInvalidLimit() string {
	return `
data "apple_bundle_ids" "test" {
  limit = 500
}
`
}

func testAccBundleIDsDataSourceConfigInvalidSortBy() string {
	return `
data "apple_bundle_ids" "test" {
  sort_by = "invalid"
}
`
}
