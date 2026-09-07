## 0.1.0 (Unreleased)

FEATURES:

* examples/signing: a runnable module replacing `fastlane match`, covering the App
  ID, capabilities, devices, signing certificates, and development/Ad Hoc/App Store
  profiles, plus `scripts/install-signing.sh` for the local keychain and
  provisioning profile installation Terraform does not do.

BREAKING CHANGES:

* resource/apple_profile: `profile_type` is now a required argument, and `platform` is
  computed rather than configurable. Apple's `POST /v1/profiles` takes `profileType`
  (which encodes the platform) and does not accept `platform`, so the profile type is
  what selects development, ad hoc, App Store, or in-house distribution. Existing
  configurations must replace `platform = "IOS"` with the corresponding
  `profile_type`, e.g. `profile_type = "IOS_APP_STORE"`.

BUG FIXES:

* resource/apple_profile: profile creation sent `platform` instead of the required
  `profileType` attribute, so no profile could be created and the distribution method
  could not be expressed at all.
* resource/apple_profile: the `certificates` and `devices` relationships were
  serialized as arrays of to-one identifiers rather than a single object with a `data`
  array, which Apple rejects. An empty `devices` relationship is now omitted instead of
  sent, as App Store and Developer ID profiles have no devices.
* resource/apple_profile: `profile_content` is now marked sensitive, matching
  `certificate_content` on `apple_certificate`. Outputs deriving values from it must be
  declared `sensitive = true`.
