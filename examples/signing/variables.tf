variable "bundle_identifier" {
  description = "The App ID to manage, e.g. com.example.myapp."
  type        = string
}

variable "app_name" {
  description = "Human-readable name for the App ID as it appears in the developer portal."
  type        = string
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

    Replacement reuses the CSR in `var.csr_contents`, so the renewed certificate
    covers the same key and the identity already installed on a machine keeps
    working. It does not apply to adopted certificates: renewing one means
    issuing a replacement, and issuing revokes the original.
  EOT
  type        = number
  default     = 720
}

variable "csr_contents" {
  description = <<-EOT
    Certificate signing requests in PEM format, keyed by role: "development",
    "distribution", or both.

    Generate these yourself; the matching private key never reaches Terraform:

      openssl genrsa -out distribution.key 2048
      openssl req -new -key distribution.key -out distribution.csr \
          -subj "/CN=My App distribution"

    A CSR carries a public key and a signature over it, so nothing here is
    secret and neither is anything Terraform stores or outputs as a result.
    Keep the private key -- it is the half that signs, and the certificate Apple
    returns is inert without it.

    A role listed in `adopt_certificate_serials` is read rather than issued, so
    it needs no CSR here.
  EOT
  type        = map(string)
  default     = {}

  validation {
    condition = alltrue([
      for role in keys(var.csr_contents) : contains(["development", "distribution"], role)
    ])
    error_message = "csr_contents may only be keyed by \"development\" or \"distribution\"."
  }

  validation {
    condition = alltrue([
      for csr in values(var.csr_contents) :
      can(regex("-----BEGIN CERTIFICATE REQUEST-----[\\s\\S]*-----END CERTIFICATE REQUEST-----", csr))
    ])
    error_message = "Every value in csr_contents must be a PEM-encoded certificate signing request. A key or a certificate is not a CSR."
  }

  validation {
    condition = (
      contains(keys(var.csr_contents), "distribution") ||
      contains(keys(var.adopt_certificate_serials), "distribution")
    )
    error_message = "The distribution role is required: every profile this module creates signs with it. Give it a CSR in csr_contents, or a serial number in adopt_certificate_serials."
  }
}

variable "adopt_certificate_serials" {
  description = <<-EOT
    Serial numbers of existing Apple certificates to adopt, keyed by role:
    "development", "distribution", or both.

    A role listed here is read through the `apple_certificates` data source
    rather than issued, so applying this module cannot revoke the certificate
    your team is still shipping with. This is the migration path off match:
    adopt what match issued, keep using the key match holds, and cut over to a
    Terraform-issued certificate later, on your own schedule.

    Adoption takes precedence: a role given both a serial here and a CSR in
    `csr_contents` is adopted, and the CSR is unused until you remove the serial.

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
