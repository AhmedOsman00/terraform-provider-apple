# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Terraform provider (`terraform-provider-apple`, module `github.com/AhmedOsman00/terraform-provider-apple`) for Apple App Store Connect, built on the **Terraform Plugin Framework** (not SDK v2). It is published on the Terraform Registry as `ahmedosman00/apple`. It manages Bundle IDs, Bundle ID capabilities, certificates, devices, merchant IDs, Pass Type IDs, and provisioning profiles, plus auto-renewable subscriptions (groups, subscriptions, localizations, prices, availability) and one-time in-app purchases (purchases, localizations, price schedules, availability).

## Commands

```bash
make            # default: fmt + lint + install + generate
make build      # go build -v ./...
make install    # go install -v ./...
make fmt        # gofmt -s -w -e .
make lint       # golangci-lint run  (config in .golangci.yml)
make test       # go test -v -cover -race -timeout=120s -parallel=10 ./...
make testacc    # TF_ACC=1 go test -v -cover -timeout 120m ./...
make generate   # regenerates docs/ from schemas + examples/ (see caveats below)
make validate-examples  # terraform validate over every directory under examples/
```

Run a single test: `go test -v ./internal/apple/ -run TestGetAllPages`, or `TF_ACC=1 go test -v ./internal/provider/ -run TestAccBundleIDResource` for an acceptance test.

`go test ./...` is safe without credentials: acceptance tests skip themselves when the App Store Connect environment variables are absent. Setting `TF_ACC=1` without credentials also skips rather than fails. Acceptance tests hit the real API and create billable resources.

Note that `terraform-plugin-testing` drives a real `terraform` binary even for `resource.UnitTest`, so the provider schema tests need one — it is downloaded automatically if not on PATH.

### `make generate` caveats

`tools/tools.go` drives generation via `go:generate` and requires the `terraform` binary on PATH (`terraform fmt -recursive ../examples/`).

The copywrite step stamps `// Copyright (c) AO Studio` + `// SPDX-License-Identifier: MPL-2.0` headers into source files, configured by `.copywrite.hcl` (MPL-2.0, holder "AO Studio", matching `LICENSE`). `examples/`, `docs/`, and the tooling configs are excluded via `header_ignore`. Every `.go` file now carries the header, so the step is a no-op unless a file is added.

Generation runs `terraform fmt` but never `terraform validate`, which is why
`make validate-examples` (`scripts/validate-examples.sh`) exists separately: it
builds the provider into a throwaway filesystem mirror, synthesizes a
`required_providers` block for the example directories that have none, and runs
`terraform init` + `terraform validate` in each. That is the only check that
catches an example written against a schema the provider does not have — a
`settings { ... }` block where the schema is a `ListNestedAttribute`, an
attribute that does not exist, or a value a validator rejects. `dev_overrides`
cannot be used for this: it makes `terraform init` refuse to run.

`tfplugindocs` builds `docs/` from provider/resource/data-source `MarkdownDescription` strings plus the matching files under `examples/`. CI (`.github/workflows/test.yml`) fails the `generate` job if `make generate` produces a diff, so regenerate and commit whenever a schema, example, or guide changes.

`docs/` is fully generated and must never be hand-edited — tfplugindocs deletes and re-renders the whole directory on every run. All thirty pages are checked in: `index.md`, sixteen under `resources/`, and thirteen under `data-sources/`.

Hand-written prose lives in `templates/`, which is the only part of the docs pipeline a human edits directly. `templates/guides/<name>.md.tmpl` renders to `docs/guides/<name>.md`; a `templates/` directory does not suppress the auto-generated resource and data-source pages, which are still built from the schemas into a temporary directory. Two guides exist: `getting-started` (credentials, installing the provider from the Registry and overriding it with a local build, first configuration, import IDs per resource) and `code-signing` (the `fastlane match` replacement, the state-vs-bundle distribution model, adopting a match certificate).

## Architecture

Two layers, strictly separated, plus a standalone CLI:

### `internal/apple/` — App Store Connect API client

