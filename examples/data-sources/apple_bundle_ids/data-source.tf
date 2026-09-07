# Copyright (c) HashiCorp, Inc.

# Query all Bundle IDs
data "apple_bundle_ids" "all" {}

# Query Bundle IDs for iOS platform only
data "apple_bundle_ids" "ios_only" {
  platform = "IOS"
}

# Query Bundle IDs whose identifier starts with a prefix
data "apple_bundle_ids" "example_apps" {
  identifier_prefix = "com.example"
}

# Query Bundle IDs with identifier pattern matching (regular expression)
data "apple_bundle_ids" "beta_apps" {
  identifier_pattern = "^com\\.example\\..*\\.beta$"
}

# Query Bundle IDs with name pattern matching (glob, matched against the whole name)
data "apple_bundle_ids" "my_apps" {
  name_pattern = "My *"
}

# Query with multiple filters and sorting
data "apple_bundle_ids" "filtered_sorted" {
  platform          = "IOS"
  identifier_prefix = "com.example"
  sort_by           = "identifier"
  limit             = 10
}

# Output the results
output "all_bundle_ids" {
  value = data.apple_bundle_ids.all.bundle_ids
}

output "ios_bundle_identifiers" {
  value = [for bid in data.apple_bundle_ids.ios_only.bundle_ids : bid.identifier]
}