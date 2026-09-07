# Copyright (c) HashiCorp, Inc.

# Create a Bundle ID first (required for capabilities)
resource "apple_bundle_id" "example_app" {
  identifier = "com.example.capabilities-demo"
  name       = "Capabilities Demo App"
  platform   = "IOS"
}

# Basic capability without settings - Push Notifications
resource "apple_bundle_id_capability" "push_notifications" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "PUSH_NOTIFICATIONS"
}

# Basic capability without settings - In-App Purchase
resource "apple_bundle_id_capability" "in_app_purchase" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "IN_APP_PURCHASE"
}

# Capability with settings - iCloud
resource "apple_bundle_id_capability" "icloud" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "ICLOUD"

  # Configure iCloud settings
  settings = [
    {
      key   = "ICLOUD_VERSION"
      value = "XCODE_5"
    },
    {
      key   = "ICLOUD_SERVICES"
      value = "CloudKit"
    },
  ]
}

# Capability with multiple settings - App Groups
resource "apple_bundle_id_capability" "app_groups" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "APP_GROUPS"

  settings = [
    {
      key   = "APP_GROUPS"
      value = "group.com.example.capabilities-demo.shared"
    },
  ]
}

# Apple Pay capability with merchant identifier
resource "apple_bundle_id_capability" "apple_pay" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "APPLE_PAY"

  settings = [
    {
      key   = "APPLE_PAY_IDENTIFIERS"
      value = "merchant.com.example.capabilities-demo"
    },
  ]
}

# Associated Domains for universal links and app clips
resource "apple_bundle_id_capability" "associated_domains" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "ASSOCIATED_DOMAINS"

  settings = [
    {
      key   = "ASSOCIATED_DOMAINS"
      value = "applinks:example.com,appclips:clips.example.com,webcredentials:auth.example.com"
    },
  ]
}

# Game Center capability
resource "apple_bundle_id_capability" "game_center" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "GAME_CENTER"
}

# HealthKit capability
resource "apple_bundle_id_capability" "health_kit" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "HEALTH_KIT"
}

# SiriKit capability
resource "apple_bundle_id_capability" "siri" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "SIRI"
}

# Personal VPN capability
resource "apple_bundle_id_capability" "personal_vpn" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "PERSONAL_VPN"
}

# Wallet capability for creating passes
resource "apple_bundle_id_capability" "wallet_passes" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "WALLET_PASSES"

  settings = [
    {
      key   = "WALLET_PASS_TEAM_ID"
      value = "ABCD123456"
    },
  ]
}

# Wireless Accessory Configuration
resource "apple_bundle_id_capability" "wireless_accessory" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "WIRELESS_ACCESSORY_CONFIGURATION"
}

# Data Protection capability with an explicit protection level.
# Background modes are an entitlement, not a Bundle ID capability, so they are
# configured in Xcode rather than here.
resource "apple_bundle_id_capability" "data_protection" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "DATA_PROTECTION"

  settings = [
    {
      key   = "DATA_PROTECTION_PERMISSION_LEVEL"
      value = "COMPLETE_PROTECTION"
    },
  ]
}