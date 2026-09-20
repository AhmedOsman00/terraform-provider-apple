# App Store listing

A runnable module that manages an app's App Store listing: content rights,
categories, age rating, localized metadata, the release, App Review
information, the price and the storefronts it sells in.

It is the Terraform equivalent of the flat metadata file EAS and fastlane use —
an `app.json` `store.config`, or a `Deliverfile` — with two differences that are
Apple's rather than this module's, and both worth understanding before you
start.

## What Apple splits that a metadata file does not

**Localized metadata has two lifetimes.** The app's name, subtitle and privacy
policy link describe the app and survive every release; the description,
keywords, promotional text and release notes describe one release and are
replaced with it. Apple keeps them in two different records —
`appInfoLocalizations` and `appStoreVersionLocalizations` — with two different
lifecycles. A flat file hides this. This module keeps a single `locales` map and
fans it out to both resources, which is as close to the flat shape as a
configuration can honestly get.

**Most of these records already exist.** Apple creates the app info, the age
rating declaration and (once an app has shipped) the price schedule and
availability alongside the app itself, and publishes no `POST` for them. The
resources covering those adopt what is there rather than creating it, and
`terraform destroy` drops them from state with a warning instead of deleting
something Apple would refuse to delete.

## What this module cannot do

`terraform output remaining_manual_steps` lists these. The first is the one that
will stop a submission:

- **App Privacy.** App Store Connect blocks a submission with "Admin must
  provide information about the app's privacy practices in the App Privacy
  section", and Apple publishes **no API for it at all** — there is no
  `appDataUsages` resource in the App Store Connect API. It has to be answered
  once on the website. It is declared per app rather than per version, so it
  does not recur with each release.
- **Screenshots and app previews.** Media assets are not managed by this
  provider.
- **The build upload.** Push the binary with Xcode, Transporter or fastlane;
  Apple publishes no upload endpoint. *Attaching* it is not manual — set
  `build_number` to the `CURRENT_PROJECT_VERSION` the pipeline just built and
  this module links it to the version. Apple leaves a build `PROCESSING` for
  five to thirty minutes after the upload finishes and refuses to attach one
  until it is `VALID`, so a pipeline that uploads and applies in one run should
  wait in between rather than expect the apply to.
- **Export compliance.** Answer it with `ITSAppUsesNonExemptEncryption` in
  `Info.plist` at build time. It lives on the build rather than the version, and
  a build without it parks the version in `WAITING_FOR_EXPORT_COMPLIANCE`
  however complete the metadata is.
- **Submitting for review.** App Store Connect only. Apple does publish a
  submission API, but submission is an event rather than a state: a resource for
  it would cancel a live review on destroy and resubmit on apply.

## Usage

The app record must already exist — Apple's API cannot create one. Create it on
the App Store Connect website, then:

```console
$ cp terraform.tfvars.example terraform.tfvars
$ $EDITOR terraform.tfvars

$ export APPLE_APP_STORE_CONNECT_ISSUER_ID=...
$ export APPLE_APP_STORE_CONNECT_API_KEY=...
$ export APPLE_APP_STORE_CONNECT_PRIVATE_KEY="$(cat AuthKey_XXXXXXXXXX.p8)"

$ terraform init
$ terraform apply
```

### If the app already has a version being prepared

An app holds only one editable version per platform at a time, and Apple answers
a second with a 409 naming the one that already exists. Import it rather than
letting Terraform try to create another:

```console
$ terraform import apple_app_store_version.this <app_id>/IOS/1.0
```

The same applies to any localization that already exists:

```console
$ terraform import 'apple_app_info_localization.this["en-US"]' <app_id>/en-US
$ terraform import 'apple_app_store_version_localization.this["en-US"]' <version_id>/en-US
```

Everything else in this module adopts rather than creates, so it needs no
import — `apply` will find the existing record and patch it.

## Things that are easy to get wrong

**Keywords are one 100-character string, not a list.** Apple has never modelled
them as a list; the provider joins yours with commas and no spaces, because a
space is a character and every one of them comes out of the hundred. The limit
is checked at plan time against the joined length, so ten individually legal
keywords can still be rejected together.

**Character limits are counted in characters, not bytes.** A 30-character Arabic
name is 30 characters, not 54 bytes' worth. This matters throughout: `title`
and `subtitle` get 30, `promo_text` 170, `description` and `whats_new` 4000.

**An unset age rating answer is not `NONE`.** Apple leaves an omitted attribute
exactly as it was, which for a new app means unanswered — and an unanswered
questionnaire blocks the submission. The defaults in `variables.tf` answer
`NONE` and `false` to everything, which is right for an app with no mature
content and wrong to leave unread. Note also that Apple revised the
questionnaire: `age_assurance`, `loot_box`, `messaging_and_chat` and
`user_generated_content` exist only on the newer form, and
`age_rating_override_v2` replaced `age_rating_override` (in which
`SEVENTEEN_PLUS` became `EIGHTEEN_PLUS`).

**Release notes are rejected on a first version and required after it.** Leave
`whats_new` unset for 1.0; Apple has nothing to compare it against.

**A free app is a price point of `0`,** not the absence of a price schedule.
Apple retired price tiers entirely — the price is a base territory whose price
is equalized into every other storefront.

**Writes only land while a version is being prepared.** Once a version enters
review, Apple freezes the app info, its localizations and the age rating, and
publishes no way to make a fresh one. A plan that suddenly cannot write is
usually explained by the `app_version_state` output.

**`available_in_new_territories` covers only storefronts Apple opens later.** It
is no substitute for naming the ones that exist today, which is why leaving
`territories` empty reads the full list from the `apple_territories` data source
rather than relying on the flag.

## Territories and the demo account

`review.demo_account_password` is **stored in Terraform state in plain text**.
Supply it from a secret store rather than writing it into `terraform.tfvars`,
and use an account created for App Review alone. The variable is marked
`sensitive`, which keeps it out of console output but not out of state.
