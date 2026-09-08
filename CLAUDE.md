# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Terraform provider (`terraform-provider-apple`, module `github.com/AhmedOsman00/terraform-provider-apple`) for Apple App Store Connect, built on the **Terraform Plugin Framework** (not SDK v2). It is published on the Terraform Registry as `ahmedosman00/apple`. It manages Bundle IDs, Bundle ID capabilities, certificates, devices, merchant IDs, Pass Type IDs, and provisioning profiles.

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
make consume    # pipes examples/signing's signing_bundle output into applesign
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

`docs/` is fully generated and must never be hand-edited — tfplugindocs deletes and re-renders the whole directory on every run. All fifteen pages are checked in: `index.md`, seven under `resources/`, and seven under `data-sources/`.

Hand-written prose lives in `templates/`, which is the only part of the docs pipeline a human edits directly. `templates/guides/<name>.md.tmpl` renders to `docs/guides/<name>.md`; a `templates/` directory does not suppress the auto-generated resource and data-source pages, which are still built from the schemas into a temporary directory. Two guides exist: `getting-started` (credentials, installing the provider from the Registry and overriding it with a local build, first configuration, import IDs per resource) and `code-signing` (the `fastlane match` replacement, the state-vs-bundle distribution model, adopting a match certificate).

## Architecture

Two layers, strictly separated, plus a standalone CLI:

### `internal/apple/` — App Store Connect API client

- `client.go`: `Client` holds the JWT (ES256, `kid` header, 20-minute expiry) built from issuer ID + key ID + PKCS#8 private key. The token is minted once in `NewClient` and is immutable thereafter — a 20-minute life outlives any single Terraform command, so there is deliberately no refresh path and the signing key is not retained. `doRequest` decodes `models.ErrorResponse` into a formatted error, preferring Apple's own message; a 401 is reported as a credentials failure rather than retried, since re-signing with the same key yields an equivalent token.
- One file per Apple resource (`budleIds.go` — note the typo in the filename — `certificates.go`, `devices.go`, `merchantIds.go`, `passTypeIds.go`, `profiles.go`, `bundleIdCapabilities.go`). Each exposes plain `Get*/Create*/Update*/Delete*` methods on `*Client` returning `models.*` structs. No Terraform types here.
- `models/`: JSON wire types. All responses go through the generics in `common.go`: `Response[T]`, `ListResponse[T]`, `Request[T]`.
- `pagination.go`: `getAllPages[T]` walks every page of a collection, following the absolute `links.next` URL until it is empty and refusing links that point off-host. All seven `Get*s()` functions go through it — reading only the first page silently truncates results. A `pageSize` of `noPageSize` (0) omits the `limit` parameter entirely, which the `bundleIdCapabilities` relationship requires: it rejects `limit` with a 400.
- Apple's API has no server-side filtering here — "get by identifier/name" helpers (e.g. `GetPassTypeIDByIdentifier`, `GetProfileByName`) list a full collection and scan client-side, so they depend on pagination being complete.

### `internal/provider/` — Terraform layer

`provider.go` defines the `apple` provider: credential schema with regex validators (issuer ID = UUID, api_key = 10 chars `[A-Z0-9]`, private_key = PEM block), reads env vars **first** and lets explicit config override them, builds the `*apple.Client`, and hands it to every resource/data source via `resp.ResourceData` / `resp.DataSourceData`.

The optional `scope` attribute restricts the JWT to named operations. It must be left unset rather than configured as an empty list: `createToken` omits the claim unless it is populated, because App Store Connect rejects a token carrying an empty or null `scope` with a 400 `ENTITY_INVALID` titled "JSON processing failed" — an error about the token that reads like a complaint about the request body, and one that broke every API call the provider made.

Env vars: `APPLE_APP_STORE_CONNECT_ISSUER_ID`, `APPLE_APP_STORE_CONNECT_API_KEY`, `APPLE_APP_STORE_CONNECT_PRIVATE_KEY`.

Each domain lives in its own package (`bundle`, `certificate`, `device`, `merchant`, `passtypeid`, `profile`) following a consistent four-file convention:

| File | Contents |
|---|---|
| `models.go` | `tfsdk`-tagged state structs + exported `Get*Validator()` helpers shared by resource and data source |
| `resource.go` | CRUD + `Configure` + `ImportState` |
| `data_source.go` | plural listing data source: filter attrs, `limit`/`sort_by`/`sort_order`, computed `total_count`/`filtered_count` |
| `filters.go` | pure `Filter*` / `Sort*` / `Limit*` functions over `[]models.*`, applied in-memory after fetching |

