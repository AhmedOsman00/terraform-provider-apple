# Road to 1.0

What stands between today's provider (0.8.1) and CLI (0.3.2) and a 1.0 of each.
Ordered by priority: correctness first, then proof, then the promise, then reach.

A 1.0 means: every resource and CLI command has run against the live API,
state written by any 1.x plans cleanly on any later 1.x, and the schema, CLI
flags, exit codes and `--json` output change only through a deprecation cycle.

## Already verified

- **`apple build upload`**: an `ASSET`-only reservation is enough. fenn-money's
  release pipeline (CLI 0.3.0, 2026-10-22) uploaded to `COMPLETE`, processed to
  `VALID` and attached the build to an App Store version.
- **The 0.7.0 surface used by fenn-money**: about 35 of the 41 resources (signing,
  the full App Store listing, screenshots, all pricing and availability,
  subscriptions and in-app purchases with review screenshots, TestFlight groups,
  testers and pages, users and invitations), plus `version`, `release preflight`,
  `signing install/clean`, `build verify/upload/wait` and `testflight notes`.

## 1. Fix: changing the price of an approved subscription (0.8.1)

`apple_subscription_price_schedule` always sends the replace-all
`PATCH /v1/subscriptions/{id}`. Apple reads undated prices there as an
*initial* price and accepts that only in `MISSING_METADATA`, `READY_TO_SUBMIT`
and `DEVELOPER_ACTION_NEEDED`. For an approved subscription it answers 409
"Initial price cannot be created again after subscription is approved", so the
resource can set a first price but never change it.

- [x] `write` reads the subscription's state first. In the three states above it
      keeps the `PATCH`. In any other state it sends one
      `POST /v1/subscriptionPrices` for each territory whose price point differs
      from the one in effect, carrying `start_date` and `preserve_current_price`.
      A territory absent from the configuration is left alone, since a live price
      can't be withdrawn. The decision is a pure function
      (`planPriceChanges`), tested without HTTP.
- [x] A price change is always dated. A `POST` with no `startDate` is an initial
      price and gets the same 409, so an unset `start_date` sends three days from
      today (UTC). That's past the one-to-two-day minimum Apple gives for every
      time zone.
- [x] On read, Apple lists ended and preserved records beside the current price.
      Drop `preserved` records and keep the latest-starting record per territory,
      so a change scheduled by hand in App Store Connect reads back equal to a
      configuration asking for it.
- [x] An unset `plan_type` is `UPFRONT`, which is what Apple reports for the
      ordinary price. An adopted price keeps the form of the one it replaced
      (unset stays unset), or else the read plans an update forever. Fix the
      `plan_type` descriptions on both price resources: they call `MONTHLY` the
      ordinary price, but `MONTHLY` is the monthly-payment plan on a longer
      commitment.
- [x] `preserve_current_price` is an instruction, never read from Apple. An
      adopted price keeps the value of the state price it replaced, or null.
- [x] Error text names the subscription's state instead of always saying
      "in review".
- [x] `httptest` tests for each case above.
- [ ] One live run that raises the price of an approved subscription.

## 2. Adopting an existing account

Three changes that make `apple terraform export` and certificate adoption
usable on a real account. The certificate fix lands first, so the export note
can point to it.

### 2a. Fix: adopt a certificate by giving its CSR (`bugfix/`)

An imported `apple_certificate` records `csr_content` as null, since Apple
returns it null. Setting it in configuration then plans null → value, and
because the attribute is `RequiresReplace`, the next apply reissues the
certificate and revokes the original.

- [ ] Make `csr_content` `RequiresReplaceIf`: skip the replacement when prior
      state holds no CSR and the CSR's public key equals the key in
      `certificate_content` (`x509.ParseCertificateRequest` against
      `x509.ParseCertificate`, comparing the public key info). The change is
      then recorded in place, and `Update` already carries the issued
      certificate forward.
- [ ] A CSR generated later from the same private key matches too, since only
      the public key is compared.
