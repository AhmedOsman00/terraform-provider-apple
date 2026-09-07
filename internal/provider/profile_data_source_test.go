// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccProfilesDataSource_basic creates one profile and reads it back.
// name_pattern is matched as a regular expression against the whole collection,
// so the fixture name is distinctive enough to pin the count exactly.
func TestAccProfilesDataSource_basic(t *testing.T) {
	csr := testAccCertificateCSR(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckProfileDestroy,
			testAccCheckCertificateDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccProfilesDataSourceConfig(csr),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_profiles.all", "profiles.#"),
					resource.TestCheckResourceAttrSet("data.apple_profiles.all", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_profiles.all", "filtered_count"),

					resource.TestCheckResourceAttr("data.apple_profiles.by_name", "filtered_count", "1"),
					resource.TestCheckResourceAttr("data.apple_profiles.by_name", "profiles.#", "1"),
					resource.TestCheckResourceAttrPair(
						"data.apple_profiles.by_name", "profiles.0.id",
						"apple_profile.test", "id",
					),
					resource.TestCheckResourceAttr("data.apple_profiles.by_name", "profiles.0.profile_type", "IOS_APP_STORE"),
					resource.TestCheckResourceAttr("data.apple_profiles.by_name", "profiles.0.platform", "IOS"),
					resource.TestCheckResourceAttr("data.apple_profiles.by_name", "profiles.0.profile_state", "ACTIVE"),

					// The fixture is an active iOS App Store profile, so
					// neither filter can come back empty.
					resource.TestMatchResourceAttr("data.apple_profiles.by_type", "filtered_count", regexp.MustCompile(`^[1-9][0-9]*$`)),
					resource.TestMatchResourceAttr("data.apple_profiles.by_state", "filtered_count", regexp.MustCompile(`^[1-9][0-9]*$`)),

					resource.TestCheckResourceAttr("data.apple_profiles.limited", "profiles.#", "1"),
				),
			},
		},
	})
}

func TestAccProfilesDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "apple_profiles" "test" {
  profile_type = "NOT_A_PROFILE_TYPE"
}
`,
				ExpectError: regexp.MustCompile(`Attribute profile_type value must be one of`),
			},
			{
				Config: `
data "apple_profiles" "test" {
  profile_state = "SOMEWHAT_ACTIVE"
}
`,
				ExpectError: regexp.MustCompile(`Attribute profile_state value must be one of`),
			},
			{
				Config: `
data "apple_profiles" "test" {
  limit = 0
}
`,
				ExpectError: regexp.MustCompile(`Attribute limit value must be between 1 and 200`),
			},
		},
	})
}

func testAccProfilesDataSourceConfig(csr string) string {
	return fmt.Sprintf(`
resource "apple_bundle_id" "test" {
  identifier = "com.test.terraform-profile-ds"
  name       = "Terraform Profile DS Test"
  platform   = "IOS"
}

resource "apple_certificate" "test" {
  certificate_type = "IOS_DISTRIBUTION"
  csr_content      = %[1]q
}

resource "apple_profile" "test" {
  name         = "Terraform DS Profile"
  profile_type = "IOS_APP_STORE"
  bundle_id    = apple_bundle_id.test.id
  certificates = [apple_certificate.test.id]
}

data "apple_profiles" "all" {
  depends_on = [apple_profile.test]
}

data "apple_profiles" "by_name" {
  name_pattern = "^Terraform DS Profile$"

  depends_on = [apple_profile.test]
}

data "apple_profiles" "by_type" {
  profile_type = apple_profile.test.profile_type
}

data "apple_profiles" "by_state" {
  profile_state = apple_profile.test.profile_state
}

data "apple_profiles" "limited" {
  profile_type = apple_profile.test.profile_type
  limit        = 1
}
`, csr)
}