`bundle` additionally carries `capability_*.go` for the `apple_bundle_id_capability` resource/data source, where `capability_models.go` holds `SettingsFromAPI`/`SettingsToAPI` and `OptionsFromAPI`/`OptionsToAPI` to convert nested capability settings between API structs and `types.List`.

**Adding a resource or data source requires registering the constructor in both `DataSources()` and `Resources()` in `internal/provider/provider.go`** — nothing is discovered automatically.

### `cmd/applesign` — the signing install CLI

A second binary in this module (`go install ./...` builds both). It installs the
signing material the `examples/signing` module produces onto a machine: private
key plus certificate into a keychain, provisioning profiles into the two
directories Xcode reads. This is the last mile `fastlane match` does locally,
and it replaces the deleted `examples/signing/scripts/install-signing.sh`.

The `examples/signing` module assembles that bundle in `bundle.tf` as
`local.signing_bundle`. Certificates reach it through `local.certificates`,
which merges the ones the module issues with ones adopted from an existing
serial via `data.apple_certificates` — adoption exists because importing an
`apple_certificate` reissues it (`csr_content` forces replacement and Apple does
not reliably return the CSR), and issuing a replacement revokes the original.
Note that `var.private_keys` is sensitive, and `for_each` rejects values derived
from sensitive ones, so role names are unwrapped once through
`nonsensitive(keys(var.private_keys))` in `local.supplied_key_roles`.

The bundle is a local and deliberately not inside the `output` block: outputs
cannot be referenced by resources in the same configuration, and a user needs to
be able to add their own `aws_secretsmanager_secret_version` or
`vault_kv_secret_v2` alongside the module without editing it. The module
declares no secret resource itself, so it forces no cloud provider dependency.

`applesign install -` reads the bundle as JSON on **stdin**. That is the design
constraint, not an implementation detail: the CLI must know about no secret
store and no backend, so `terraform output -json signing_bundle`, `sops -d`, and
`op read` all compose as pipes and a consumer needs no state access. A path is
accepted too, mainly so process substitution works.

| File | Contents |
|---|---|
| `bundle.go` | Bundle/Certificate/Profile JSON types, decoding, validation |
| `identity.go` | Certificate and key decoding, key-matches-certificate check, PKCS#12 |
| `keychain.go` | Every `/usr/bin/security` call |
| `profile.go` | Profile directories and installation |
| `install.go` | Command line and orchestration |

Everything but the `security` calls is unit-tested and credential-free.

Two things are load-bearing and easy to break:

- PKCS#12 must use `pkcs12.LegacyRC2` (`software.sslmate.com/src/go-pkcs12`).
  `pkcs12.Modern` is what OpenSSL 3 emits by default and `security import`
  rejects it — this is why the Go rewrite dropped the `openssl`/LibreSSL
  divergence the shell version had to detect.
- `security import` gets `-T /usr/bin/codesign -T /usr/bin/security` rather than
  a blanket `-A`, and `--ci` additionally sets the key partition list so
  codesign does not block on an interactive prompt. That call needs the keychain
  password, so it only works on a keychain applesign created; on the login
  keychain the prompt is approved by hand, once.

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
- **`apple_profile`**: every configurable attribute is `RequiresReplace`, `name` included — Apple rejects `PATCH /v1/profiles` with 403 `does not allow 'UPDATE'`, so a rename is a reissue and `Update` exists only to report that it was reached. Profile content/UUID/state/dates are Apple-computed. `CreateProfile` retries a 5xx up to three times: `POST /v1/profiles` intermittently answers 500 for a well-formed request that succeeds on the next attempt. It looks the name up before each retry and adopts an already-issued profile, so a 500 that arrives after Apple committed the write does not duplicate it.

## Local testing

The provider serves `registry.terraform.io/ahmedosman00/apple` (set in `main.go`; lower case because Terraform normalizes source addresses, while the Registry displays the namespace as `AhmedOsman00`). `terraform init` in `examples/` therefore downloads the last released version — to exercise the working tree instead, wire a locally built binary up through a `dev_overrides` block in `~/.terraformrc`, or a filesystem mirror if `init` needs to keep working. Run with `-debug` to attach a debugger.

The examples pin `version = "~> 0.1"`, and `scripts/validate-examples.sh` builds the working tree into a mirror at version `0.1.0` to satisfy that constraint — bump `PROVIDER_VERSION` there if the pin ever moves past it.

## Releasing

Pushing a `v*` tag runs `.github/workflows/release.yml`: GoReleaser cross-compiles the provider (only `main.go` — `cmd/applesign` is installed with `go install`, since the Registry matches every entry in `SHA256SUMS` against expected artifact names), signs the checksums with the GPG key in the `GPG_PRIVATE_KEY` / `PASSPHRASE` secrets, and attaches `terraform-registry-manifest.json`. `project_name` is pinned in `.goreleaser.yml` because the Registry requires `terraform-provider-apple_<version>_<os>_<arch>.zip`, and GoReleaser would otherwise take the name from the checkout directory. The Registry ingests new tags automatically once the repository is connected and the public GPG key is uploaded; update `CHANGELOG.md` before tagging.