- `client.go`: `Client` holds the JWT (ES256, `kid` header, 20-minute expiry) built from issuer ID + key ID + PKCS#8 private key. The token is minted once in `NewClient` and is immutable thereafter — a 20-minute life outlives any single Terraform command, so there is deliberately no refresh path and the signing key is not retained. `doRequest` decodes `models.ErrorResponse` into a formatted error, preferring Apple's own message; a 401 is reported as a credentials failure rather than retried, since re-signing with the same key yields an equivalent token.
- One file per Apple resource (`budleIds.go` — note the typo in the filename — `certificates.go`, `devices.go`, `merchantIds.go`, `passTypeIds.go`, `profiles.go`, `bundleIdCapabilities.go`, `apps.go`, `subscriptionGroups.go`, `subscriptions.go`, `subscriptionLocalizations.go`, `subscriptionPrices.go`, `subscriptionAvailabilities.go`, `inAppPurchases.go`, `inAppPurchaseVersions.go`, `inAppPurchaseLocalizations.go`, `inAppPurchasePrices.go`, `inAppPurchaseAvailabilities.go`). Each exposes plain `Get*/Create*/Update*/Delete*` methods on `*Client` returning `models.*` structs. No Terraform types here.
- `models/`: JSON wire types. All responses go through the generics in `common.go`: `Response[T]`, `ListResponse[T]`, `Request[T]`.
- `pagination.go`: `getAllPages[T]` walks every page of a collection, following the absolute `links.next` URL until it is empty and refusing links that point off-host. All seven `Get*s()` functions go through it — reading only the first page silently truncates results. A `pageSize` of `noPageSize` (0) omits the `limit` parameter entirely, which the `bundleIdCapabilities` relationship requires: it rejects `limit` with a 400. `getAllPagesQuery` is the same walk with extra query parameters applied to the first request only — Apple's `links.next` already carries them forward, so re-appending would duplicate them. The subscription and in-app purchase collections need it: a price is unreadable without its price point included, and either price point catalogue is unusable without `filter[territory]`.
- Apple's API has no server-side filtering here — "get by identifier/name" helpers (e.g. `GetPassTypeIDByIdentifier`, `GetProfileByName`) list a full collection and scan client-side, so they depend on pagination being complete. The App Store Connect endpoints are the exception: `/v1/apps` supports `filter[bundleId]` and the price point catalogue supports `filter[territory]`, and both are used server-side because the collections are too large to pull whole.

### `internal/provider/` — Terraform layer

`provider.go` defines the `apple` provider: credential schema with regex validators (issuer ID = UUID, api_key = 10 chars `[A-Z0-9]`, private_key = PEM block), reads env vars **first** and lets explicit config override them, builds the `*apple.Client`, and hands it to every resource/data source via `resp.ResourceData` / `resp.DataSourceData`.

The optional `scope` attribute restricts the JWT to named operations. It must be left unset rather than configured as an empty list: `createToken` omits the claim unless it is populated, because App Store Connect rejects a token carrying an empty or null `scope` with a 400 `ENTITY_INVALID` titled "JSON processing failed" — an error about the token that reads like a complaint about the request body, and one that broke every API call the provider made.

Env vars: `APPLE_APP_STORE_CONNECT_ISSUER_ID`, `APPLE_APP_STORE_CONNECT_API_KEY`, `APPLE_APP_STORE_CONNECT_PRIVATE_KEY`.

Each domain lives in its own package (`bundle`, `certificate`, `device`, `merchant`, `passtypeid`, `profile`, `subscription`, `inapppurchase`, `app`) following a consistent four-file convention:

| File | Contents |
|---|---|
| `models.go` | `tfsdk`-tagged state structs + exported `Get*Validator()` helpers shared by resource and data source |
| `resource.go` | CRUD + `Configure` + `ImportState` |
| `data_source.go` | plural listing data source: filter attrs, `limit`/`sort_by`/`sort_order`, computed `total_count`/`filtered_count` |
| `filters.go` | pure `Filter*` / `Sort*` / `Limit*` functions over `[]models.*`, applied in-memory after fetching |

`bundle` additionally carries `capability_*.go` for the `apple_bundle_id_capability` resource/data source, where `capability_models.go` holds `SettingsFromAPI`/`SettingsToAPI` and `OptionsFromAPI`/`OptionsToAPI` to convert nested capability settings between API structs and `types.List`.

`subscription` carries five resources rather than one, so it prefixes by kind
the way `bundle` does: `group_*.go`, `localization_*.go`, `price_*.go`,
`availability_resource.go` and `price_point_data_source.go` alongside the
unprefixed `resource.go` / `data_source.go` for `apple_subscription` itself.
`app` is a data source only.

