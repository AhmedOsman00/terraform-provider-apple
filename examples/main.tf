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