- [ ] A key mismatch still replaces, but `ModifyPlan` warns that the CSR comes
      from a different key, so it isn't mistaken for the old bug.
- [ ] After adoption, `early_renewal_hours` renews as usual. The successor is
      issued from the stored CSR, so it reuses the same key; say so in the
      code-signing guide.
- [ ] Unit tests for the key comparison; an acceptance step that imports a
      certificate, sets its CSR and expects an empty plan.

### 2b. Clearer certificate note in the export

- [ ] Reword the `EXPORT_NOTES.md` note on each certificate: it is read through
      a data source, so Terraform won't renew it, and `early_renewal_hours`
      acts only on an `apple_certificate` Terraform owns. To hand it over,
      import it as `apple_certificate` with `csr_content` from the same private
      key (2a), then point profiles at the resource's `id`.

### 2c. Per-app scoping for `--app` (`feature/`, CLI changelog: behaviour change)

Today `--app` filters only the listing, products and TestFlight. Signing and
team are always exported for the whole account, so `--app com.acme` still
writes every Bundle ID, device, merchant ID and user.

Shape:

```
apple terraform export --out DIR [--app APP]... [--only PART]... [--list] [--force] [--json]
```

| Part (`--only`) | Scope | Written as | Under `--app` |
|---|---|---|---|
| `listing` | app | resources | yes |
| `products` | app | resources | yes |
| `testflight` | app | resources | yes |
| `bundle-ids` (with capabilities) | app | resources | yes |
| `profiles` | app | resources | yes |
| `certificates` | referenced | data sources, always | only those the profiles use |
| `devices` | account | resources | data sources, only those the profiles use |
| `merchant-ids`, `pass-type-ids` | account | resources | only when named in `--only` |
| `users`, `invitations` | account | resources | only when named in `--only` |

Aliases keep today's commands working: `signing` = bundle-ids, profiles,
certificates, devices, merchant-ids, pass-type-ids; `team` = users,
invitations; `app` = listing.

- [ ] An app owns its own Bundle ID and every one under it (`com.acme.*`),
      except one that is another app record's own Bundle ID
      (`com.acme.staging` goes with that app, not as an extension). This needs
      one `GetApps` call even under `--app`.
- [ ] Bundle IDs that look related but sit outside the prefix
      (`com-acme-widget`) are listed in `EXPORT_NOTES.md` as unclaimed, not
      guessed at.
- [ ] Profiles are matched through `GetProfileLinkages`, which the export
      already calls per profile.
- [ ] Referenced devices are written as `data.apple_devices` with
      `udid_pattern = "^<udid>$"`. When `devices` is named in `--only`,
      profiles refer to the `apple_device` resources instead.
- [ ] Defaults: without `--app` and `--only`, the whole account, unchanged.
      With `--app` alone, the app parts plus referenced certificates and
      devices. An explicit `--only` always wins, so
      `--app com.acme --only users` exports only users.
- [ ] Files: under `--app`, signing goes in `<bundle_id>_signing.tf` beside
      `<bundle_id>.tf`, `_products.tf` and `_testflight.tf`. A certificate
      shared by two apps is written once. A whole-account export keeps
      `signing.tf` and `team.tf` as they are, so the golden output only grows.
- [ ] `--list` reads the account and prints what would be written, by app and
      part, plus referenced and unclaimed records, without writing anything
      (`--out` not required; `--json` gives the same as data).
- [ ] `--help` and the generated CLI reference carry the full list of resource
      types per part, so what an export can contain is fixed and documented.
- [ ] Golden output for a fake account with an extension, a staging app and a
      shared certificate; `make validate-examples` over it.

## 3. Verify everything against the live API

- [ ] Move fenn-money to provider 0.8.1 and CLI 0.3.2. That covers the retries
      and the export version pin for free.
- [ ] **`apple release submit`**: add it to fenn-money's `release.yml` after
      "Attach the build", or run it once by hand. It is the most important
      unverified command.