`inapppurchase` is the same shape for one-time purchases: `localization_*.go`,
`price_schedule_resource.go`, `availability_resource.go` and
`price_point_data_source.go` alongside the unprefixed `resource.go` /
`data_source.go` for `apple_in_app_purchase`. It shares nothing with
`subscription` — Apple models the two as separate resources, and a price point
from one is meaningless to the other.

**App Store Connect resources differ from Developer Portal ones in two ways
that shape the whole `subscription` package.** They hang off an app record,
which Apple's API cannot create — its documentation says "Don't use this API to
create new apps; instead, create new apps on the App Store Connect website" —
so there is deliberately no `apple_app` resource, only the `apple_apps` data
source. And they nest: a group holds subscriptions, a subscription holds
localizations and prices, and Apple publishes **no top-level collection** for
any of them (`GET /v1/subscriptions` and `GET /v1/subscriptionGroups` are both
404), so the parent must be known before a child can be read. That is why every
listing data source takes a required scope argument, and why several import
forms are composite.

In-app purchases take both of those further. `GET /v2/inAppPurchases` is a 404
too — a purchase is reachable only through `/v1/apps/{id}/inAppPurchasesV2` —
and the purchase record has **no app relationship at all**, so the parent cannot
even be read back from a child that was found. Their metadata nests one level
deeper as well: localizations hang off an in-app purchase version rather than
the purchase.

**Adding a resource or data source requires registering the constructor in both `DataSources()` and `Resources()` in `internal/provider/provider.go`** — nothing is discovered automatically.

### `examples/signing` — the fastlane match replacement

A runnable module, not a documentation snippet. It manages the App ID, its
capabilities, the devices, the certificates and one profile per distribution
method.

**The signing key never enters Terraform.** The user generates the key and the
CSR locally and supplies only `var.csr_contents`; a CSR is a public key plus a
signature over it. That single decision is what makes the state ordinary: a
CSR, an issued certificate and a provisioning profile are all public documents,
so nothing in the state or the outputs is secret, and the module needs no
secret-store dependency, no encrypted bundle, and no companion CLI. The last
mile — a `.p12` assembled from the certificate and the user's key, plus
profiles copied where Xcode reads them — is documented in
`examples/signing/README.md` and `templates/guides/code-signing.md.tmpl` rather
than automated.

There was previously a `cmd/applesign` CLI and a `local.signing_bundle` JSON
document carrying private keys between them. Both are gone; do not reintroduce
either without also reintroducing the reason they existed.

Certificates reach the outputs through `local.certificates`, which merges the
ones the module issues with ones adopted from an existing serial via
`data.apple_certificates` — adoption exists because importing an
`apple_certificate` reissues it (`csr_content` forces replacement and Apple does
not reliably return the CSR), and issuing a replacement revokes the original. An
adopted role needs no CSR.

Two things there are easy to get wrong:

- `local.certificates` and the `profiles` output call `nonsensitive()` on
  `certificate_content` and `profile_content`. The provider marks both
  `Sensitive`, but they are public documents, and leaving them wrapped would
  force every output to be sensitive and redact values that are safe to print.
  `nonsensitive()` **errors on a value that is not sensitive**, so un-marking
  either attribute in the provider schema breaks this module.
- `required_version` is `>= 1.9` because `var.csr_contents` uses a `validation`
  block that references `var.adopt_certificate_serials` — cross-variable
  validation landed in 1.9. That check is what enforces "the distribution role
  needs either a CSR or a serial", which nothing else can express: a missing
  role would otherwise surface as an invalid map index deep in a profile.

Note that `terraform validate` does not evaluate variable validation for a
variable left at its default, so `make validate-examples` passes without any
CSR configured. The rules only fire at plan time.

## Conventions to follow

- Data sources deliberately expose **no `last_updated` attribute**. A data source is re-read on every plan and the framework gives it no prior state, so such a value can only ever be "now" — it churns every run and propagates a perpetual diff to anything referencing it. Do not reintroduce it.

