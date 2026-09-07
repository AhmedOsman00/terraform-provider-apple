## 0.1.0 (Unreleased)

NOTES:

* The provider is published on the Terraform Registry as `ahmedosman00/apple`;
  `terraform init` installs it, and the Go module path is
  `github.com/AhmedOsman00/terraform-provider-apple`. Earlier working copies
  served the development address `aostudio.com/aostudio/apple` and had to be
  built locally.

FEATURES:

* examples/signing: a runnable module replacing `fastlane match`, covering the App
  ID, capabilities, devices, signing certificates, and development/Ad Hoc/App Store
  profiles. It emits a signing bundle holding everything a machine needs to sign.
* cmd/applesign: a CLI that installs a signing bundle on macOS — private key and
  certificate into a keychain (the login keychain, or a throwaway one under
  `--ci`), and provisioning profiles into the directories Xcode reads. It reads
  the bundle as JSON on stdin, so `terraform output -json signing_bundle`,
  `sops -d`, and `op read` all compose as pipes and a consumer needs no state
  access.
* examples/signing: `var.private_keys` and `var.adopt_certificate_serials` adopt a
  certificate `fastlane match` already issued, reading it through the
  `apple_certificates` data source rather than reissuing — which would revoke the
  original and break every build already signed with it.

DOCUMENTATION:

* Per-resource and per-data-source reference pages are now generated for all seven
  resources and seven data sources; previously only `docs/index.md` was checked in,
  so `make generate` produced a diff and the `generate` CI job failed.
* New guides: `docs/guides/getting-started.md` (creating App Store Connect
  credentials, installing the provider, a first configuration, and importing
  existing portal resources) and
  `docs/guides/code-signing.md` (the `fastlane match` replacement, the distinction
  between the Terraform state and the signing bundle, and migrating off match).
  Hand-written guides live in `templates/guides/` and render into `docs/guides/`.
* Examples no longer carry a `# Copyright (c) HashiCorp, Inc.` header, which was
  incorrect attribution and was being embedded into the generated documentation.

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
