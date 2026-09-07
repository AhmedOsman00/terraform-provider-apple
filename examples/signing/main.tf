terraform {
  required_version = ">= 1.5"

  required_providers {
    apple = {
      source = "aostudio.com/aostudio/apple"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}

provider "apple" {
  # Credentials come from the environment:
  #   APPLE_APP_STORE_CONNECT_ISSUER_ID
  #   APPLE_APP_STORE_CONNECT_API_KEY
  #   APPLE_APP_STORE_CONNECT_PRIVATE_KEY
}

locals {
  # The two identities a normal iOS project needs. Development signs debug
  # builds onto registered devices; distribution signs everything that leaves
  # the machine, both Ad Hoc and App Store.
  certificate_types = {
    development  = "IOS_DEVELOPMENT"
    distribution = "IOS_DISTRIBUTION"
  }

  # Development and Ad Hoc profiles are only meaningful once at least one
  # device is registered, so they are skipped on a devices-free configuration.
  has_devices = length(var.devices) > 0

  # A role is adopted when an existing serial number is given for it, and issued
  # otherwise. Adopted certificates are read, never created, so an apply cannot
  # revoke the certificate a team is still shipping with.
  issued_certificate_types = {
    for role, type in local.certificate_types :
    role => type if !contains(keys(var.adopt_certificate_serials), role)
  }

  # Keys are generated only for the roles whose key was not supplied.
  #
  # nonsensitive() is needed because for_each rejects anything derived from a
  # sensitive value, and var.private_keys is sensitive. Only the role names are
  # unwrapped here -- "development" and "distribution" are not secrets, and the
  # keys themselves stay sensitive throughout.
  supplied_key_roles = nonsensitive(keys(var.private_keys))

  generated_key_roles = {
    for role, type in local.certificate_types :
    role => type if !contains(local.supplied_key_roles, role)
  }

  private_keys = {
    for role, type in local.certificate_types :
    role => try(var.private_keys[role], tls_private_key.signing[role].private_key_pem)
  }

  # Issued and adopted certificates in one shape, so that profiles and the
  # signing bundle do not have to care which is which.
  certificates = merge(
    {
      for role, certificate in apple_certificate.signing : role => {
        id                  = certificate.id
        certificate_type    = certificate.certificate_type
        certificate_content = certificate.certificate_content
        expiration_date     = certificate.expiration_date
      }
    },
    {
      for role, adopted in data.apple_certificates.adopted : role => {
        id                  = one(adopted.certificates).id
        certificate_type    = one(adopted.certificates).certificate_type
        certificate_content = one(adopted.certificates).certificate_content
        expiration_date     = one(adopted.certificates).expiration_date
      }
    },
  )
}

# --- App ID and entitlements ------------------------------------------------

resource "apple_bundle_id" "app" {
  identifier = var.bundle_identifier
  name       = var.app_name
  platform   = "IOS"
}

# Capabilities that need no further configuration. Anything with settings
# (ICLOUD, APP_GROUPS, APPLE_PAY, ASSOCIATED_DOMAINS) takes nested `settings`
# blocks and belongs in your own configuration -- see
# examples/resources/apple_bundle_id_capability.
resource "apple_bundle_id_capability" "app" {
  for_each = var.capabilities

  bundle_id       = apple_bundle_id.app.id
  capability_type = each.value
}

# --- Devices ----------------------------------------------------------------

# Adding a device here regenerates every profile that includes devices, because
# `devices` forces replacement on apple_profile. That is the behaviour fastlane
# match needs `--force_for_new_devices` for; here it is just what the next plan
# does.
resource "apple_device" "team" {
  for_each = var.devices

  name     = each.key
  udid     = each.value
  platform = "IOS"
}

# --- Signing identities -----------------------------------------------------

# The private key is generated locally and never sent to Apple: apple_certificate
# submits only the CSR. This is the material fastlane match keeps in an encrypted
# git repo. Here it lives in Terraform state, so the state backend is what
# protects it -- see README.md before running this against a real team.
#
# A key supplied through var.private_keys is used as-is instead, which is how a
# team adopts the identity match already holds.
resource "tls_private_key" "signing" {
  for_each = local.generated_key_roles

  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "signing" {
  for_each = local.issued_certificate_types

  private_key_pem = local.private_keys[each.key]

  subject {
    common_name  = "${var.app_name} ${each.key}"
    organization = var.organization_name
  }
}

resource "apple_certificate" "signing" {
  for_each = local.issued_certificate_types

  certificate_type    = each.value
  csr_content         = tls_cert_request.signing[each.key].cert_request_pem
  early_renewal_hours = var.early_renewal_hours
}

# Certificates the team already has. Reading one rather than importing it is
# deliberate: `csr_content` forces replacement on apple_certificate, and Apple
# does not reliably return the CSR it was issued from, so an imported
# certificate is reissued on the next apply -- and issuing a replacement revokes
# the original, breaking every build already signed with it.
data "apple_certificates" "adopted" {
  for_each = var.adopt_certificate_serials

  serial_number = each.value

  lifecycle {
    postcondition {
      condition     = length(self.certificates) == 1
      error_message = "No certificate in this Apple team has the serial number given in var.adopt_certificate_serials."
    }
  }
}

# --- Provisioning profiles --------------------------------------------------

# Profiles snapshot the App ID's entitlements at the moment Apple generates
# them, so they must be created after the capabilities they are meant to carry.
resource "apple_profile" "development" {
  count = local.has_devices ? 1 : 0

  name         = "${var.profile_name_prefix} Development"
  profile_type = "IOS_APP_DEVELOPMENT"
  bundle_id    = apple_bundle_id.app.id
  certificates = [local.certificates["development"].id]
  devices      = [for device in apple_device.team : device.id]

  depends_on = [apple_bundle_id_capability.app]
}

resource "apple_profile" "ad_hoc" {
  count = local.has_devices ? 1 : 0

  name         = "${var.profile_name_prefix} Ad Hoc"
  profile_type = "IOS_APP_ADHOC"
  bundle_id    = apple_bundle_id.app.id
  certificates = [local.certificates["distribution"].id]
  devices      = [for device in apple_device.team : device.id]

  depends_on = [apple_bundle_id_capability.app]
}

# App Store profiles carry no devices at all.
resource "apple_profile" "app_store" {
  name         = "${var.profile_name_prefix} App Store"
  profile_type = "IOS_APP_STORE"
  bundle_id    = apple_bundle_id.app.id
  certificates = [local.certificates["distribution"].id]

  depends_on = [apple_bundle_id_capability.app]
}
