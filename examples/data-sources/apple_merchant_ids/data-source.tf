# Copyright (c) HashiCorp, Inc.

# Query all Merchant IDs
data "apple_merchant_ids" "all" {}

# Query Merchant IDs with identifier prefix matching
data "apple_merchant_ids" "example_merchants" {
  identifier_prefix = "merchant.com.example"
}

# Query Merchant IDs with identifier pattern matching (regex)
data "apple_merchant_ids" "app_merchants" {
  identifier_pattern = "merchant\\.com\\.example\\..*\\.app"
}

# Query Merchant IDs with display name pattern matching
data "apple_merchant_ids" "store_merchants" {
  display_name_pattern = ".*Store.*"
}

# Query with multiple filters and sorting
data "apple_merchant_ids" "filtered_sorted" {
  identifier_prefix = "merchant.com.example"
  sort_by           = "display_name"
  sort_order        = "asc"
  limit             = 10
}

# Query with sorting by identifier descending
data "apple_merchant_ids" "sorted_by_id" {
  sort_by    = "identifier"
  sort_order = "desc"
  limit      = 5
}

# Output the results
output "all_merchant_ids" {
  value = data.apple_merchant_ids.all.merchant_ids
}

output "example_merchant_identifiers" {
  value = [for mid in data.apple_merchant_ids.example_merchants.merchant_ids : mid.identifier]
}

output "merchant_count_info" {
  value = {
    total_count    = data.apple_merchant_ids.all.total_count
    filtered_count = data.apple_merchant_ids.filtered_sorted.filtered_count
  }
}

# Example of using a Merchant ID data source result in a resource
data "apple_merchant_ids" "my_merchants" {
  identifier_prefix = "merchant.com.mycompany"
  limit             = 1
}

variable "bundle_id" {
  description = "Apple-generated ID of an existing Bundle ID to attach the Apple Pay capability to."
  type        = string
}

resource "apple_bundle_id_capability" "apple_pay_from_data" {
  # Any Bundle ID works here: a variable, or apple_bundle_id.example.id
  bundle_id       = var.bundle_id
  capability_type = "APPLE_PAY"

  # Use the first merchant ID found in the data source
  settings = [
    {
      key   = "APPLE_PAY_IDENTIFIERS"
      value = length(data.apple_merchant_ids.my_merchants.merchant_ids) > 0 ? data.apple_merchant_ids.my_merchants.merchant_ids[0].identifier : "merchant.default"
    },
  ]
}