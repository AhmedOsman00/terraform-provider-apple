## 0.1.0 (Unreleased)

NOTES:

* The provider is published on the Terraform Registry as `ahmedosman00/apple`;
  `terraform init` installs it, and the Go module path is
  `github.com/AhmedOsman00/terraform-provider-apple`. Earlier working copies
  served the development address `aostudio.com/aostudio/apple` and had to be
  built locally.

FEATURES:

* **New:** one-time in-app purchase support, covering `apple_in_app_purchase`,
  `apple_in_app_purchase_localization`, `apple_in_app_purchase_price_schedule`
  and `apple_in_app_purchase_availability`, with `apple_in_app_purchases` and
  `apple_in_app_purchase_price_points` data sources. These are the consumables,
  non-consumables and non-renewing subscriptions of the App Store — a different
  Apple resource from auto-renewable subscriptions, sharing nothing with them,
  price points included. Three shapes are worth knowing before you plan:
  `app_id` is write-once and unreadable, because Apple's in-app purchase
  resource has no app relationship at all, so import takes
  `<app_id>/<in_app_purchase_id>`; localizations sit behind an in-app purchase
  version, which the provider resolves and reports as `version_id`; and the
  price schedule and availability are singular records Apple replaces with a
  `POST` and publishes no `DELETE` for, so they update in place and cannot be
  destroyed.
* **New:** auto-renewable subscription support, covering `apple_subscription_group`,
  `apple_subscription`, `apple_subscription_localization` and
  `apple_subscription_price`, with `apple_subscription_groups`,
  `apple_subscriptions` and `apple_subscription_price_points` data sources.
  These are App Store Connect resources rather than Developer Portal ones: they
  hang off an app record, and Apple publishes no top-level collection for any of
  them, so every listing takes a required scope argument and several import
  forms are composite.
* **New:** `apple_apps` data source. There is deliberately no `apple_app`
  resource — Apple's documentation says to create new apps on the App Store
  Connect website and publishes no endpoint to create or delete one — but a
  subscription group needs the app's ID, so it has to be readable.
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