- Resource type names come from `req.ProviderTypeName + "_bundle_id"` in `Metadata`; data sources use the plural form (`_bundle_ids`).
- Every resource/data source declares its interface assertions (`var _ resource.ResourceWithImportState = &fooResource{}`) and a `Configure` that type-asserts `req.ProviderData.(*apple.Client)`, erroring with the received type on mismatch.
- Immutable Apple attributes get `stringplanmodifier.RequiresReplace()`; Apple-computed attributes get `stringplanmodifier.UseStateForUnknown()`.
- `MarkdownDescription` (not `Description`) everywhere — it is the source for generated docs, so enumerate valid enum values in the string.
- Error handling is deliberately verbose: `strings.Contains` on the API error to map "already exists" / "not found" / 404 into specific, actionable diagnostics rather than surfacing raw HTTP errors. Match this when adding operations.
- Logging uses `tflog` with `tflog.SetField` to attach structured context; never log credential values.
- `ImportState` is hand-written per resource and generally accepts either Apple's opaque ID or a human identifier (bundle identifier, UDID, certificate serial/display name, profile name), disambiguating by string shape.

## Resource-specific behavior worth knowing

- **`apple_certificate`**: immutable — every attribute describing the certificate is `RequiresReplace`, and Delete revokes it. `certificate_content` is `Sensitive`. Optional `early_renewal_hours` triggers replacement before expiry: `ModifyPlan` compares prior state's `expiration_date` against the window, flips the computed `ready_for_renewal` to true, marks the Apple-issued attributes unknown, and reports `ready_for_renewal` as the path forcing replacement. `Create`/`Read`/`ImportState` all record `ready_for_renewal` as false so the flip registers as a change. `Update` exists solely to accept a changed `early_renewal_hours` in place — it copies the issued certificate forward untouched. `Read` never refreshes `csr_content` or the `requester_*` fields from the response: Apple returns `csrContent` as null and omits the requester fields, so assigning them would blank values only the configuration and the create response hold — and since `csr_content` is `RequiresReplace`, a blanked CSR makes the next plan revoke the certificate and issue a new one.
- **`apple_bundle_id`**: `platform` accepts `IOS`, `MAC_OS` and `UNIVERSAL` — Apple rejects `TV_OS` and `WATCH_OS`, and tvOS and watchOS App IDs are created as `UNIVERSAL`. Apple then stores *every* Bundle ID as `UNIVERSAL` regardless of what was sent, so `Create` and `Read` keep the configured value rather than the reported one: refreshing it would rewrite a configured `IOS` to `UNIVERSAL`, and `platform` is `RequiresReplace`, so the next plan would destroy the Bundle ID and take its capabilities with it. `Read` only adopts Apple's value when state has none (an import) or when Apple reports something that was never a valid input.
- **`apple_merchant_id`**: Apple names the human-readable field `name` on `merchantIds`, not `displayName`; the Terraform attribute stays `display_name` and only the wire tag differs. Import distinguishes the two accepted forms by the `merchant.` prefix — Apple's own IDs are 10-character alphanumerics like `4B62XU6945`, not UUIDs.
- **`apple_device`**: the Apple API cannot delete devices, so `Delete` disables the device (`PATCH` with `status: DISABLED`) and lets Terraform drop it from state, emitting a warning. A 404 while disabling is treated as already-gone so destroy still succeeds.
- **`apple_bundle_id_capability`**: Apple exposes no `GET` for a single capability (403 `does not allow 'GET_INSTANCE'`), so every read lists the parent Bundle ID's collection and scans — which is why the parent has to be known. Import accepts `<bundle_id>/<capability_id>`; a bare capability ID recovers the parent from the ID itself, which Apple composes as `<bundleID>_<CAPABILITY_TYPE>`, and confirms it against the collection before writing state. The lookup through the parent *is* the ownership check. Capability writes are serialized per Bundle ID (`capability_serialize.go`) because Apple applies them as an unlocked read-modify-write over the whole set: two concurrent creates both return 201 and one silently vanishes. The resource models only what Apple accepts as input — `key` plus `value` or `options` — since Apple has no scalar `value` property (`value` is sent as a one-element options list) and its display metadata, exposed as `Optional+Computed` inside a list, made every plan non-empty.

  The resource and the data source therefore describe a setting differently on purpose: `name`, `visible`, `min_count` and the per-option labels stay on the data source (`capability_data_source.go`), which only reports, and are absent from the resource, which only writes. Dropping them from the resource narrowed a published schema, which is normally breaking — it was safe here solely because no configuration could have been relying on them: every capability operation failed before this, so the resource had never created anything. That escape hatch does not generalise. Removing an attribute from a resource that works needs a deprecation cycle instead.
