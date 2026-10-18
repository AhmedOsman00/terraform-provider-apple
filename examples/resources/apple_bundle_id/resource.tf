# Create a Bundle ID for iOS platform
resource "apple_bundle_id" "ios_app" {
  identifier = "com.example.myapp"
  name       = "My iOS App"
  platform   = "IOS"
}

# Create a Bundle ID for macOS platform
resource "apple_bundle_id" "macos_app" {
  identifier = "com.example.myapp.macos"
  name       = "My macOS App"
  platform   = "MAC_OS"
}

# The seed ID (team prefix) is assigned by Apple and exported as a computed
# attribute; it cannot be set in configuration.
resource "apple_bundle_id" "premium_app" {
  identifier = "com.example.myapp.premium"
  name       = "My Premium App"
  platform   = "IOS"
}

output "premium_app_seed_id" {
  description = "Team prefix Apple assigned to the premium Bundle ID"
  value       = apple_bundle_id.premium_app.seed_id
}

# Bundle ID Capabilities - Add capabilities to enable app features
resource "apple_bundle_id_capability" "push_notifications" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "PUSH_NOTIFICATIONS"
  # Push notifications don't require additional settings
}

resource "apple_bundle_id_capability" "in_app_purchase" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "IN_APP_PURCHASE"
  # In-app purchases don't require additional settings
}

# ICLOUD_VERSION is the only iCloud setting: XCODE_6 is CloudKit. The iCloud
# containers are assigned in the Developer portal; the API has no field for them.
resource "apple_bundle_id_capability" "icloud" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "ICLOUD"

  settings = [
    {
      key   = "ICLOUD_VERSION"
      value = "XCODE_6"
    },
  ]
}

# App Groups takes no settings; the group is assigned in the Developer portal.
resource "apple_bundle_id_capability" "app_groups" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "APP_GROUPS"
}

# Create Merchant ID for Apple Pay
resource "apple_merchant_id" "ios_app_merchant" {
  identifier   = "merchant.com.example.myapp"
  display_name = "My iOS App Merchant"
}

# Apple Pay takes no settings. The Merchant ID above is associated with the App
# ID in the Developer portal; the API has no field for it.
resource "apple_bundle_id_capability" "apple_pay" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "APPLE_PAY"
}

# Associated Domains takes no settings; the domains live in the entitlements file.
resource "apple_bundle_id_capability" "associated_domains" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "ASSOCIATED_DOMAINS"
}

# Capability for macOS app - HomeKit
resource "apple_bundle_id_capability" "homekit_macos" {
  bundle_id       = apple_bundle_id.macos_app.id
  capability_type = "HOMEKIT"
  # HomeKit doesn't require additional settings for basic functionality
}

# Additional Merchant IDs for different purposes
resource "apple_merchant_id" "premium_features" {
  identifier   = "merchant.com.example.myapp.premium"
  display_name = "My App Premium Features"
}

resource "apple_merchant_id" "subscriptions" {
  identifier   = "merchant.com.example.myapp.subscriptions"
  display_name = "My App Subscriptions"
}

# Query existing Merchant IDs using data source
data "apple_merchant_ids" "example_merchants" {
  identifier_prefix = "merchant.com.example"
  sort_by           = "display_name"
  depends_on = [
    apple_merchant_id.ios_app_merchant,
    apple_merchant_id.premium_features,
    apple_merchant_id.subscriptions,
  ]
}