## Tests and CI

Two tiers:

- **Credential-free** (`internal/apple/pagination_test.go`, `internal/provider/bundle/capability_models_test.go`, `internal/provider/certificate/renewal_test.go`, `internal/provider/device/models_test.go`): `httptest`-backed client tests and pure-function tests. `apple.Client` has all-exported fields, so pointing one at a test server needs no production seam — `&apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}`.
- **Acceptance** (`internal/provider/*_test.go`, package `provider`): `terraform-plugin-testing` against the real API. `testAccPreCheck` skips when credentials are absent.

All seven resources now have acceptance coverage. Checks go through `testAccAPIClient()` (provider_test.go), which builds a client from the same environment variables the provider reads: a `CheckDestroy` that only inspects Terraform state passes even when the resource is still live in the portal, so every existence and destroy check asks Apple. `testAccCheckDestroyAll` composes the checks for configurations that create several kinds of resource; the bundle ID checks predate this and still assert nothing.

Two things constrain what the acceptance tier is allowed to do:

- **Devices are permanent.** Apple has no device delete, so `device_resource_test.go` registers two fake UDIDs that stay in the team forever and consume device slots. Nothing new should register devices — which is why the profile tests use `IOS_APP_STORE` (needs no devices) rather than a development or ad hoc type. The UDIDs are hardcoded and `Delete` only disables, so a second run of those tests hits Apple's 409 and fails on `Device Already Exists`: they pass once per team, ever. CI skips them for that reason; run them by hand when you have accepted the slots.
- **Certificates consume a slot while they exist.** `certificate_resource_test.go` and both profile test files issue one and revoke it on destroy, so a completed run is neutral; an interrupted run leaves a certificate to revoke by hand. `IOS_DEVELOPMENT` is preferred over a distribution type where the profile type allows it.

Identifiers are fixed rather than randomised, matching the existing tests, so an interrupted run leaves a Bundle ID, Merchant ID or profile that must be deleted in the portal before the test passes again.

`ACCEPTANCE_TESTING.md` is the per-test reference: what every `TestAcc*` creates at Apple, where it appears in the portal, what it leaves behind, the cleanup list for an interrupted run, and a staged run order that defers the two irreversible device tests. It is hand-written and unverified — update the matching table when you add or rename an acceptance test.

One trap it records: Bundle ID identifiers cannot contain the underscore the platform constants carry (`BundleIdentifierValidator` is `^[a-zA-Z0-9.-]+\.[a-zA-Z0-9.-]+$`), and Apple's App ID *name* accepts only alphanumerics and spaces. `testAccBundleIDPlatformFixture` does both substitutions for `TestAccBundleIDResource_platforms`; interpolating a platform constant straight into either field fails.

`.github/workflows/test.yml` is the credential-free lane and runs four independent jobs: `build` (build + lint), `generate` (docs diff), `examples` (`make validate-examples`), and `unit` (`go test ./...`, where the acceptance tests skip themselves for want of `TF_ACC` and credentials). It triggers on every pull request and on pushes to `main` only — a push to a branch with an open PR would otherwise run it twice for the same commit — and cancels superseded runs.

Acceptance tests live in their own workflow, `.github/workflows/acceptance.yml`, and run **only on `workflow_dispatch`**: they create billable resources in a real Apple team, use fixed identifiers that collide if two runs overlap, and leave material to clean up by hand when interrupted. Its inputs are the Terraform version (one per run, not a matrix), a `-run` pattern, and an `include_device_tests` boolean that defaults to false — `TestAccDeviceResource*` registers two UDIDs permanently and passes once per team, ever. The job fails fast if the three `APPLE_APP_STORE_CONNECT_*` secrets are absent, because a suite that skips itself would otherwise report green on a run somebody triggered deliberately, and it sets `cancel-in-progress: false` so a queued run never kills one mid-apply.

All six filter packages have table-driven tests (`internal/provider/*/filters_test.go`) covering filtering, sorting, limiting, and the malformed-pattern errors. Resource CRUD paths are still only reachable through the acceptance tier.

Two behaviours worth knowing when writing filter tests, because they differ per package: `bundle` and `certificate` match `name_pattern` as a **glob** (`filepath.Match`, whole-string), while `device`, `merchant`, `passtypeid`, and `profile` match it as a **regex** (substring unless anchored). `merchant`'s display-name pattern is additionally case-insensitive.
