# Copyright (c) HashiCorp, Inc.

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

# Create a Bundle ID with explicit seed ID (optional)
resource "apple_bundle_id" "app_with_seed" {
  identifier = "com.example.myapp.premium"
  name       = "My Premium App"
  platform   = "IOS"
  seed_id    = "ABCD123456"
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

resource "apple_bundle_id_capability" "icloud" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "ICLOUD"
  
  # iCloud requires settings configuration
  settings {
    key   = "ICLOUD_VERSION"
    value = "XCODE_5"
  }
  
  settings {
    key   = "ICLOUD_SERVICES"
    value = "CloudKit"
  }
}

resource "apple_bundle_id_capability" "app_groups" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "APP_GROUPS"
  
  # App Groups allows data sharing between apps
  settings {
    key   = "APP_GROUPS"
    value = "group.com.example.myapp.shared"
  }
}

resource "apple_bundle_id_capability" "apple_pay" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "APPLE_PAY"
  
  # Apple Pay configuration
  settings {
    key   = "APPLE_PAY_IDENTIFIERS"
    value = "merchant.com.example.myapp"
  }
}

resource "apple_bundle_id_capability" "associated_domains" {
  bundle_id       = apple_bundle_id.ios_app.id
  capability_type = "ASSOCIATED_DOMAINS"
  
  # Associated domains for universal links and app clips
  settings {
    key   = "ASSOCIATED_DOMAINS"
    value = "applinks:example.com,appclips:clips.example.com"
  }
}

# Capability for macOS app - HomeKit
resource "apple_bundle_id_capability" "homekit_macos" {
  bundle_id       = apple_bundle_id.macos_app.id
  capability_type = "HOME_KIT"
  # HomeKit doesn't require additional settings for basic functionality
}