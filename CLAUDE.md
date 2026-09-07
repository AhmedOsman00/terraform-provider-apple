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
- `pagination.go`: `getAllPages[T]` walks every page of a collection, following the absolute `links.next` URL until it is empty and refusing links that point off-host. All seven `Get*s()` functions go through it — reading only the first page silently truncates results.
- Apple's API has no server-side filtering here — "get by identifier/name" helpers (e.g. `GetPassTypeIDByIdentifier`, `GetProfileByName`) list a full collection and scan client-side, so they depend on pagination being complete.

### `internal/provider/` — Terraform layer

`provider.go` defines the `apple` provider: credential schema with regex validators (issuer ID = UUID, api_key = 10 chars `[A-Z0-9]`, private_key = PEM block), reads env vars **first** and lets explicit config override them, builds the `*apple.Client`, and hands it to every resource/data source via `resp.ResourceData` / `resp.DataSourceData`.

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

- **`apple_certificate`**: immutable — every attribute describing the certificate is `RequiresReplace`, and Delete revokes it. `certificate_content` is `Sensitive`. Optional `early_renewal_hours` triggers replacement before expiry: `ModifyPlan` compares prior state's `expiration_date` against the window, flips the computed `ready_for_renewal` to true, marks the Apple-issued attributes unknown, and reports `ready_for_renewal` as the path forcing replacement. `Create`/`Read`/`ImportState` all record `ready_for_renewal` as false so the flip registers as a change. `Update` exists solely to accept a changed `early_renewal_hours` in place — it copies the issued certificate forward untouched.
- **`apple_device`**: the Apple API cannot delete devices, so `Delete` disables the device (`PATCH` with `status: DISABLED`) and lets Terraform drop it from state, emitting a warning. A 404 while disabling is treated as already-gone so destroy still succeeds.
- **`apple_bundle_id_capability`**: Apple's capability response omits the parent Bundle ID, so import accepts `<bundle_id>/<capability_id>` and verifies the pair before writing state. A bare capability ID still imports, but records `bundle_id` as null and warns that the next plan will force replacement.
- **`apple_profile`**: `bundle_id`, `certificates`, `devices`, and `platform` are all `RequiresReplace`; profile content/UUID/state/dates are Apple-computed.

## Local testing

The provider serves `registry.terraform.io/ahmedosman00/apple` (set in `main.go`; lower case because Terraform normalizes source addresses, while the Registry displays the namespace as `AhmedOsman00`). `terraform init` in `examples/` therefore downloads the last released version — to exercise the working tree instead, wire a locally built binary up through a `dev_overrides` block in `~/.terraformrc`, or a filesystem mirror if `init` needs to keep working. Run with `-debug` to attach a debugger.

The examples pin `version = "~> 0.1"`, and `scripts/validate-examples.sh` builds the working tree into a mirror at version `0.1.0` to satisfy that constraint — bump `PROVIDER_VERSION` there if the pin ever moves past it.

## Releasing

Pushing a `v*` tag runs `.github/workflows/release.yml`: GoReleaser cross-compiles the provider (only `main.go` — `cmd/applesign` is installed with `go install`, since the Registry matches every entry in `SHA256SUMS` against expected artifact names), signs the checksums with the GPG key in the `GPG_PRIVATE_KEY` / `PASSPHRASE` secrets, and attaches `terraform-registry-manifest.json`. `project_name` is pinned in `.goreleaser.yml` because the Registry requires `terraform-provider-apple_<version>_<os>_<arch>.zip`, and GoReleaser would otherwise take the name from the checkout directory. The Registry ingests new tags automatically once the repository is connected and the public GPG key is uploaded; update `CHANGELOG.md` before tagging.

## Tests and CI

Two tiers:

- **Credential-free** (`internal/apple/pagination_test.go`, `internal/provider/bundle/capability_models_test.go`, `internal/provider/certificate/renewal_test.go`, `internal/provider/device/models_test.go`): `httptest`-backed client tests and pure-function tests. `apple.Client` has all-exported fields, so pointing one at a test server needs no production seam — `&apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}`.
- **Acceptance** (`internal/provider/*_test.go`, package `provider`): `terraform-plugin-testing` against the real API. `testAccPreCheck` skips when credentials are absent.

`.github/workflows/test.yml` runs five jobs: `build` (build + lint), `generate` (docs diff), `examples` (`make validate-examples`), `unit` (always, no credentials), and `acceptance` (gated on a `check-credentials` output because secrets cannot be read from a job-level `if`; serialized with `max-parallel: 1` since the suite uses fixed identifiers that would collide across matrix entries).

All six filter packages have table-driven tests (`internal/provider/*/filters_test.go`) covering filtering, sorting, limiting, and the malformed-pattern errors. Resource CRUD paths are still only reachable through the acceptance tier.

Two behaviours worth knowing when writing filter tests, because they differ per package: `bundle` and `certificate` match `name_pattern` as a **glob** (`filepath.Match`, whole-string), while `device`, `merchant`, `passtypeid`, and `profile` match it as a **regex** (substring unless anchored). `merchant`'s display-name pattern is additionally case-insensitive.
