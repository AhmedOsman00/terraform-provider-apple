terraform {
  required_providers {
    apple = {
      source = "theaostudio.com/hashicorp/apple"
    }
  }
}

# Configure the Apple Provider
provider "apple" {
  # Authentication via environment variables:
  # APPLE_APP_STORE_CONNECT_ISSUER_ID
  # APPLE_APP_STORE_CONNECT_API_KEY
  # APPLE_APP_STORE_CONNECT_PRIVATE_KEY
}

# Create a new Bundle ID
resource "apple_bundle_id" "example_app" {
  identifier = "com.example.myapp"
  name       = "My Example App"
  platform   = "IOS"
}

# Add capabilities to the Bundle ID
resource "apple_bundle_id_capability" "push_notifications" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "PUSH_NOTIFICATIONS"
}

resource "apple_bundle_id_capability" "in_app_purchase" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "IN_APP_PURCHASE"
}

resource "apple_bundle_id_capability" "icloud" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "ICLOUD"
  
  settings {
    key   = "ICLOUD_VERSION"
    value = "XCODE_5"
  }
  
  settings {
    key   = "ICLOUD_SERVICES"
    value = "CloudKit"
  }
}

# Query existing Bundle IDs
data "apple_bundle_ids" "ios_apps" {
  platform = "IOS"
}

# Query Bundle ID capabilities
data "apple_bundle_id_capabilities" "example_app_capabilities" {
  bundle_id = apple_bundle_id.example_app.id
}

# Example CSR for certificate creation
locals {
  example_csr = <<-EOT
    -----BEGIN CERTIFICATE REQUEST-----
    MIICljCCAX4CAQAwUTELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMRYwFAYDVQQH
    DA1TYW4gRnJhbmNpc2NvMR0wGwYDVQQKDBRFeGFtcGxlIENvbXBhbnksIEluYzCC
    ASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAM7K2+Xk3nWoQ5j+9fGOZjXf
    ... (truncated for example - use a real CSR)
    -----END CERTIFICATE REQUEST-----
  EOT
}

# Create certificates for development and distribution
resource "apple_certificate" "ios_development" {
  certificate_type = "IOS_DEVELOPMENT"
  csr_content      = local.example_csr
}

resource "apple_certificate" "ios_distribution" {
  certificate_type = "IOS_DISTRIBUTION"
  csr_content      = local.example_csr
}

# Query existing certificates
data "apple_certificates" "development_certificates" {
  certificate_type = "IOS_DEVELOPMENT"
  platform         = "IOS"
  sort_by          = "expiration_date"
  sort_order       = "desc"
  limit            = 5
}

# Query all certificates with name pattern
data "apple_certificates" "production_certificates" {
  name_pattern = "*Production*"
  sort_by      = "display_name"
}

# Register development devices
resource "apple_device" "dev_iphone" {
  name     = "Development iPhone"
  udid     = "12345678-90123456789012345678901234567890" # Replace with actual UDID
  platform = "IOS"
}

resource "apple_device" "dev_ipad" {
  name     = "Development iPad"
  udid     = "abcdef12-34567890123456789012345678901234" # Replace with actual UDID
  platform = "IOS"
}

resource "apple_device" "test_mac" {
  name     = "Test MacBook"
  udid     = "550e8400-e29b-41d4-a716-446655440000" # Replace with actual UDID
  platform = "MAC_OS"
}

# Query existing devices
data "apple_devices" "all_devices" {}

# Query iOS devices only
data "apple_devices" "ios_devices" {
  platform   = "IOS"
  sort_by    = "name"
  sort_order = "asc"
}

# Query enabled devices by device class
data "apple_devices" "iphones" {
  device_class = "IPHONE"
  status       = "ENABLED"
}

# ============================================================================
# PROVISIONING PROFILES
# ============================================================================

# Create a development profile for the Bundle ID
resource "apple_profile" "example_development" {
  name         = "Development Profile for ${apple_bundle_id.example_app.name}"
  platform     = "IOS"
  bundle_id    = apple_bundle_id.example_app.id
  certificates = [apple_certificate.ios_development.id]
  devices      = [apple_device.dev_iphone.id, apple_device.dev_ipad.id]
}

