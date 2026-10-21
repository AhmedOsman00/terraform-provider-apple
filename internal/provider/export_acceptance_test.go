// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"strings"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/export"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccExport_firstPlanChangesNothing is the promise apple terraform export
// makes: configuration written from the live account imports every record and
// plans no change to any of them. A value the export writes differently from
// what the import records shows up here as an update, and one on a
// RequiresReplace attribute as a replacement.
//
// It only plans. An import-only plan counts as empty, so a PlanOnly step fails
// on anything else, and nothing is applied: there is no state to destroy
// afterwards, which matters because destroying these would delete the test
// app's version, beta groups and team members. Signing and the team are
// account-wide, so this reads the whole team's Developer Portal records too.
func TestAccExport_firstPlanChangesNothing(t *testing.T) {
	testAccPreCheckSubscription(t)

	client, err := testAccAPIClient()
	if err != nil {
		t.Fatalf("building client: %v", err)
	}

	result, err := export.Run(client, export.Options{Apps: []string{testAccAppID()}, Progress: t.Logf})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	for _, note := range result.Notes {
		t.Logf("%s %s %s", note.Severity, note.Address, note.Message)
	}
	if result.ImportCount() == 0 {
		t.Fatal("the export imports nothing, so there is no plan to check")
	}

	var config strings.Builder
	for _, name := range result.FileNames() {
		// The test framework supplies the provider; the generated
		// required_providers block would name the Registry's copy instead.
		if name == "providers.tf" || !strings.HasSuffix(name, ".tf") {
			continue
		}
		config.Write(result.Files[name])
		config.WriteString("\n")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:   config.String(),
				PlanOnly: true,
			},
		},
	})
}
