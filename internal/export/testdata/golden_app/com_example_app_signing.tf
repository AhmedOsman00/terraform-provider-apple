import {
  to = apple_bundle_id.com_example_app
  id = "com.example.app"
}
resource "apple_bundle_id" "com_example_app" {
  identifier = "com.example.app"
  name       = "Example App"
  platform   = "IOS"
}

import {
  to = apple_bundle_id.com_example_app_widget
  id = "com.example.app.widget"
}
resource "apple_bundle_id" "com_example_app_widget" {
  identifier = "com.example.app.widget"
  name       = "Example Widget"
  platform   = "IOS"
}

data "apple_certificates" "example_distribution_distribution" {
  serial_number = "6A1B2C3D4E5F"
}

data "apple_devices" "ada_s_iphone" {
  udid_pattern = "^00008030-000A4D8E0AB8802E$"
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
  to = apple_profile.example_widget_ad_hoc
  id = "PROFW"
}
resource "apple_profile" "example_widget_ad_hoc" {
  name         = "Example Widget Ad Hoc"
  profile_type = "IOS_APP_ADHOC"
  bundle_id    = apple_bundle_id.com_example_app_widget.id
  certificates = [data.apple_certificates.example_distribution_distribution.certificates[0].id]
  devices      = [data.apple_devices.ada_s_iphone.devices[0].id, "DEV2"]
}

