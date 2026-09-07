// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Certificates are the one resource here that consumes a limited allowance:
// Apple caps how many of each type a team may hold at once. Destroy revokes,
// so a completed run gives the slot back, but a run interrupted between create
// and destroy leaves a certificate that has to be revoked in the portal.
//
// IOS_DEVELOPMENT is used throughout rather than a distribution type because
// its allowance is the larger of the two.
const testAccCertificateType = "IOS_DEVELOPMENT"

func TestAccCertificateResource_basic(t *testing.T) {
	// One CSR for the whole test: csr_content forces replacement, so
	// regenerating it between steps would reissue the certificate every step
	// and hide whether anything else caused the replacement.
	csr := testAccCertificateCSR(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCertificateResourceConfig(testAccCertificateType, csr, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckCertificateExists("apple_certificate.test"),
					resource.TestCheckResourceAttr("apple_certificate.test", "certificate_type", testAccCertificateType),
					resource.TestCheckResourceAttrSet("apple_certificate.test", "id"),
					resource.TestCheckResourceAttrSet("apple_certificate.test", "serial_number"),
					resource.TestCheckResourceAttrSet("apple_certificate.test", "certificate_content"),
					resource.TestCheckResourceAttrSet("apple_certificate.test", "display_name"),
					resource.TestCheckResourceAttrSet("apple_certificate.test", "expiration_date"),
					// Recorded as false between runs so that flipping it to
					// true during a plan registers as a change.
					resource.TestCheckResourceAttr("apple_certificate.test", "ready_for_renewal", "false"),
				),
			},
			// Import by serial number. csr_content is ignored because Apple
			// does not return the CSR a certificate was issued from, and
			// early_renewal_hours because it is configuration rather than
			// anything Apple stores.
			{
				ResourceName:            "apple_certificate.test",
				ImportState:             true,
				ImportStateIdFunc:       testAccCertificateImportIDFunc("apple_certificate.test", "serial_number"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"csr_content", "early_renewal_hours"},
			},
			// Import by the display name Apple assigned.
			{
				ResourceName:            "apple_certificate.test",
				ImportState:             true,
				ImportStateIdFunc:       testAccCertificateImportIDFunc("apple_certificate.test", "display_name"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"csr_content", "early_renewal_hours"},
			},
		},
	})
}

// TestAccCertificateResource_earlyRenewal walks the whole early renewal
// mechanism: adding the window is an in-place update, and widening it past the
// certificate's remaining life is what forces the replacement.
func TestAccCertificateResource_earlyRenewal(t *testing.T) {
	csr := testAccCertificateCSR(t)

	var serialBefore, serialAfter string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCertificateResourceConfig(testAccCertificateType, csr, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckCertificateExists("apple_certificate.test"),
					testAccCaptureAttr("apple_certificate.test", "serial_number", &serialBefore),
					resource.TestCheckResourceAttr("apple_certificate.test", "ready_for_renewal", "false"),
				),
			},
			// An hour before expiry is a window the certificate is nowhere
			// near, so Update takes the new value in place and keeps the
			// issued certificate.
			{
				Config: testAccCertificateResourceConfig(testAccCertificateType, csr, "early_renewal_hours = 1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("apple_certificate.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_certificate.test", "early_renewal_hours", "1"),
					resource.TestCheckResourceAttr("apple_certificate.test", "ready_for_renewal", "false"),
					resource.TestCheckResourceAttrPtr("apple_certificate.test", "serial_number", &serialBefore),
				),
			},
			// A window wider than an Apple certificate's one-year validity puts
			// it permanently inside the renewal window, which is exactly the
			// footgun the schema documents -- and the only way to observe the
			// replacement without waiting a year.
			{
				Config: testAccCertificateResourceConfig(testAccCertificateType, csr, "early_renewal_hours = 99999"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("apple_certificate.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckCertificateExists("apple_certificate.test"),
					testAccCaptureAttr("apple_certificate.test", "serial_number", &serialAfter),
					// Create records false again, so the next plan can flip it.
					resource.TestCheckResourceAttr("apple_certificate.test", "ready_for_renewal", "false"),
					func(*terraform.State) error {
						if serialBefore == serialAfter {
							return fmt.Errorf("certificate was not reissued: serial number is still %s", serialAfter)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccCertificateResource_requiresReplace covers certificate_type, which
// like every other attribute describing the certificate is immutable.
func TestAccCertificateResource_requiresReplace(t *testing.T) {
	csr := testAccCertificateCSR(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCertificateResourceConfig("IOS_DEVELOPMENT", csr, ""),
				Check:  testAccCheckCertificateExists("apple_certificate.test"),
			},
			{
				Config: testAccCertificateResourceConfig("IOS_DISTRIBUTION", csr, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("apple_certificate.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckCertificateExists("apple_certificate.test"),
					resource.TestCheckResourceAttr("apple_certificate.test", "certificate_type", "IOS_DISTRIBUTION"),
				),
			},
		},
	})
}

// TestAccCertificateResource_validation is rejected at plan time and issues
// nothing.
func TestAccCertificateResource_validation(t *testing.T) {
	csr := testAccCertificateCSR(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCertificateResourceConfig("NOT_A_CERTIFICATE_TYPE", csr, ""),
				ExpectError: regexp.MustCompile(`Attribute certificate_type value must be one of`),
			},
			{
				Config:      testAccCertificateResourceConfig(testAccCertificateType, "not a csr", ""),
				ExpectError: regexp.MustCompile(`CSR content must be a valid PEM-encoded certificate signing request`),
			},
			{
				Config:      testAccCertificateResourceConfig(testAccCertificateType, csr, "early_renewal_hours = -1"),
				ExpectError: regexp.MustCompile(`Attribute early_renewal_hours value must be at least 0`),
			},
		},
	})
}

