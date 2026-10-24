// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccCiWorkflowResource_basic(t *testing.T) {
	name := "Terraform Disposable " + acctest.RandString(8)
	config := func(description string) string {
		return fmt.Sprintf(`
resource "apple_ci_workflow" "test" {
 name = %q
 description = %q
 is_enabled = false
 clean = false
 container_file_path = %q
 product_id = %q
 repository_id = %q
 xcode_version_id = %q
 mac_os_version_id = %q
 manual_branch_start_condition = { source = { is_all_match = true } }
 actions = [{ action_type = "BUILD", scheme = %q, platform = "IOS", destination = "ANY_IOS_DEVICE", is_required_to_pass = true }]
}
`, name, description, os.Getenv("APPLE_TEST_CI_CONTAINER_FILE_PATH"), os.Getenv("APPLE_TEST_CI_PRODUCT_ID"), os.Getenv("APPLE_TEST_CI_REPOSITORY_ID"), os.Getenv("APPLE_TEST_CI_XCODE_VERSION_ID"), os.Getenv("APPLE_TEST_CI_MAC_OS_VERSION_ID"), os.Getenv("APPLE_TEST_CI_SCHEME"))
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			for _, name := range []string{"APPLE_TEST_CI_PRODUCT_ID", "APPLE_TEST_CI_REPOSITORY_ID", "APPLE_TEST_CI_XCODE_VERSION_ID", "APPLE_TEST_CI_MAC_OS_VERSION_ID", "APPLE_TEST_CI_CONTAINER_FILE_PATH", "APPLE_TEST_CI_SCHEME"} {
				if os.Getenv(name) == "" {
					t.Skip(name + " must identify an existing Xcode Cloud record")
				}
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			client, err := testAccAPIClient()
			if err != nil {
				return err
			}
			for _, r := range state.RootModule().Resources {
				if r.Type != "apple_ci_workflow" {
					continue
				}
				workflows, err := client.GetCiWorkflows(r.Primary.Attributes["product_id"])
				if err != nil {
					return err
				}
				for _, workflow := range workflows {
					if workflow.ID == r.Primary.ID {
						return fmt.Errorf("CI workflow %s is still active", r.Primary.ID)
					}
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config("Disposable acceptance test"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("apple_ci_workflow.test", "id"), resource.TestCheckResourceAttr("apple_ci_workflow.test", "is_enabled", "false"), resource.TestCheckResourceAttr("apple_ci_workflow.test", "actions.0.name", "Build - iOS"))},
			{Config: config("Updated disposable acceptance test"), Check: resource.TestCheckResourceAttr("apple_ci_workflow.test", "description", "Updated disposable acceptance test")},
			{ResourceName: "apple_ci_workflow.test", ImportState: true, ImportStateVerify: true},
		},
	})
}
