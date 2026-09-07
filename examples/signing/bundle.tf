# The signing bundle: everything a machine needs in order to sign, assembled in
# one place.
#
# It is a local rather than only an output because outputs cannot be referenced
# by resources in the same configuration. Keeping it here lets you add your own
# file next to this one, wire the bundle into whatever secret store your team
# uses, and never edit the module itself:
#
#   resource "aws_secretsmanager_secret_version" "signing" {
#     secret_id     = aws_secretsmanager_secret.signing.id
#     secret_string = jsonencode(local.signing_bundle)
#   }
#
# Whatever stores it, the JSON that comes back out is what `applesign install -`
# reads. See README.md.

locals {
  # count-based profiles come back as lists; the App Store profile is always
  # present. Flattening them here keeps the bundle schema stable whether or not
  # devices are configured.
  all_profiles = concat(
    apple_profile.development,
    apple_profile.ad_hoc,
    [apple_profile.app_store],
  )

  signing_bundle = {
    bundle_identifier = apple_bundle_id.app.identifier

    # Issued and adopted certificates alike; local.private_keys holds the
    # generated key or the one the team supplied, per role.
    certificates = [
      for role, certificate in local.certificates : {
        key                 = role
        certificate_type    = certificate.certificate_type
        certificate_content = certificate.certificate_content
        private_key_pem     = local.private_keys[role]
        expiration_date     = certificate.expiration_date
      }
    ]

    profiles = [
      for profile in local.all_profiles : {
        name            = profile.name
        uuid            = profile.uuid
        profile_type    = profile.profile_type
        profile_content = profile.profile_content
        expiration_date = profile.expiration_date
      }
    ]
  }
}
