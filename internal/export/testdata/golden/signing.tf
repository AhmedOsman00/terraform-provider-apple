import {
  to = apple_bundle_id.com_example_app
  id = "com.example.app"
}
resource "apple_bundle_id" "com_example_app" {
  identifier = "com.example.app"
  name       = "Example App"
  platform   = "UNIVERSAL"
}

import {
  to = apple_bundle_id_capability.com_example_app_push_notifications
  id = "B2C3D4E5F6/B2C3D4E5F6_PUSH_NOTIFICATIONS"
}
resource "apple_bundle_id_capability" "com_example_app_push_notifications" {
  bundle_id       = apple_bundle_id.com_example_app.id
  capability_type = "PUSH_NOTIFICATIONS"
}

import {
  to = apple_bundle_id_capability.com_example_app_icloud
  id = "B2C3D4E5F6/B2C3D4E5F6_ICLOUD"
}
resource "apple_bundle_id_capability" "com_example_app_icloud" {
  bundle_id       = apple_bundle_id.com_example_app.id
  capability_type = "ICLOUD"
  settings = [
    {
      key   = "ICLOUD_VERSION"
      value = "XCODE_6"
    },
  ]
}

import {
  to = apple_device.ada_s_iphone
  id = "00008030-000A4D8E0AB8802E"
}
resource "apple_device" "ada_s_iphone" {
  name     = "Ada's iPhone"
  udid     = "00008030-000A4D8E0AB8802E"
  platform = "IOS"
}

import {
  to = apple_merchant_id.merchant_com_example_app
  id = "merchant.com.example.app"
}
resource "apple_merchant_id" "merchant_com_example_app" {
  identifier   = "merchant.com.example.app"
  display_name = "Example Pay"
}

data "apple_certificates" "example_distribution_distribution" {
  serial_number = "6A1B2C3D4E5F"
}

import {
  to = apple_profile.example_app_store
  id = "PROF1"
}
resource "apple_profile" "example_app_store" {
  name         = "Example App Store"
  profile_type = "IOS_APP_STORE"
  bundle_id    = apple_bundle_id.com_example_app.id
  certificates = [data.apple_certificates.example_distribution_distribution.certificates[0].id]
}

import {
  to = apple_profile.example_ad_hoc
  id = "PROF2"
}
resource "apple_profile" "example_ad_hoc" {
  name         = "Example Ad Hoc"
  profile_type = "IOS_APP_ADHOC"
  bundle_id    = apple_bundle_id.com_example_app.id
  certificates = [data.apple_certificates.example_distribution_distribution.certificates[0].id, "CERT_GONE"]
  devices      = [apple_device.ada_s_iphone.id]
}