- **`apple_subscription_group`**: `app_id` is write-once and unreadable. Apple's update request has no `app` member and `GET /v1/subscriptionGroups/{id}` accepts no `app` value for its `include` parameter, so the owning app can never be refreshed — `Read` keeps the configured value, and `ImportState` accepts only the composite `<app_id>/<group_id>` form, confirming ownership against the app's collection before writing state. `reference_name` is the only mutable attribute; it is internal to App Store Connect, and the customer-facing name is a subscription group localization this provider does not manage.
- **`apple_subscription`**: `product_id` is `RequiresReplace` because Apple's `SubscriptionUpdateRequest` has no `productId` member — and replacing it is expensive in a way Terraform cannot express, because **Apple never releases a product identifier**, not even one belonging to a subscription deleted before it was ever approved. `group_id` is likewise immutable: a subscription cannot move between groups. `state` is Apple's and always starts at `MISSING_METADATA`; it only leaves once a localization *and* a price exist, which is why those are separate resources rather than optional attributes. `Read` and `ImportState` rely on `GetSubscription` requesting `include=group`, without which Apple reports the relationship as links alone and an import would leave `group_id` null.
- **`apple_subscription_localization`**: `locale` is `RequiresReplace` — it identifies the record and is absent from Apple's update request. This is the customer-facing name and description; the `name` on `apple_subscription` is an internal reference name. Import accepts a bare localization ID (the client asks for `include=subscription`) or the composite `<subscription_id>/<localization_id>`, which additionally checks ownership.
- **`apple_subscription_price`**: every attribute is `RequiresReplace` and `Update` exists only to report that it was reached, because Apple publishes no `PATCH /v1/subscriptionPrices` — a price change is a new record plus a deletion. There is also no `GET` for a single price, so `Read` lists the subscription's price collection and scans it, the same shape `apple_bundle_id_capability` is forced into; import is therefore composite. A price is never a number: `price_point_id` references Apple's catalogue, read through `apple_subscription_price_points`, and a price point ID encodes the subscription it belongs to, so it cannot be reused across subscriptions. `preserve_current_price` is an instruction Apple does not report back — only its outcome, through the computed `preserved` — so import leaves it null and `ImportStateVerifyIgnore` covers it.
  **A price requires an `apple_subscription_availability` to exist first**, and nothing about the failure says so: Apple answers `POST /v1/subscriptionPrices` with a 409 `There is a problem with the request entity - An error occurred while processing the pricing information`, naming neither availability nor the territory. That message cost a full afternoon of eliminating hypotheses — the price point, an empty `attributes` member, a missing `territory` relationship, and app-level pricing and availability were all ruled out by experiment before the subscription's own availability turned out to be the prerequisite. `Create` therefore special-cases the string `processing the pricing information` and names the missing availability in the diagnostic; do not fold that case back into the default branch. Nothing in a price references an availability, so a configuration has to declare `depends_on` to get the ordering — both the example and the acceptance test do.
  Two things follow from a price that actually exists, both of which only became reachable once availability unblocked creation. **`territory_id` is `Optional+Computed`, not merely Optional:** Apple reports the territory of every price whether or not one was sent, so a plain Optional attribute had `Read` writing `USA` into state against a configuration holding null — and because the attribute is `RequiresReplace`, the next plan destroyed the price to remove it. Making it computed then exposed the other half: Apple omits the territory from the create response — only a read carries one, since the read asks for `include=territory` and a `POST` takes no `include` — so `Create` re-reads the price to resolve the unknown, and falls back to null rather than leaving an unknown the framework would reject. **`Delete` tolerates Apple's 409 `Only future price changes can be deleted`:** a live price is not a record that can be withdrawn, it is what the subscription currently costs, and the only way past it is a later price that supersedes it. Warn and drop state, the way `apple_device` does; deleting the subscription removes its prices anyway.