# Create an App Store distribution profile
resource "apple_profile" "example_app_store" {
  name         = "App Store Profile for ${apple_bundle_id.example_app.name}"
  platform     = "IOS"
  bundle_id    = apple_bundle_id.example_app.id
  certificates = [apple_certificate.ios_distribution.id]
  # Note: No devices for App Store distribution profiles
}

# Create an Ad Hoc distribution profile
resource "apple_profile" "example_adhoc" {
  name         = "Ad Hoc Profile for ${apple_bundle_id.example_app.name}"
  platform     = "IOS"
  bundle_id    = apple_bundle_id.example_app.id
  certificates = [apple_certificate.ios_distribution.id]
  devices      = [apple_device.dev_iphone.id, apple_device.dev_ipad.id]
}

# Query all profiles
data "apple_profiles" "all_profiles" {
  depends_on = [
    apple_profile.example_development,
    apple_profile.example_app_store,
    apple_profile.example_adhoc,
  ]
}

# Query active iOS profiles
data "apple_profiles" "active_ios_profiles" {
  platform      = "IOS"
  profile_state = "ACTIVE"
  sort_by       = "created_date"
  sort_order    = "desc"
}

# Query development profiles by name pattern
data "apple_profiles" "development_profiles" {
  name_pattern = ".*Development.*"
  platform     = "IOS"
}

# Output the new Bundle ID details
output "new_bundle_id" {
  value = {
    id         = apple_bundle_id.example_app.id
    identifier = apple_bundle_id.example_app.identifier
    name       = apple_bundle_id.example_app.name
  }
}

# Output Bundle ID capabilities
output "bundle_id_capabilities" {
  value = [for cap in data.apple_bundle_id_capabilities.example_app_capabilities.capabilities : {
    capability_type = cap.capability_type
    settings_count  = length(cap.settings)
  }]
}

# Output existing Bundle IDs
output "existing_ios_bundle_ids" {
  value = [for bid in data.apple_bundle_ids.ios_apps.bundle_ids : {
    identifier = bid.identifier
    name       = bid.name
    platform   = bid.platform
  }]
}

# Output certificate details (excluding sensitive content)
output "created_certificates" {
  value = {
    ios_development = {
      id               = apple_certificate.ios_development.id
      serial_number    = apple_certificate.ios_development.serial_number
      display_name     = apple_certificate.ios_development.display_name
      certificate_type = apple_certificate.ios_development.certificate_type
      expiration_date  = apple_certificate.ios_development.expiration_date
      platform         = apple_certificate.ios_development.platform
    }
    ios_distribution = {
      id               = apple_certificate.ios_distribution.id
      serial_number    = apple_certificate.ios_distribution.serial_number
      display_name     = apple_certificate.ios_distribution.display_name
      certificate_type = apple_certificate.ios_distribution.certificate_type
      expiration_date  = apple_certificate.ios_distribution.expiration_date
      platform         = apple_certificate.ios_distribution.platform
    }
  }
}

# Output development certificates from data source
output "development_certificates" {
  value = {
    total_count     = data.apple_certificates.development_certificates.total_count
    filtered_count  = data.apple_certificates.development_certificates.filtered_count
    certificates    = [for cert in data.apple_certificates.development_certificates.certificates : {
      display_name    = cert.display_name
      serial_number   = cert.serial_number
      expiration_date = cert.expiration_date
      requester_email = cert.requester_email
    }]
  }
}

# Output production certificates
output "production_certificates" {
  value = [for cert in data.apple_certificates.production_certificates.certificates : {
    display_name     = cert.display_name
    certificate_type = cert.certificate_type
    serial_number    = cert.serial_number
    expiration_date  = cert.expiration_date
  }]
}

