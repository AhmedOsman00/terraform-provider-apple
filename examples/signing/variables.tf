variable "bundle_identifier" {
  description = "The App ID to manage, e.g. com.example.myapp."
  type        = string
}

variable "app_name" {
  description = "Human-readable name for the App ID as it appears in the developer portal."
  type        = string
}

variable "organization_name" {
  description = <<-EOT
    Organization name placed in the certificate signing request subject.

    Cosmetic: Apple ignores the CSR subject and issues certificates under your
    team's own name. It only affects the local CSR.
  EOT
  type        = string
  default     = "Terraform"
}

variable "profile_name_prefix" {
  description = <<-EOT
    Prefix for generated provisioning profile names.

    Profile names must be unique across the whole team, so give each app its own
    prefix. The suffixes " Development", " Ad Hoc", and " App Store" are appended.
  EOT
  type        = string
}

variable "devices" {
  description = <<-EOT
    Devices to register and include in the development and Ad Hoc profiles, as a
    map of device name to UDID.

    Leaving this empty produces only the App Store profile, which is the usual
    shape for a CI-only release pipeline.
  EOT
  type        = map(string)
  default     = {}
}

variable "capabilities" {
  description = <<-EOT
    Capability types to enable on the App ID, limited to those that take no
    settings -- for example PUSH_NOTIFICATIONS, IN_APP_PURCHASE, SIRIKIT,
    HOMEKIT, WALLET, GAME_CENTER.

    Capabilities that require settings (ICLOUD, APP_GROUPS, APPLE_PAY,
    ASSOCIATED_DOMAINS) need nested `settings` blocks and should be declared
    directly rather than through this variable.
  EOT
  type        = set(string)
  default     = []
}

variable "early_renewal_hours" {
  description = <<-EOT
    Replace signing certificates this many hours before they expire.

    Apple certificates last a year and builds break the moment one lapses. The
    default of 720 hours (30 days) rotates them during a routine apply instead.
    The window is only evaluated when Terraform runs, so schedule a periodic
    plan/apply if you depend on it.
  EOT
  type        = number
  default     = 720
}

variable "private_keys" {
  description = <<-EOT
    Existing signing private keys in PEM format, keyed by role: "development",
    "distribution", or both. A role left out gets a freshly generated key.

    Supply the key fastlane match already holds when adopting its certificate.
    A certificate is only usable together with the key it was issued against, so
    an adopted certificate without its key is something you cannot sign with.
  EOT
  type        = map(string)
  default     = {}
  sensitive   = true

  validation {
    condition = alltrue([
      for role in keys(var.private_keys) : contains(["development", "distribution"], role)
    ])
    error_message = "private_keys may only be keyed by \"development\" or \"distribution\"."
  }
}

variable "adopt_certificate_serials" {
  description = <<-EOT
    Serial numbers of existing Apple certificates to adopt, keyed by role:
    "development", "distribution", or both.

    A role listed here is read through the `apple_certificates` data source
    rather than issued, so applying this module cannot revoke the certificate
    your team is still shipping with. This is the migration path off match:
    adopt what match issued, supply its key in `private_keys`, and cut over to a
    Terraform-issued certificate later, on your own schedule.

    An adopted certificate is read-only. `early_renewal_hours` does not apply to
    it, because renewing means issuing a replacement and issuing revokes the
    original. Drop the role from this map when you are ready for that.
  EOT
  type        = map(string)
  default     = {}

  validation {
    condition = alltrue([
      for role in keys(var.adopt_certificate_serials) : contains(["development", "distribution"], role)
    ])
    error_message = "adopt_certificate_serials may only be keyed by \"development\" or \"distribution\"."
  }
}
