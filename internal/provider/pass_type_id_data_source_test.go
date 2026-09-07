package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPassTypeIDsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPassTypeIDsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_pass_type_ids.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_pass_type_ids.test", "filtered_count"),
				),
			},
		},
	})
}

func TestAccPassTypeIDsDataSource_withFilters(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPassTypeIDsDataSourceConfigWithFilters(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_pass_type_ids.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_pass_type_ids.test", "filtered_count"),
				),
			},
		},
	})
}

func TestAccPassTypeIDsDataSource_withSorting(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPassTypeIDsDataSourceConfigWithSorting(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_pass_type_ids.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_pass_type_ids.test", "filtered_count"),
				),
			},
		},
	})
}

func TestAccPassTypeIDsDataSource_withLimit(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPassTypeIDsDataSourceConfigWithLimit(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_pass_type_ids.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_pass_type_ids.test", "filtered_count"),
				),
			},
		},
	})
}

func testAccPassTypeIDsDataSourceConfig() string {
	return `
data "apple_pass_type_ids" "test" {
}
`
}

func testAccPassTypeIDsDataSourceConfigWithFilters() string {
	return `
data "apple_pass_type_ids" "test" {
  identifier_prefix = "pass.com.example"
  name_pattern      = ".*Test.*"
}
`
}

func testAccPassTypeIDsDataSourceConfigWithSorting() string {
	return `
data "apple_pass_type_ids" "test" {
  sort_by    = "name"
  sort_order = "desc"
}
`
}

func testAccPassTypeIDsDataSourceConfigWithLimit() string {
	return `
data "apple_pass_type_ids" "test" {
  limit = 10
}
`
}