- **`apple_subscription_availability`**: the same singular, `POST`-replaces, no-`PATCH`, no-`DELETE` shape as `apple_in_app_purchase_availability`, and the same consequences — `Update` posts again, `Delete` warns and drops state only, and `ModifyPlan` marks `id` unknown when anything else changes because Apple issues a new record ID on every `POST`. It is a separate Apple resource from the in-app purchase one (`/v1/subscriptionAvailabilities` against `/v1/inAppPurchaseAvailabilities`) and the two share no code, the same way the two price catalogues do not. The record carries only `availableInNewTerritories`; the territory list is a paginated collection under `/v1/subscriptionAvailabilities/{id}/availableTerritories`, so `Read` takes two requests. Import ID is the subscription ID. Its real significance is ordering: it gates pricing, so a subscription cannot leave `MISSING_METADATA` without it.
- **Customer-facing text is validated in characters, not bytes.** `GetNameValidator` and `GetDescriptionValidator` — in both `subscription` and `inapppurchase` — use `stringvalidator.UTF8LengthBetween`, not `LengthBetween`. Apple's 30- and 45-character limits are character counts, while `LengthBetween` counts bytes, which rejected a legal 27-character Arabic description at "49". Any new customer-visible string field must follow this.
- **`apple_in_app_purchase`**: the one-time purchase side of the App Store — `CONSUMABLE`, `NON_CONSUMABLE`, `NON_RENEWING_SUBSCRIPTION` — on `/v2/inAppPurchases`, and a different Apple resource from `apple_subscription`. `app_id` is worse than write-once: `InAppPurchaseV2` has **no app relationship at all** and no `include` produces one, so the app can never be read back, `Read` keeps the configured value, and `ImportState` accepts only `<app_id>/<in_app_purchase_id>` — confirming ownership by listing the app's collection, which is the only check the API offers. `product_id` and `in_app_purchase_type` are `RequiresReplace` because Apple's update request has neither member, and replacing either is expensive: Apple never releases a product identifier. `Read` never refreshes `app_id`.
- **`apple_in_app_purchase_localization`**: localizations hang off an **in-app purchase version**, not the purchase. Apple moved them there in App Store Connect API 4.4.1 and deprecated the v1 endpoints that hid it, so the provider writes to `/v2/inAppPurchaseLocalizations` against a version resolved by `EnsureEditableInAppPurchaseVersion`: it picks the highest-numbered version in `PREPARE_FOR_SUBMISSION`, `DEVELOPER_REJECTED` or `REJECTED`, and creates one when every version is in review or approved — which is what App Store Connect's UI does when you edit approved metadata. Versions have no `DELETE`, so one created that way outlives the localization. The resolved version is reported as the computed `version_id`. Unlike a subscription localization there is **no `state`**: `InAppPurchaseLocalizationV2` has none, because the state lives on the version. Import accepts a bare ID — the parent is resolved in two hops, localization → version → purchase — or the composite `<in_app_purchase_id>/<localization_id>`. **Delete tolerates Apple's 409 `Cannot delete the last localization`**: a version must keep at least one, so destroying the only localization warns and drops state rather than erroring — erroring failed the whole destroy and left the purchase behind, and the record goes away with the purchase anyway. `ImportState` sets `review_note` itself instead of going through `applyPurchaseAttributes`, which skips it so that a null in configuration survives a `Read`; an import has no configuration to contradict, and leaving it null dropped a value Apple holds.
- **`apple_in_app_purchase_price_schedule`**: a purchase has exactly **one** schedule, not a price record per territory the way a subscription does. `POST /v1/inAppPurchasePriceSchedules` replaces it wholesale, and there is no `PATCH` and no `DELETE` — so `Update` posts again (a genuine in-place update, not `RequiresReplace`) and `Delete` warns and drops state only. Both this and `apple_in_app_purchase_availability` need a `ModifyPlan` that marks `id` unknown when anything else changes: Terraform proposes the prior value for a computed attribute, Apple issues a new record ID on every `POST`, and without it every update would be rejected as an inconsistent result. The create request is the only one in the provider using JSON:API's `included` member: the prices do not exist yet, so each is sent inline under a literal placeholder ID (`${price0}`) that the `manualPrices` relationship references, and Apple substitutes real IDs on commit. `baseTerritory` is required — Apple added that after the endpoint shipped. `prices` is a **set**, because Apple returns them unordered, and `Read` matches by `price_point_id` and **keeps the configured dates** for a price already in state: Apple derives an `endDate` for any price a later one supersedes, and adopting it would be a permanent diff. Only `manualPrices` are read; the prices Apple equalizes from the base territory are computed and never the provider's. Import ID is the purchase ID.
- **`apple_in_app_purchase_availability`**: same singular, `POST`-replaces, no-`DELETE` shape as the price schedule, and the same consequences — `Update` posts again, `Delete` warns and drops state only. The record itself carries only `availableInNewTerritories`; the territory list is a separate paginated collection under `/v1/inAppPurchaseAvailabilities/{id}/availableTerritories`, so `Read` takes two requests. `available_in_new_territories` defaults to `true`, matching App Store Connect's own default. Import ID is the purchase ID.
- **`apple_profile`**: every configurable attribute is `RequiresReplace`, `name` included — Apple rejects `PATCH /v1/profiles` with 403 `does not allow 'UPDATE'`, so a rename is a reissue and `Update` exists only to report that it was reached. Profile content/UUID/state/dates are Apple-computed. `CreateProfile` retries a 5xx up to three times: `POST /v1/profiles` intermittently answers 500 for a well-formed request that succeeds on the next attempt. It looks the name up before each retry and adopts an already-issued profile, so a 500 that arrives after Apple committed the write does not duplicate it.

