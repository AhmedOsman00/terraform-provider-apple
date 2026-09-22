// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The listing is read-only and writes nothing to the team, so unlike the two
// resources in user_resource_test.go it needs credentials and nothing else.
// Every team has at least the account holder in it, which is what makes an
// unfiltered count worth asserting on.

func TestAccUsersDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "apple_users" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_users.all", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_users.all", "filtered_count"),
					// Every App Store Connect team has an account holder.
					resource.TestMatchResourceAttr("data.apple_users.all", "users.#",
						regexp.MustCompile(`^[1-9][0-9]*$`)),
					resource.TestCheckResourceAttrSet("data.apple_users.all", "users.0.id"),
					resource.TestCheckResourceAttrSet("data.apple_users.all", "users.0.username"),
					resource.TestCheckResourceAttrSet("data.apple_users.all", "users.0.roles.#"),
				),
			},
		},
	})
}

// TestAccUsersDataSource_filtered covers the filters against a real team.
//
// The assertions are deliberately shape-only: which people a team holds is not
// something a test can know, so it checks that filtering narrows rather than
// that it narrows to anybody in particular.
func TestAccUsersDataSource_filtered(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "apple_users" "all" {}

data "apple_users" "admins" {
  role       = "ADMIN"
  sort_by    = "username"
  sort_order = "asc"
}

# A pattern nobody can match, to prove the filter is applied rather than ignored.
data "apple_users" "none" {
  username_pattern = "terraform-acceptance-matches-nobody"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.apple_users.admins", "filtered_count"),
					resource.TestCheckResourceAttrPair(
						"data.apple_users.admins", "total_count", "data.apple_users.all", "total_count"),
					resource.TestCheckResourceAttr("data.apple_users.none", "filtered_count", "0"),
					resource.TestCheckResourceAttr("data.apple_users.none", "users.#", "0"),
				),
			},
		},
	})
}
