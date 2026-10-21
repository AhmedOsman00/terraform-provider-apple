# App Store listing for Example (com.example.app, app ID 6478123456).

import {
  to = apple_app_settings.com_example_app
  id = "6478123456"
}
resource "apple_app_settings" "com_example_app" {
  app_id                     = "6478123456"
  primary_locale             = "en-US"
  content_rights_declaration = "DOES_NOT_USE_THIRD_PARTY_CONTENT"
  subscription_status_url    = "https://example.com/asn"
}

import {
  to = apple_app_info.com_example_app
  id = "6478123456"
}
resource "apple_app_info" "com_example_app" {
  app_id           = "6478123456"
  primary_category = "PRODUCTIVITY"
}

import {
  to = apple_app_info_localization.com_example_app_en_us
  id = "6478123456/en-US"
}
resource "apple_app_info_localization" "com_example_app_en_us" {
  app_id             = "6478123456"
  locale             = "en-US"
  name               = "Example"
  subtitle           = "Gets things $${done}"
  privacy_policy_url = "https://example.com/privacy"
}

import {
  to = apple_app_age_rating_declaration.com_example_app
  id = "6478123456"
}
resource "apple_app_age_rating_declaration" "com_example_app" {
  app_id                  = "6478123456"
  contests                = "NONE"
  violence_realistic      = "NONE"
  gambling                = false
  unrestricted_web_access = false
}

import {
  to = apple_app_store_version.com_example_app_ios
  id = "VER_NEW"
}
resource "apple_app_store_version" "com_example_app_ios" {
  app_id         = "6478123456"
  platform       = "IOS"
  version_string = "1.10"
  copyright      = "2026 Example Ltd"
  release_type   = "MANUAL"
  build_number   = "42"
}

import {
  to = apple_app_store_version_localization.com_example_app_ios_en_us
  id = "VER_NEW/en-US"
}
resource "apple_app_store_version_localization" "com_example_app_ios_en_us" {
  app_store_version_id = apple_app_store_version.com_example_app_ios.id
  locale               = "en-US"
  description          = "Line one.\nLine two."
  keywords             = ["tasks", "todo", "lists"]
  support_url          = "https://example.com/help"
}

import {
  to = apple_app_store_review_detail.com_example_app_ios
  id = "VER_NEW"
}
resource "apple_app_store_review_detail" "com_example_app_ios" {
  app_store_version_id  = apple_app_store_version.com_example_app_ios.id
  contact_email         = "review@example.com"
  demo_account_name     = "demo"
  demo_account_required = true
  demo_account_password = var.app_review_com_example_app_ios_demo_password
}

import {
  to = apple_app_store_version.com_example_app_mac_os
  id = "VER_MAC"
}
resource "apple_app_store_version" "com_example_app_mac_os" {
  app_id         = "6478123456"
  platform       = "MAC_OS"
  version_string = "2.0"
}

import {
  to = apple_app_price_schedule.com_example_app
  id = "6478123456"
}
resource "apple_app_price_schedule" "com_example_app" {
  app_id         = "6478123456"
  base_territory = "USA"
  prices = [
    {
      price_point_id = "eyJzIjoiMTAwMDAifQ"
    },
  ]
}

import {
  to = apple_app_availability.com_example_app
  id = "6478123456"
}
resource "apple_app_availability" "com_example_app" {
  app_id                       = "6478123456"
  available_in_new_territories = true
  territories = [
    {
      release_date = "2025-01-01"
      territory    = "GBR"
    },
    {
      territory = "USA"
    },
  ]
}

