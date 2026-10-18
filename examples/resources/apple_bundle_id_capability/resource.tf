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

# iCloud with CloudKit. ICLOUD_VERSION is the only iCloud setting Apple
# accepts: XCODE_6 is CloudKit, XCODE_5 the legacy iCloud Documents. Which
# iCloud containers the App ID uses is not part of the API at all -- assign
# them in the Developer portal.
resource "apple_bundle_id_capability" "icloud" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "ICLOUD"

  settings = [
    {
      key   = "ICLOUD_VERSION"
      value = "XCODE_6"
    },
  ]
}

# App Groups takes no settings. The API enables the capability but cannot
# create a group or assign one to the App ID; that is Developer portal work.
resource "apple_bundle_id_capability" "app_groups" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "APP_GROUPS"
}

# Apple Pay takes no settings either. A Merchant ID can be created with
# apple_merchant_id, but associating it with the App ID happens in the
# Developer portal.
resource "apple_bundle_id_capability" "apple_pay" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "APPLE_PAY"
}

# Associated Domains takes no settings. The domains themselves are listed in
# the app's entitlements file, not on the App ID.
resource "apple_bundle_id_capability" "associated_domains" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "ASSOCIATED_DOMAINS"
}

# Game Center capability
resource "apple_bundle_id_capability" "game_center" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "GAME_CENTER"
}

# HealthKit capability
resource "apple_bundle_id_capability" "health_kit" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "HEALTHKIT"
}

# SiriKit capability
resource "apple_bundle_id_capability" "siri" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "SIRIKIT"
}

# Personal VPN capability
resource "apple_bundle_id_capability" "personal_vpn" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "PERSONAL_VPN"
}

# Wallet capability for creating passes
resource "apple_bundle_id_capability" "wallet_passes" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "WALLET"
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