// testAccCertificateCSR generates a PEM-encoded certificate signing request.
//
// Generating it in Go keeps the test to a single provider: pulling in
// hashicorp/tls would make every certificate test depend on a Registry
// download. The subject mirrors what examples/signing sends through
// tls_cert_request, since that is the shape Apple is known to accept.
func testAccCertificateCSR(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key for CSR: %s", err)
	}

	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   "terraform-provider-apple acceptance test",
			Organization: []string{"terraform-provider-apple"},
		},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}, key)
	if err != nil {
		t.Fatalf("generating CSR: %s", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}

// testAccCertificateResourceConfig renders a certificate resource. extra is
// injected verbatim so a step can add early_renewal_hours without a second
// config function.
func testAccCertificateResourceConfig(certificateType, csr, extra string) string {
	return fmt.Sprintf(`
resource "apple_certificate" "test" {
  certificate_type = %[1]q
  csr_content      = %[2]q

  %[3]s
}
`, certificateType, csr, extra)
}

// testAccCertificateImportIDFunc imports by one of the human identifiers
// ImportState accepts, read out of the applied state.
func testAccCertificateImportIDFunc(resourceName, attribute string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		is, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}

		value := is.Attributes[attribute]
		if value == "" {
			return "", fmt.Errorf("%s has no %s in state to import by", resourceName, attribute)
		}

		return value, nil
	}
}

// testAccCaptureAttr records an attribute so a later step can compare against
// it -- the only way to assert that a replacement actually reissued rather
// than kept the same certificate.
func testAccCaptureAttr(resourceName, attribute string, into *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		is, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		value := is.Attributes[attribute]
		if value == "" {
			return fmt.Errorf("%s has no %s in state to capture", resourceName, attribute)
		}

		*into = value

		return nil
	}
}

func testAccCheckCertificateExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		is, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("building API client: %w", err)
		}

		certificate, err := client.GetCertificate(is.ID)
		if err != nil {
			return fmt.Errorf("reading Certificate %s from Apple: %w", is.ID, err)
		}

		if got := certificate.Attributes.SerialNumber; got != is.Attributes["serial_number"] {
			return fmt.Errorf("Certificate %s has serial %q at Apple, state says %q", is.ID, got, is.Attributes["serial_number"])
		}

		return nil
	}
}

// testAccCheckCertificateDestroy asserts the certificate is no longer listed.
// Revocation is checked through the collection rather than a GET by ID because
// a revoked certificate drops out of the list, which holds whether or not
// Apple keeps answering for the ID.
func testAccCheckCertificateDestroy(s *terraform.State) error {
	var ids []string
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "apple_certificate" || rs.Primary == nil {
			continue
		}
		ids = append(ids, rs.Primary.ID)
	}

	if len(ids) == 0 {
		return nil
	}

	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("building API client: %w", err)
	}

	certificates, err := client.GetCertificates()
	if err != nil {
		return fmt.Errorf("listing certificates: %w", err)
	}

	live := make(map[string]struct{}, len(certificates))
	for _, certificate := range certificates {
		live[certificate.ID] = struct{}{}
	}

	for _, id := range ids {
		if _, ok := live[id]; ok {
			return fmt.Errorf("Certificate %s was not revoked: it is still listed by Apple", id)
		}
	}

	return nil
}
