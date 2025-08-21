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