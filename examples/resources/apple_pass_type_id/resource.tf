# Copyright (c) HashiCorp, Inc.

# Create a Pass Type ID for loyalty card passes
resource "apple_pass_type_id" "loyalty_card" {
  identifier = "pass.com.example.loyalty"
  name       = "My Store Loyalty Card"
}

# Create a Pass Type ID for event tickets
resource "apple_pass_type_id" "event_tickets" {
  identifier = "pass.com.example.events"
  name       = "Event Tickets"
}

# Create a Pass Type ID for boarding passes
resource "apple_pass_type_id" "boarding_pass" {
  identifier = "pass.com.example.airline.boarding"
  name       = "Airline Boarding Passes"
}

# Create a Pass Type ID for store cards with detailed naming
resource "apple_pass_type_id" "store_card" {
  identifier = "pass.com.example.retail.storecard"
  name       = "Example Retail Store Card"
}

# Create a Pass Type ID for coupons
resource "apple_pass_type_id" "coupon_pass" {
  identifier = "pass.com.example.coupons"
  name       = "Promotional Coupons"
}

# Example of creating certificates for Pass Type IDs
# Note: This would require CSR generation outside of Terraform
resource "apple_certificate" "loyalty_pass_cert" {
  certificate_type = "PASS_TYPE_ID"
  csr_content     = file("${path.module}/loyalty_pass.csr")
  
  # This certificate will be associated with the Pass Type ID
  depends_on = [apple_pass_type_id.loyalty_card]
}

# Create a certificate with NFC capability for contactless passes
resource "apple_certificate" "nfc_pass_cert" {
  certificate_type = "PASS_TYPE_ID_WITH_NFC"
  csr_content     = file("${path.module}/nfc_pass.csr")
  
  # This certificate supports NFC for contactless interactions
  depends_on = [apple_pass_type_id.store_card]
}

# Query existing Pass Type IDs using data source
data "apple_pass_type_ids" "company_passes" {
  identifier_prefix = "pass.com.example"
  sort_by          = "name"
  depends_on = [
    apple_pass_type_id.loyalty_card,
    apple_pass_type_id.event_tickets,
    apple_pass_type_id.boarding_pass,
    apple_pass_type_id.store_card,
    apple_pass_type_id.coupon_pass,
  ]
}

# Output Pass Type ID information for use in pass generation
output "loyalty_pass_type_id" {
  description = "Pass Type ID for loyalty card passes"
  value = {
    id         = apple_pass_type_id.loyalty_card.id
    identifier = apple_pass_type_id.loyalty_card.identifier
    name       = apple_pass_type_id.loyalty_card.name
  }
}

output "event_pass_type_id" {
  description = "Pass Type ID for event ticket passes"
  value = {
    id         = apple_pass_type_id.event_tickets.id
    identifier = apple_pass_type_id.event_tickets.identifier
    name       = apple_pass_type_id.event_tickets.name
  }
}

# Example of using Pass Type IDs with local values for pass generation
locals {
  pass_type_configs = {
    loyalty = {
      pass_type_id = apple_pass_type_id.loyalty_card.identifier
      description  = "Used for customer loyalty programs"
      format_version = 1
      pass_type     = "storeCard"
    }
    events = {
      pass_type_id = apple_pass_type_id.event_tickets.identifier
      description  = "Used for event admission tickets"
      format_version = 1
      pass_type     = "eventTicket"
    }
    boarding = {
      pass_type_id = apple_pass_type_id.boarding_pass.identifier
      description  = "Used for airline boarding passes"
      format_version = 1
      pass_type     = "boardingPass"
    }
  }
}

# Output the pass configurations for external use
output "pass_configurations" {
  description = "Pass Type ID configurations for pass generation"
  value       = local.pass_type_configs
}