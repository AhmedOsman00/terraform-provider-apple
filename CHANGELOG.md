## 0.1.0 (September 8, 2026)

Initial release.

NOTES:

* The provider is published on the Terraform Registry as `ahmedosman00/apple`;
  `terraform init` installs it, and the Go module path is
  `github.com/AhmedOsman00/terraform-provider-apple`. Earlier working copies
  served the development address `aostudio.com/aostudio/apple` and had to be
  built locally.

FEATURES:

* **Developer Portal:** `apple_bundle_id`, `apple_bundle_id_capability`,
  `apple_certificate`, `apple_device`, `apple_merchant_id`,
  `apple_pass_type_id` and `apple_profile`, each with a plural data source that
  lists the collection with filtering, `sort_by`, `sort_order` and `limit`.
  Together these cover what a build pipeline needs from the portal: App IDs and
  their capabilities, signing certificates, registered devices, Apple Pay
  Merchant IDs, Apple Wallet Pass Type IDs, and provisioning profiles. Two
  destroy behaviours are worth knowing before the first apply: destroying an
  `apple_certificate` revokes it at Apple, and every build already signed with
  it stops verifying; and Apple's API cannot delete a device, so removing an
  `apple_device` disables it and drops it from state with a warning.
* `apple_profile.profile_type` is the required argument that selects the
  distribution method — development, ad hoc, App Store, or in-house — and
  `platform` is computed from it rather than configured. Apple's
  `POST /v1/profiles` takes `profileType`, which encodes the platform, and does
  not accept `platform` at all. `profile_content` is marked sensitive, matching
  `certificate_content` on `apple_certificate`, so outputs deriving values from
  it must be declared `sensitive = true`.
* **App Store Connect — auto-renewable subscriptions:**
  `apple_subscription_group`, `apple_subscription`,
  `apple_subscription_localization` and `apple_subscription_price`, with
  `apple_subscription_groups`, `apple_subscriptions` and
  `apple_subscription_price_points` data sources. These behave differently from
  Developer Portal resources: they hang off an app record, and Apple publishes
  no top-level collection for any of them, so every listing takes a required
  scope argument and several import forms are composite.
* **App Store Connect — one-time in-app purchases:** `apple_in_app_purchase`,
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
* `apple_apps` data source. There is deliberately no `apple_app` resource —
  Apple's documentation says to create new apps on the App Store Connect
  website and publishes no endpoint to create or delete one — but a
  subscription group needs the app's ID, so it has to be readable.
* examples/signing: a runnable module replacing `fastlane match`, covering the App
  ID, capabilities, devices, signing certificates, and development/Ad Hoc/App Store
  profiles. The signing key is generated on your machine and never reaches
  Terraform: the module takes certificate signing requests through
  `var.csr_contents` and exports the issued certificates and generated profiles
  as plain values. A CSR, a certificate and a provisioning profile are all public
  documents, so nothing in the state or the outputs is secret, and installing the
  result is a `.p12` built locally from the certificate and the key you kept.
* examples/signing: `var.adopt_certificate_serials` adopts a certificate
  `fastlane match` already issued, reading it through the `apple_certificates`
  data source rather than reissuing — which would revoke the original and break
  every build already signed with it. An adopted role needs no CSR; keep using
  the key match holds for it.

DOCUMENTATION:

* Generated reference pages for all fifteen resources and thirteen data
  sources, built by `tfplugindocs` from the schemas and the matching
  directories under `examples/`.
* Two hand-written guides: `docs/guides/getting-started.md` (creating App Store
  Connect credentials, installing the provider, a first configuration, and
  importing existing portal resources) and `docs/guides/code-signing.md` (the
  `fastlane match` replacement, generating a CSR and installing the issued
  certificate by hand, what actually needs protecting once the key never enters
  Terraform, and migrating off match). Guides live in `templates/guides/` and
  render into `docs/guides/`.
