# Everything this module produces, as plain values.
#
# None of it is secret. The private keys stay on the machine that generated the
# CSRs, so what Terraform holds is a public certificate, a public provisioning
# profile, and some identifiers -- safe to print, to log, and to commit. See
# README.md for turning these into an installed signing identity.

output "bundle_id" {
  description = "The Apple-generated ID of the App ID, for referencing from other configurations."
  value       = apple_bundle_id.app.id
}

output "certificates" {
  description = <<-EOT
    The signing certificates, issued and adopted alike, keyed by role.

    `certificate_content` is the base64-encoded DER Apple issued. Pair it with
    the private key you generated the CSR from to get something that can sign:

      terraform output -json certificates \
        | jq -r '.distribution.certificate_content' | base64 -d > distribution.cer
  EOT
  value       = local.certificates
}

output "profiles" {
  description = <<-EOT
    The generated provisioning profiles.

    `profile_content` is the base64-encoded `.mobileprovision`. Xcode indexes
    profiles by the UUID in the filename, so install one as `<uuid>.mobileprovision`.
  EOT
  value = [
    for profile in local.all_profiles : {
      name            = profile.name
      uuid            = profile.uuid
      profile_type    = profile.profile_type
      profile_content = nonsensitive(profile.profile_content)
      expiration_date = profile.expiration_date
    }
  ]
}

output "registered_devices" {
  description = "The devices included in the development and Ad Hoc profiles."
  value = {
    for name, device in apple_device.team : name => device.udid
  }
}
