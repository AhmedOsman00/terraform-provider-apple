## 0.2.1 (September 17, 2026)

BUG FIXES:

* **`apple_app_info` no longer fails an apply with "Provider produced
  inconsistent result after apply".** Apple's `PATCH /v1/appInfos/{id}` reports
  the six category relationships as links alone — it fills in a linkage only for
  an `include`, which the modify endpoint does not accept — so reading the
  categories off that response wrote null into state for every category the
  apply had just set. The resource now applies the attributes from the `PATCH`
  response and re-reads the app info for the categories. If that read fails the
  apply keeps the configured categories and warns, rather than losing the values
  it wrote.

## 0.2.0 (September 16, 2026)

FEATURES:

* **App Store listing metadata.** Nine resources and three data sources that
  manage what the App Store shows about an app: `apple_app_settings` (content
  rights, primary language, subscription notification URLs), `apple_app_info`
  (categories), `apple_app_info_localization` (name, subtitle, privacy policy
  link), `apple_app_age_rating_declaration` (the content questionnaire),
  `apple_app_store_version` (version string, copyright, release type),
  `apple_app_store_version_localization` (description, keywords, promotional
  text, release notes), `apple_app_store_review_detail` (what App Review is
  told), `apple_app_price_schedule` (what the app costs) and
  `apple_app_availability` (the storefronts it sells in), alongside
  `apple_app_categories`, `apple_app_price_points` and
  `apple_app_store_versions`.

  Together these cover three of the four items App Store Connect blocks a
  submission on. Content Rights Information is `content_rights_declaration` on
  `apple_app_settings`; the price tier is `apple_app_price_schedule`, Apple
  having retired tiers in favour of price points. **App Privacy is the fourth,
  and it has no API** — Apple's published App Store Connect API contains no
  `appDataUsages` resource, so the data-collection questionnaire has to be
  answered once on the website. It is declared per app rather than per version,
  so it does not recur with each release.

* `examples/app-listing` is a runnable module that manages a whole listing from
  one `terraform.tfvars`, the way an EAS `store.config.json` or a fastlane
  `Deliverfile` does. Its `remaining_manual_steps` output names what no
  configuration can do: App Privacy, screenshots, the build upload, and
  submitting for review.

NOTES:

* **Localized metadata is split across two resources, because Apple splits it
  across two records.** The app's name, subtitle and privacy policy link live on
  an `AppInfo` and survive every release; the description, keywords,
  promotional text and release notes live on an `AppStoreVersion` and are
  replaced with it. A flat metadata file hides that; Terraform cannot, since the
  two have different lifecycles. `examples/app-listing` keeps one map and fans
  it out to both.

* **`apple_app_settings`, `apple_app_info` and
  `apple_app_age_rating_declaration` adopt records Apple already created.** Apple
  makes them alongside the app and publishes no `POST` for any of them, so these
  resources patch on create and `terraform destroy` drops them from state with a
  warning rather than deleting anything. The same is true of
  `apple_app_price_schedule` and `apple_app_availability`, which Apple replaces
  wholesale with a `POST` and never deletes.

* **Listing metadata can only be written while a version is being prepared.**
  Apple freezes an app info once it is in review or distributed and publishes no
  way to create a fresh one, so the categories, the localized name and the age
  rating cannot change until the current review finishes. The provider resolves
  the editable record itself and says so plainly when there is none.

* **The age rating questionnaire has changed.** The provider models Apple's
  current form, which adds `age_assurance`, `loot_box`, `messaging_and_chat`,
  `parental_controls`, `social_media`, `social_media_age_restricted`,
  `user_generated_content`, `advertising`, `health_or_wellness_topics` and
  `guns_or_other_weapons`, and replaces `age_rating_override` with
  `age_rating_override_v2` (in which `SEVENTEEN_PLUS` became `EIGHTEEN_PLUS`).
  Both override attributes and both frequency vocabularies are accepted, because
  an existing declaration may hold either. An omitted answer is **not** `NONE`:
  Apple leaves it as it was, which for a new app means unanswered.

* **`keywords` is a list in Terraform and one 100-character comma-separated
  string at Apple.** The provider joins with no space after the comma, because a
  space is a character and every one comes out of the hundred. The cap applies
  to the joined string, so it is checked at plan time rather than per keyword.

* Two acceptance tests — `TestAccAppPriceScheduleResource_basic` and
  `TestAccAppAvailabilityResource_basic` — skip unless
  `APPLE_TEST_ALLOW_APP_PRICING` is set, because they change what an app costs
  and where it sells and Apple publishes no `DELETE` for either record. The rest
  of the app listing tests edit the `APPLE_TEST_APP_ID` app's own metadata in
  place; point it at a scratch app. `ACCEPTANCE_TESTING.md` has the detail.

## 0.1.0 (September 9, 2026)

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
  `apple_subscription_localization`, `apple_subscription_price` and
  `apple_subscription_availability`, with `apple_subscription_groups`,
  `apple_subscriptions` and `apple_subscription_price_points` data sources.
  These behave differently from Developer Portal resources: they hang off an
  app record, and Apple publishes no top-level collection for any of them, so
  every listing takes a required scope argument and several import forms are
  composite. Two ordering rules are worth knowing before the first apply. A
  subscription needs an `apple_subscription_availability` before it can be
  priced — Apple rejects a price without one and says only that "an error
  occurred while processing the pricing information", so declare `depends_on`
  from the price to the availability, since nothing else connects them. And
  like the in-app purchase availability, the record is a singular one Apple
  replaces with a `POST` and publishes no `DELETE` for: it updates in place and
  cannot be destroyed. Destroying an `apple_subscription_price` that is already
  in effect likewise warns and drops state rather than deleting: Apple removes
  only price changes scheduled for the future, and a live price is superseded by
  a later one rather than withdrawn. Deleting the subscription removes both.
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
  destroyed. Destroying the *only* localization of a purchase likewise warns and
  drops state rather than deleting: Apple requires every version to keep one.
  Deleting the `apple_in_app_purchase` removes it for real.
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

* Generated reference pages for all sixteen resources and thirteen data
  sources, built by `tfplugindocs` from the schemas and the matching
  directories under `examples/`.
* Two hand-written guides: `docs/guides/getting-started.md` (creating App Store
  Connect credentials, installing the provider, a first configuration, and
  importing existing portal resources) and `docs/guides/code-signing.md` (the
  `fastlane match` replacement, generating a CSR and installing the issued
  certificate by hand, what actually needs protecting once the key never enters
  Terraform, and migrating off match). Guides live in `templates/guides/` and
  render into `docs/guides/`.