# Output registered devices
output "registered_devices" {
  value = {
    dev_iphone = {
      id           = apple_device.dev_iphone.id
      name         = apple_device.dev_iphone.name
      udid         = apple_device.dev_iphone.udid
      platform     = apple_device.dev_iphone.platform
      device_class = apple_device.dev_iphone.device_class
      status       = apple_device.dev_iphone.status
    }
    test_mac = {
      id           = apple_device.test_mac.id
      name         = apple_device.test_mac.name
      udid         = apple_device.test_mac.udid
      platform     = apple_device.test_mac.platform
      device_class = apple_device.test_mac.device_class
      status       = apple_device.test_mac.status
    }
  }
}

# Output device statistics
output "device_statistics" {
  value = {
    total_devices     = data.apple_devices.all_devices.total_count
    ios_device_count  = data.apple_devices.ios_devices.filtered_count
    iphone_count      = data.apple_devices.iphones.filtered_count
    
    devices_by_platform = {
      for platform in ["IOS", "MAC_OS", "TV_OS", "WATCH_OS", "VISION_OS"] :
      platform => length([
        for device in data.apple_devices.all_devices.devices :
        device if device.platform == platform
      ])
    }
    
    devices_by_status = {
      for status in ["ENABLED", "PROCESSING", "INELIGIBLE"] :
      status => length([
        for device in data.apple_devices.all_devices.devices :
        device if device.status == status
      ])
    }
  }
}

# Output iOS device details
output "ios_device_details" {
  value = [
    for device in data.apple_devices.ios_devices.devices : {
      name         = device.name
      device_class = device.device_class
      model        = device.model
      status       = device.status
      added_date   = device.added_date
    }
  ]
}

# Output created profiles
output "created_profiles" {
  value = {
    development_profile = {
      id              = apple_profile.example_development.id
      name            = apple_profile.example_development.name
      uuid            = apple_profile.example_development.uuid
      profile_state   = apple_profile.example_development.profile_state
      profile_type    = apple_profile.example_development.profile_type
      created_date    = apple_profile.example_development.created_date
      expiration_date = apple_profile.example_development.expiration_date
      device_count    = length(apple_profile.example_development.devices)
      certificate_count = length(apple_profile.example_development.certificates)
    }
    
    app_store_profile = {
      id              = apple_profile.example_app_store.id
      name            = apple_profile.example_app_store.name
      uuid            = apple_profile.example_app_store.uuid
      profile_state   = apple_profile.example_app_store.profile_state
      profile_type    = apple_profile.example_app_store.profile_type
      created_date    = apple_profile.example_app_store.created_date
      expiration_date = apple_profile.example_app_store.expiration_date
    }
    
    adhoc_profile = {
      id              = apple_profile.example_adhoc.id
      name            = apple_profile.example_adhoc.name
      uuid            = apple_profile.example_adhoc.uuid
      profile_state   = apple_profile.example_adhoc.profile_state
      profile_type    = apple_profile.example_adhoc.profile_type
      created_date    = apple_profile.example_adhoc.created_date
      expiration_date = apple_profile.example_adhoc.expiration_date
      device_count    = length(apple_profile.example_adhoc.devices)
      certificate_count = length(apple_profile.example_adhoc.certificates)
    }
  }
}

# Output profile statistics
output "profile_statistics" {
  value = {
    total_profiles = data.apple_profiles.all_profiles.total_count
    active_ios_profiles = data.apple_profiles.active_ios_profiles.filtered_count
    development_profiles = data.apple_profiles.development_profiles.filtered_count
    last_updated = data.apple_profiles.all_profiles.last_updated
  }
}

# Output active iOS profiles
output "active_ios_profiles" {
  value = [for profile in data.apple_profiles.active_ios_profiles.profiles : {
    name            = profile.name
    uuid            = profile.uuid
    profile_type    = profile.profile_type
    profile_state   = profile.profile_state
    expiration_date = profile.expiration_date
  }]
}

# Output development profiles
output "development_profiles" {
  value = [for profile in data.apple_profiles.development_profiles.profiles : {
    name         = profile.name
    uuid         = profile.uuid
    profile_type = profile.profile_type
    created_date = profile.created_date
  }]
}