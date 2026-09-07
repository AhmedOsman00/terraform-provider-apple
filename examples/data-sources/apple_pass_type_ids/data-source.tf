# Copyright (c) HashiCorp, Inc.

# Query all Pass Type IDs
data "apple_pass_type_ids" "all" {}

# Query Pass Type IDs with identifier pattern matching
data "apple_pass_type_ids" "example_passes" {
  identifier_pattern = "^pass\\.com\\.example\\."
}

# Query Pass Type IDs with identifier prefix matching
data "apple_pass_type_ids" "company_passes" {
  identifier_prefix = "pass.com.example"
}

# Query Pass Type IDs with name pattern matching
data "apple_pass_type_ids" "loyalty_passes" {
  name_pattern = ".*[Ll]oyalty.*"
}

# Query Pass Type IDs for event-related passes
data "apple_pass_type_ids" "event_passes" {
  name_pattern = ".*[Ee]vent.*|.*[Tt]icket.*"
  sort_by      = "name"
  sort_order   = "asc"
}

# Query with multiple filters and sorting
data "apple_pass_type_ids" "filtered_sorted" {
  identifier_prefix = "pass.com.example"
  name_pattern      = ".*Card.*"
  sort_by           = "identifier"
  sort_order        = "desc"
  limit             = 5
}

# Query store-related Pass Type IDs
data "apple_pass_type_ids" "store_passes" {
  identifier_pattern = ".*\\.(store|retail|shop)\\."
  sort_by            = "name"
  limit              = 10
}

# Query passes with specific naming convention
data "apple_pass_type_ids" "mobile_wallet_passes" {
  name_pattern = ".*Mobile.*|.*Wallet.*"
  sort_by      = "identifier"
}

# Query all passes and limit results
data "apple_pass_type_ids" "limited_results" {
  limit      = 20
  sort_by    = "name"
  sort_order = "asc"
}

# Output the results
output "all_pass_type_ids" {
  description = "All Pass Type IDs in the account"
  value       = data.apple_pass_type_ids.all.pass_type_ids
}

output "example_pass_identifiers" {
  description = "List of example company pass identifiers"
  value       = [for pass in data.apple_pass_type_ids.company_passes.pass_type_ids : pass.identifier]
}

output "example_pass_names" {
  description = "List of example company pass names"
  value       = [for pass in data.apple_pass_type_ids.company_passes.pass_type_ids : pass.name]
}

output "loyalty_pass_details" {
  description = "Detailed information about loyalty passes"
  value = [
    for pass in data.apple_pass_type_ids.loyalty_passes.pass_type_ids : {
      id         = pass.id
      identifier = pass.identifier
      name       = pass.name
    }
  ]
}

# Output metadata from data sources
output "pass_counts" {
  description = "Pass Type ID counts and metadata"
  value = {
    total_passes   = data.apple_pass_type_ids.all.total_count
    company_passes = data.apple_pass_type_ids.company_passes.filtered_count
    loyalty_passes = data.apple_pass_type_ids.loyalty_passes.filtered_count
    event_passes   = data.apple_pass_type_ids.event_passes.filtered_count
  }
}

# Use Pass Type ID data in locals for conditional logic
locals {
  has_loyalty_passes = length(data.apple_pass_type_ids.loyalty_passes.pass_type_ids) > 0
  has_event_passes   = length(data.apple_pass_type_ids.event_passes.pass_type_ids) > 0

  pass_categories = {
    loyalty = [for pass in data.apple_pass_type_ids.loyalty_passes.pass_type_ids : pass.identifier]
    events  = [for pass in data.apple_pass_type_ids.event_passes.pass_type_ids : pass.identifier]
  }
}

# Conditional outputs based on data
output "has_passes" {
  description = "Boolean flags indicating presence of different pass types"
  value = {
    loyalty_passes = local.has_loyalty_passes
    event_passes   = local.has_event_passes
  }
}

output "pass_categories" {
  description = "Pass Type IDs organized by category"
  value       = local.pass_categories
}

# Example of using data source results to create resources
# This would typically be in a separate configuration file
resource "local_file" "pass_config" {
  count = length(data.apple_pass_type_ids.company_passes.pass_type_ids)

  filename = "${path.module}/pass_configs/${data.apple_pass_type_ids.company_passes.pass_type_ids[count.index].identifier}.json"
  content = jsonencode({
    passTypeIdentifier = data.apple_pass_type_ids.company_passes.pass_type_ids[count.index].identifier
    name               = data.apple_pass_type_ids.company_passes.pass_type_ids[count.index].name
    appleId            = data.apple_pass_type_ids.company_passes.pass_type_ids[count.index].id
    formatVersion      = 1
  })
}