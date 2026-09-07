// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccCertificatesDataSource_basic issues one certificate and reads it back
// through the data source, filtering on the serial number Apple assigned so the
// assertions hold whatever else the account contains.
func TestAccCertificatesDataSource_basic(t *testing.T) {
	csr := testAccCertificateCSR(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCertificatesDataSourceConfig(csr),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Unfiltered: only that the collection came back.
					resource.TestCheckResourceAttrSet("data.apple_certificates.all", "certificates.#"),
					resource.TestCheckResourceAttrSet("data.apple_certificates.all", "total_count"),
					resource.TestCheckResourceAttrSet("data.apple_certificates.all", "filtered_count"),

					// Filtered by serial number: exactly the one just issued.
					resource.TestCheckResourceAttr("data.apple_certificates.by_serial", "filtered_count", "1"),
					resource.TestCheckResourceAttr("data.apple_certificates.by_serial", "certificates.#", "1"),
					resource.TestCheckResourceAttrPair(
						"data.apple_certificates.by_serial", "certificates.0.id",
						"apple_certificate.test", "id",
					),
					resource.TestCheckResourceAttr("data.apple_certificates.by_serial", "certificates.0.certificate_type", testAccCertificateType),

					// Type filter: the certificate just issued has to be in it,
					// so the count cannot be zero.
					resource.TestMatchResourceAttr("data.apple_certificates.by_type", "filtered_count", regexp.MustCompile(`^[1-9][0-9]*$`)),

					// A limit of 1 truncates the list without changing the
					// pre-limit count.
					resource.TestCheckResourceAttr("data.apple_certificates.limited", "certificates.#", "1"),
				),
			},
		},
	})
}

func TestAccCertificatesDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "apple_certificates" "test" {
  certificate_type = "NOT_A_CERTIFICATE_TYPE"
}
`,
				ExpectError: regexp.MustCompile(`Attribute certificate_type value must be one of`),
			},
			{
				Config: `
data "apple_certificates" "test" {
  limit = 500
}
`,
				ExpectError: regexp.MustCompile(`Attribute limit value must be between 1 and 200`),
			},
			{
				Config: `
data "apple_certificates" "test" {
  sort_by = "not_a_field"
}
`,
				ExpectError: regexp.MustCompile(`Attribute sort_by value must be one of`),
			},
		},
	})
}

func testAccCertificatesDataSourceConfig(csr string) string {
	return fmt.Sprintf(`
resource "apple_certificate" "test" {
  certificate_type = %[1]q
  csr_content      = %[2]q
}

data "apple_certificates" "all" {
  depends_on = [apple_certificate.test]
}

data "apple_certificates" "by_serial" {
  serial_number = apple_certificate.test.serial_number
}

data "apple_certificates" "by_type" {
  certificate_type = apple_certificate.test.certificate_type
}

data "apple_certificates" "limited" {
  certificate_type = apple_certificate.test.certificate_type
  limit            = 1
}
`, testAccCertificateType, csr)
}
