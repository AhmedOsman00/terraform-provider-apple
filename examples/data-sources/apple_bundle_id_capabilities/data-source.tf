# Copyright (c) HashiCorp, Inc.

# Data source to fetch all Bundle ID Capabilities
data "apple_bundle_id_capabilities" "all" {
  # No filters - returns all capabilities in the account
  limit = 100
}

# Filter capabilities by Bundle ID
data "apple_bundle_id_capabilities" "ios_app_capabilities" {
  bundle_id = "com.example.myapp"  # Can use Bundle identifier
  limit     = 50
}

# Filter capabilities by type
data "apple_bundle_id_capabilities" "push_notification_capabilities" {
  capability_type = "PUSH_NOTIFICATIONS"
  sort_by         = "bundle_id"
  sort_order      = "asc"
}

# Filter by both Bundle ID and capability type
data "apple_bundle_id_capabilities" "specific_capability" {
  bundle_id       = "com.example.myapp"
  capability_type = "ICLOUD"
}

# Get capabilities with custom sorting and limiting
data "apple_bundle_id_capabilities" "sorted_capabilities" {
  sort_by    = "capability_type"
  sort_order = "desc"
  limit      = 25
}

# Output examples showing how to use the data
output "all_capabilities_count" {
  description = "Total number of capabilities found"
  value       = length(data.apple_bundle_id_capabilities.all.capabilities)
}

output "ios_app_capability_types" {
  description = "List of capability types enabled for iOS app"
  value       = [for cap in data.apple_bundle_id_capabilities.ios_app_capabilities.capabilities : cap.capability_type]
}

output "push_notification_bundle_ids" {
  description = "Bundle IDs that have push notifications enabled"
  value       = [for cap in data.apple_bundle_id_capabilities.push_notification_capabilities.capabilities : cap.bundle_id]
}

# Example of using capability data in resources
locals {
  # Check if a Bundle ID already has push notifications
  has_push_notifications = length([
    for cap in data.apple_bundle_id_capabilities.ios_app_capabilities.capabilities :
    cap if cap.capability_type == "PUSH_NOTIFICATIONS"
  ]) > 0
}

# Conditionally create a push notification capability
resource "apple_bundle_id_capability" "conditional_push" {
  count = local.has_push_notifications ? 0 : 1
  
  bundle_id       = "com.example.myapp"
  capability_type = "PUSH_NOTIFICATIONS"
}