## Local testing

The provider serves `registry.terraform.io/ahmedosman00/apple` (set in `main.go`; lower case because Terraform normalizes source addresses, while the Registry displays the namespace as `AhmedOsman00`). `terraform init` in `examples/` therefore downloads the last released version — to exercise the working tree instead, wire a locally built binary up through a `dev_overrides` block in `~/.terraformrc`, or a filesystem mirror if `init` needs to keep working. Run with `-debug` to attach a debugger.

The examples pin `version = "~> 0.1"`, and `scripts/validate-examples.sh` builds the working tree into a mirror at version `0.1.0` to satisfy that constraint — bump `PROVIDER_VERSION` there if the pin ever moves past it.

## Releasing

Pushing a `v*` tag runs `.github/workflows/release.yml`: GoReleaser cross-compiles the provider (only `main.go`; the Registry matches every entry in `SHA256SUMS` against expected artifact names, so any extra binary built here would fail ingestion), signs the checksums with the GPG key in the `GPG_PRIVATE_KEY` / `PASSPHRASE` secrets, and attaches `terraform-registry-manifest.json`. `project_name` is pinned in `.goreleaser.yml` because the Registry requires `terraform-provider-apple_<version>_<os>_<arch>.zip`, and GoReleaser would otherwise take the name from the checkout directory. The Registry ingests new tags automatically once the repository is connected and the public GPG key is uploaded; update `CHANGELOG.md` before tagging.

## Tests and CI

Two tiers:

- **Credential-free** (`internal/apple/pagination_test.go`, `internal/apple/subscriptions_test.go`, `internal/apple/inAppPurchases_test.go`, `internal/provider/bundle/capability_models_test.go`, `internal/provider/certificate/renewal_test.go`, `internal/provider/device/models_test.go`): `httptest`-backed client tests and pure-function tests. `apple.Client` has all-exported fields, so pointing one at a test server needs no production seam — `&apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}`.
- **Acceptance** (`internal/provider/*_test.go`, package `provider`): `terraform-plugin-testing` against the real API. `testAccPreCheck` skips when credentials are absent.

All sixteen resources now have acceptance coverage. Checks go through `testAccAPIClient()` (provider_test.go), which builds a client from the same environment variables the provider reads: a `CheckDestroy` that only inspects Terraform state passes even when the resource is still live in the portal, so every existence and destroy check asks Apple. `testAccCheckDestroyAll` composes the checks for configurations that create several kinds of resource; the bundle ID checks predate this and still assert nothing.

Two things constrain what the acceptance tier is allowed to do:

- **Devices are permanent.** Apple has no device delete, so `device_resource_test.go` registers two fake UDIDs that stay in the team forever and consume device slots. Nothing new should register devices — which is why the profile tests use `IOS_APP_STORE` (needs no devices) rather than a development or ad hoc type. The UDIDs are hardcoded and `Delete` only disables, so a second run of those tests hits Apple's 409 and fails on `Device Already Exists`: they pass once per team, ever. CI skips them for that reason; run them by hand when you have accepted the slots.
- **App Store Connect tests need an app that already exists.** Apple's API cannot create an app record, so `TestAccSubscription*`, `TestAccInAppPurchase*` and `TestAccApps*` skip unless `APPLE_TEST_APP_ID` names one — `testAccPreCheckSubscription` enforces it, and `testAccPreCheckInAppPurchase` is a thin wrapper over it, and `.github/workflows/acceptance.yml` warns rather than passing silently when the secret is absent. Product identifiers are **randomised per run** here, deliberately breaking the fixed-identifier convention the rest of the suite follows: Apple reserves every product ID it has ever seen, so a fixed one would pass exactly once per account and fail forever after, the way the device tests do. In-app purchase product identifiers follow the same rule and the same helper shape (`testAccInAppPurchaseProductID`). **Product *names* are randomised too**, through `testAccProductName`: Apple enforces name uniqueness across an app's live products — "This name is already being used by another in-app purchase associated with this app", subscriptions included — so a fixed name collides with whatever a failed test left behind and with the test that ran a second earlier. Unlike an identifier a name is released when the product is deleted, so this is only about overlap, not permanent reservation. `TestAccInAppPurchaseResource_completesMetadata` additionally leaves a price schedule and an availability behind on a purchase it then deletes — both are undeletable in their own right, but deleting the purchase takes them with it. Randomising still consumes a handful of identifiers per run from an unlimited namespace, which is harmless; a fixed one would consume the only usable value.
- **Certificates consume a slot while they exist.** `certificate_resource_test.go` and both profile test files issue one and revoke it on destroy, so a completed run is neutral; an interrupted run leaves a certificate to revoke by hand. `IOS_DEVELOPMENT` is preferred over a distribution type where the profile type allows it.

Identifiers are fixed rather than randomised, matching the existing tests, so an interrupted run leaves a Bundle ID, Merchant ID or profile that must be deleted in the portal before the test passes again.

`ACCEPTANCE_TESTING.md` is the per-test reference: what every `TestAcc*` creates at Apple, where it appears in the portal, what it leaves behind, the cleanup list for an interrupted run, and a staged run order that defers the two irreversible device tests. It is hand-written and unverified — update the matching table when you add or rename an acceptance test.

One trap it records: Bundle ID identifiers cannot contain the underscore the platform constants carry (`BundleIdentifierValidator` is `^[a-zA-Z0-9.-]+\.[a-zA-Z0-9.-]+$`), and Apple's App ID *name* accepts only alphanumerics and spaces. `testAccBundleIDPlatformFixture` does both substitutions for `TestAccBundleIDResource_platforms`; interpolating a platform constant straight into either field fails.

`.github/workflows/test.yml` is the credential-free lane and runs four independent jobs: `build` (build + lint), `generate` (docs diff), `examples` (`make validate-examples`), and `unit` (`go test ./...`, where the acceptance tests skip themselves for want of `TF_ACC` and credentials). It triggers on every pull request and on pushes to `main` only — a push to a branch with an open PR would otherwise run it twice for the same commit — and cancels superseded runs.

Acceptance tests live in their own workflow, `.github/workflows/acceptance.yml`, and run **only on `workflow_dispatch`**: they create billable resources in a real Apple team, use fixed identifiers that collide if two runs overlap, and leave material to clean up by hand when interrupted. Its inputs are the Terraform version (one per run, not a matrix), a `-run` pattern, and an `include_device_tests` boolean that defaults to false — `TestAccDeviceResource*` registers two UDIDs permanently and passes once per team, ever. The job fails fast if the three `APPLE_APP_STORE_CONNECT_*` secrets are absent, because a suite that skips itself would otherwise report green on a run somebody triggered deliberately, and it sets `cancel-in-progress: false` so a queued run never kills one mid-apply.

All nine filter packages have table-driven tests (`internal/provider/*/filters_test.go`) covering filtering, sorting, limiting, and the malformed-pattern errors. Resource CRUD paths are still only reachable through the acceptance tier.

Two behaviours worth knowing when writing filter tests, because they differ per package: `bundle` and `certificate` match `name_pattern` as a **glob** (`filepath.Match`, whole-string), while `device`, `merchant`, `passtypeid`, `profile`, `subscription`, `inapppurchase`, and `app` match it as a **regex** (substring unless anchored). `merchant`'s display-name pattern is additionally case-insensitive.
