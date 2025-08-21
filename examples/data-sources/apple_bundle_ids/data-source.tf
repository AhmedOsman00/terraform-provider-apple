# Copyright (c) HashiCorp, Inc.

# Query all Bundle IDs
data "apple_bundle_ids" "all" {}

# Query Bundle IDs for iOS platform only
data "apple_bundle_ids" "ios_only" {
  platform = "IOS"
}

# Query Bundle IDs with identifier pattern matching
data "apple_bundle_ids" "example_apps" {
  identifier_contains = "com.example"
}

# Query Bundle IDs with name pattern matching
data "apple_bundle_ids" "my_apps" {
  name_contains = "My"
}

# Query with multiple filters and sorting
data "apple_bundle_ids" "filtered_sorted" {
  platform            = "IOS"
  identifier_contains = "com.example"
  sort_by             = "identifier"
  limit               = 10
}

# Output the results
output "all_bundle_ids" {
  value = data.apple_bundle_ids.all.bundle_ids
}

output "ios_bundle_identifiers" {
  value = [for bid in data.apple_bundle_ids.ios_only.bundle_ids : bid.identifier]
}