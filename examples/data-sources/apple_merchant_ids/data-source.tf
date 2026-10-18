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

# The API cannot associate a Merchant ID with an App ID, so a lookup like this
# feeds outputs or other tooling rather than an apple_bundle_id_capability.
data "apple_merchant_ids" "my_merchants" {
  identifier_prefix = "merchant.com.mycompany"
  limit             = 1
}

output "first_merchant_identifier" {
  value = one(data.apple_merchant_ids.my_merchants.merchant_ids[*].identifier)
}