- [ ] Run these on a scratch app, since fenn-money doesn't use them:
      `apple_bundle_id`, `apple_merchant_id`, `apple_pass_type_id`,
      `apple_beta_group_build`, `apple_beta_build_localization`,
      `apple_subscription_price`, `apple_subscription_introductory_offer(_schedule)`,
      `apple_ci_workflow` and its four data sources, `apple screenshots capture`,
      `apple terraform export` (`TestAccExport_firstPlanChangesNothing`).
- [ ] Every new resource from section 6 before it ships.
- [ ] Settle the one remaining code-level unknown: whether Apple ever answers
      "last localization" 409 when deleting a subscription group localization.
- [ ] Give `acceptance.yml` the `APPLE_TEST_APP_ID` secret and run it per release.
      The last recorded run is 2026-09-09.
- [ ] Add a "verified in" column to `ACCEPTANCE_TESTING.md` (which release and
      date, and whether it was fenn-money or the acceptance suite) and remove its
      "unverified" note.

## 4. Build what makes stability checkable

- [ ] **Upgrade tests**: for each resource, apply with the previous release
      (`ExternalProviders`), then plan with the working tree, expecting an empty
      plan. Past regressions (profile import, `platform`,
      `has_access_to_all_builds`) were all of this kind.
- [ ] **Unit tests for create/read/update/delete code** that runs only against the
      live API today, through `httptest` and `resource.UnitTest`, for the riskiest
      reconcilers: the subscription price schedule, the introductory offer
      schedule, the screenshot set, `apple_beta_tester`, `apple_user_invitation`.
- [ ] **Schema audit before freezing**: one pass over every resource for
      naming consistency, `RequiresReplace` vs in-place, defaults and
      descriptions. Implement the mutual-exclusivity validators left as TODOs
      (`bundle/data_source.go:65`, `certificate/data_source.go:69,88`). Anything
      renamed has to happen before 1.0.
- [ ] **Compatibility policy** (`COMPATIBILITY.md`), covering the provider
      schema and import IDs, the CLI's flags, `APPLE_*` variables, exit codes and
      `--json` shapes, and the export's version-pin contract. State how long a
      deprecation lasts and how it is announced.
- [ ] Bring `CLAUDE.md` up to date: the `ci` package (Xcode Cloud) and the
      `releasing` guide aren't described.

## 5. Distribution and project hygiene

- [ ] **Publish to the OpenTofu Registry** as `ahmedosman00/apple`. OpenTofu
      resolves the address against `registry.opentofu.org`, where the provider
      isn't listed, so OpenTofu users currently can't install it.
- [ ] **Release as a draft and publish once every asset is attached**
      (`release.draft: true` in `.goreleaser.yml`, then `gh release edit
      --draft=false`), so neither registry ingests half a release.
- [ ] Run the release job in a GitHub `release` environment that holds the
      signing key and admits only `v*` / `cli-v*` tags.
- [ ] `SECURITY.md` with a contact, and `CODEOWNERS`.

## 6. Features that make it complete

For 1.0, subscription offers (each a resource plus a listing data source, with
an exporter or a recorded reason to leave it out, an `introducedIn` entry and
an example):

- [ ] **Promotional offers** (`apple_subscription_promotional_offer(s)`): the
      discounts an app presents to existing and lapsed subscribers, requested by
      `offer_code` with a server-computed signature. Apple's update request
      carries only the prices, so `prices` changes in place and everything else
      replaces. Read prices from the offer's own collection, since the include is
      capped at 50 and an offer priced everywhere would be truncated. Import by
      `<subscription_id>/<offer_code>` as well as the offer ID.
- [ ] **Win-back offers** (`apple_subscription_win_back_offer(s)`): eligibility
      (paid tenure, months since lapsing, wait between redemptions), dates,
      priority and promotion change in place; prices, mode, duration and
      identifiers replace, with a new offer ID, since Apple treats one as used
      once it has been. Apple creates them only for an approved subscription, so
      the live test needs one.
- [ ] **Offer codes** (`apple_subscription_offer_code`, `_custom_code`,
      `_one_time_use_code`, and data sources for the codes and the one-time-use
      code values).
