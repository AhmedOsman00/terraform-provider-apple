output "signing_bundle" {
  description = <<-EOT
    Everything applesign needs to install signing on a machine.

    Contains private keys. Consume it with `terraform output -json signing_bundle`
    and pipe it straight into `applesign install -` -- never redirect it to a
    file you keep.

    To hand it to a secret store instead, reference `local.signing_bundle`
    directly; see bundle.tf.
  EOT
  sensitive   = true

  value = local.signing_bundle

  # A certificate is only usable with the key it was issued against, so a bundle
  # that adopts one without supplying its key installs an identity nothing can
  # sign with. Fail here rather than at codesign time.
  precondition {
    condition = alltrue([
      for role in keys(var.adopt_certificate_serials) : contains(local.supplied_key_roles, role)
    ])
    error_message = "Every role in var.adopt_certificate_serials needs its key in var.private_keys; an adopted certificate without its private key cannot sign."
  }
}

output "bundle_id" {
  description = "The Apple-generated ID of the App ID, for referencing from other configurations."
  value       = apple_bundle_id.app.id
}

output "profiles" {
  description = "Name, UUID, type, and expiry of each generated profile. Safe to print."
  value = [
    for profile in local.all_profiles : {
      name            = profile.name
      uuid            = profile.uuid
      profile_type    = profile.profile_type
      expiration_date = profile.expiration_date
    }
  ]
}

output "certificate_expirations" {
  description = "Expiry of each signing certificate, issued or adopted, for monitoring the renewal window. Safe to print."
  value = {
    for role, certificate in local.certificates : role => certificate.expiration_date
  }
}

output "registered_devices" {
  description = "The devices included in the development and Ad Hoc profiles."
  value = {
    for name, device in apple_device.team : name => device.udid
  }
}
