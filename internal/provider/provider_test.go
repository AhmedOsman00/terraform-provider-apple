// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"apple": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifies and sets up necessary testing prerequisites.
func testAccPreCheck(t *testing.T) {
	// Check if running in acceptance test mode
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC environment variable must be set for acceptance tests")
	}

	// Verify that required environment variables are set for testing
	required := []string{
		"APPLE_APP_STORE_CONNECT_ISSUER_ID",
		"APPLE_APP_STORE_CONNECT_API_KEY",
		"APPLE_APP_STORE_CONNECT_PRIVATE_KEY",
	}

	var missing []string
	for _, envVar := range required {
		if os.Getenv(envVar) == "" {
			missing = append(missing, envVar)
		}
	}

	// Skip rather than fail: a checkout without App Store Connect credentials
	// is the normal case for contributors and for CI on forks. Failing there
	// reports a red build for tests that were never runnable, which hides real
	// regressions.
	if len(missing) > 0 {
		t.Skipf("skipping acceptance test: %s must be set", strings.Join(missing, ", "))
	}
}

func TestAccProvider(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig,
				Check:  resource.ComposeAggregateTestCheckFunc(
				// Basic provider functionality test
				// More detailed tests are in individual resource/data source tests
				),
			},
		},
	})
}

func TestAccProvider_ConfigurationValidation(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "apple" {
  issuer_id   = "not-a-uuid"
  api_key     = "VALIDKEY10"
  private_key = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC\n-----END PRIVATE KEY-----"
}

# Add a data source to trigger provider validation
data "apple_bundle_ids" "test" {}
`,
				ExpectError: regexp.MustCompile(`Issuer ID must be a valid UUID format`),
			},
			{
				Config: `
provider "apple" {
  issuer_id   = "12345678-1234-1234-1234-123456789012"
  api_key     = "invalid"
  private_key = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC\n-----END PRIVATE KEY-----"
}

# Add a data source to trigger provider validation
data "apple_bundle_ids" "test" {}
`,
				ExpectError: regexp.MustCompile(`API Key must be a 10-character alphanumeric string`),
			},
			{
				Config: `
provider "apple" {
  issuer_id   = "12345678-1234-1234-1234-123456789012"
  api_key     = "VALIDKEY10"
  private_key = "not-a-pem-key"
}

# Add a data source to trigger provider validation
data "apple_bundle_ids" "test" {}
`,
				ExpectError: regexp.MustCompile(`Private Key must be a valid PEM-encoded private key`),
			},
		},
	})
}

const testAccProviderConfig = `
provider "apple" {
  # Configuration will be supplied via environment variables:
  # APPLE_APP_STORE_CONNECT_ISSUER_ID
  # APPLE_APP_STORE_CONNECT_API_KEY
  # APPLE_APP_STORE_CONNECT_PRIVATE_KEY
}
`

// testAccAPIClient builds an App Store Connect client from the same environment
// variables the provider reads.
//
// Destroy and existence checks have to observe Apple rather than Terraform
// state: after a destroy the resource is gone from state either way, so a check
// that only reads state passes even when the resource is still live in the
// portal. Every check below therefore goes through this client.
func testAccAPIClient() (*apple.Client, error) {
	return apple.NewClient(
		os.Getenv("APPLE_APP_STORE_CONNECT_ISSUER_ID"),
		os.Getenv("APPLE_APP_STORE_CONNECT_API_KEY"),
		os.Getenv("APPLE_APP_STORE_CONNECT_PRIVATE_KEY"),
		nil,
	)
}

// testAccStateResource returns the state for a resource address, failing with a
// useful message rather than a nil dereference when the address is absent.
func testAccStateResource(s *terraform.State, resourceName string) (*terraform.InstanceState, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return nil, fmt.Errorf("resource not found in state: %s", resourceName)
	}

	if rs.Primary == nil || rs.Primary.ID == "" {
		return nil, fmt.Errorf("resource %s has no ID in state", resourceName)
	}

	return rs.Primary, nil
}

// testAccCheckDestroyAll runs several destroy checks against the same final
// state, for configurations that create more than one kind of resource.
func testAccCheckDestroyAll(checks ...func(*terraform.State) error) func(*terraform.State) error {
	return func(s *terraform.State) error {
		for _, check := range checks {
			if err := check(s); err != nil {
				return err
			}
		}

		return nil
	}
}