- [ ] An `apple_subscription_introductory_offers` listing data source, the one
      piece of introductory offer coverage still missing. An implementation
      already exists under MPL-2.0 and can be ported rather than written: a
      required `subscription_id`, a `territories` filter sent to Apple as
      `filter[territory]`, an `offer_mode` filter applied in memory, and one
      record per territory (`id`, `territory_id`, `offer_mode`, `duration`,
      `number_of_periods`, `price_point_id`, `start_date`, `end_date`,
      `target_subscription_plan_type`), ordered by territory then start date.
      Read it through the existing `GetSubscriptionIntroductoryOffers`. Bring it
      in line with the other listing data sources on the way in: the filters
      in `filters.go` with table tests, `limit` / `sort_by` / `sort_order`,
      `total_count` / `filtered_count`, and no `last_updated`.

Also for 1.0:
- [ ] Phased release (`appStoreVersionPhasedReleases`): a setting on most
      releases.
- [ ] Price point equalizations for apps and in-app purchases
      (`/v1/appPricePoints/{id}/equalizations`,
      `/v1/inAppPurchasePricePoints/{id}/equalizations`), so selling everywhere
      no longer means picking 175 price points by hand.
- [ ] Export compliance in the CLI: `build verify` checks
      `ITSAppUsesNonExemptEncryption`, or a command sets
      `usesNonExemptEncryption` on a build. Today it is a manual step that parks
      the version in `WAITING_FOR_EXPORT_COMPLIANCE`.

After 1.0 (additive, so they don't block it): custom product pages, in-app
events, app preview videos, sandbox testers, Game Center, App Clips.

## 7. Redesign the documentation

The current docs are generated from schema descriptions plus three guides
(`getting-started`, `code-signing`, `releasing`). That makes them a reference
of 65 pages that vary from 62 to 499 lines, without a path through them.
Problems:

- **No index template.** `docs/index.md` is the provider's `MarkdownDescription`.
- **The CLI isn't on the Registry.** Its reference lives in `cli/docs/` as
  cobra output, and nothing on the Registry ties a resource to the CLI command
  that completes it (build upload, submit, signing install).
- **The "why" is scattered.** Apple's behaviour (adopt-on-create, prerequisites
  such as availability before price, irreversible deletes) is spread across
  long attribute descriptions instead of a consistent place on each page.
- **No task-oriented guides** for most of what the provider is for.

Plan:

- [ ] `templates/index.md.tmpl`: what it manages, what it deliberately doesn't
      (no `apple_app`, App Privacy, submission as an event), and a map of
      guides.
- [ ] Guides organised by job: Getting started · Code signing (match replacement) ·
      App Store listing (deliver replacement) · Pricing in every storefront ·
      Subscriptions and offers · TestFlight (pilot replacement) · Team and
      access · Releasing with the CLI · Adopting an existing account
      (`terraform export`) · Migrating from fastlane / EAS · Using with OpenTofu ·
      Troubleshooting (Apple error text → cause → fix, starting with the 409s
      already mapped).
- [ ] A fixed structure for every resource page, via `templates/resources/`
      where the generated page isn't enough: what it is at Apple · lifecycle
      (real create/delete, or adopts and drops) · prerequisites · example ·
      schema · import · pitfalls. Move the long Apple explanations out of
      attribute descriptions and into that structure.
- [ ] A CLI reference landing page with one page per command group, linked
      from the guides.
- [ ] A rewritten README: a short pitch, a 60-second example, links to the
      guides.
- [ ] Decide whether the Registry is enough or the project wants a docs site.

## 8. Release 1.0

- [ ] Sections 1–5 done. Section 6's 1.0 items shipped in a 0.x release first.
- [ ] One release candidate used by fenn-money for a full release cycle (TestFlight
      → submit → live) with no manual fixes.
- [ ] Release provider 1.0.0 and CLI 1.0.0 together, with `MinProviderVersion`
      and the export pin moved to `>= 1.0.0, < 2.0.0`.
