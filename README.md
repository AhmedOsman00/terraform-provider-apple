# Terraform Provider for Apple

A Terraform provider for the [Apple App Store Connect API](https://developer.apple.com/documentation/appstoreconnectapi),
built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

It manages the parts of the Apple Developer portal that a build pipeline depends
on — App IDs and their capabilities, signing certificates, registered devices,
Merchant IDs, Pass Type IDs, and provisioning profiles — as configuration rather
than as clicks in a web UI.

Together with the `applesign` CLI in this repository, it is a replacement for
`fastlane match`: Terraform owns the portal, `applesign` owns the local keychain.

## Contents

| | |
|---|---|
| [`docs/`](docs/) | Generated reference for every resource and data source |
| [`docs/guides/getting-started.md`](docs/guides/getting-started.md) | Credentials, installation, first configuration, importing |
| [`docs/guides/code-signing.md`](docs/guides/code-signing.md) | The `fastlane match` replacement, end to end |
| [`examples/signing/`](examples/signing/) | A runnable module that manages a full signing setup |
| [`cmd/applesign/`](cmd/applesign/) | CLI that installs a signing bundle into a keychain |

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- An Apple Developer Program team, and an App Store Connect API key with the
  **App Manager** role
- [Go](https://golang.org/doc/install) >= 1.25 — only to build the provider or
  `applesign` from source

## Installation

The provider is published on the Terraform Registry as
[`ahmedosman00/apple`](https://registry.terraform.io/providers/ahmedosman00/apple/latest),
so declaring it is enough — `terraform init` downloads it:

```terraform
terraform {
  required_providers {
    apple = {
      source  = "ahmedosman00/apple"
      version = "~> 0.1"
    }
  }
}
```

The `applesign` CLI is a separate binary and does not come through the Registry:

```shell
go install github.com/AhmedOsman00/terraform-provider-apple/cmd/applesign@latest
```

To run against an unreleased build of the provider, use a filesystem mirror or a
`dev_overrides` block; [the getting started guide](docs/guides/getting-started.md#installing-the-provider)
covers both, with the trade-off between them.

## Authentication

The provider mints a short-lived ES256 JWT from three values you get when
creating an App Store Connect API key under **Users and Access → Integrations →
App Store Connect API**:

| Argument | What it is | Environment variable |
|---|---|---|
| `issuer_id` | Team issuer ID (UUID) | `APPLE_APP_STORE_CONNECT_ISSUER_ID` |
| `api_key` | Key ID, 10 characters | `APPLE_APP_STORE_CONNECT_API_KEY` |
| `private_key` | Contents of the `.p8` file | `APPLE_APP_STORE_CONNECT_PRIVATE_KEY` |

Environment variables are read first; an explicit value in the `provider` block
overrides them. All three are sensitive and never appear in plan output.

```bash
export APPLE_APP_STORE_CONNECT_ISSUER_ID="69a6de70-1234-47e3-e053-5b8c7c11a4d1"
export APPLE_APP_STORE_CONNECT_API_KEY="ABCD123456"
export APPLE_APP_STORE_CONNECT_PRIVATE_KEY="$(cat AuthKey_ABCD123456.p8)"
```

Apple lets you download the `.p8` file once, and it is a team-wide credential —
anyone holding it can issue and revoke certificates for your team. Keep it in a
secret store, not in version control.

## Example usage

```terraform
terraform {
  required_providers {
    apple = {
      source  = "ahmedosman00/apple"
      version = "~> 0.1"
    }
  }
}

provider "apple" {
  # Reads APPLE_APP_STORE_CONNECT_* from the environment.
}

resource "apple_bundle_id" "app" {
  identifier = "com.example.myapp"
  name       = "My Example App"
  platform   = "IOS"
}

resource "apple_bundle_id_capability" "push" {
  bundle_id       = apple_bundle_id.app.id
  capability_type = "PUSH_NOTIFICATIONS"
}

resource "apple_device" "tester" {
  name     = "QA iPhone"
  udid     = "00008030-000A4D8E0AB8802E"
  platform = "IOS"
}

resource "apple_profile" "development" {
  name         = "My Example App Development"
  profile_type = "IOS_APP_DEVELOPMENT"
  bundle_id    = apple_bundle_id.app.id
  certificates = [apple_certificate.development.id]
  devices      = [apple_device.tester.id]

  # Profiles snapshot entitlements when Apple generates them.
  depends_on = [apple_bundle_id_capability.push]
}
```

## Resources and data sources

Every resource has a matching plural data source that lists the collection with
in-memory filtering, `sort_by`, `sort_order`, and `limit`.

| Resource | Data source | Manages |
|---|---|---|
| `apple_bundle_id` | `apple_bundle_ids` | App IDs |
| `apple_bundle_id_capability` | `apple_bundle_id_capabilities` | Capabilities on an App ID |
| `apple_certificate` | `apple_certificates` | Signing certificates |
| `apple_device` | `apple_devices` | Registered devices |
| `apple_merchant_id` | `apple_merchant_ids` | Apple Pay Merchant IDs |
| `apple_pass_type_id` | `apple_pass_type_ids` | Apple Wallet Pass Type IDs |
| `apple_profile` | `apple_profiles` | Provisioning profiles |

Behaviour worth knowing before you plan against a real team:

- **Destroying an `apple_certificate` revokes it at Apple**, and every build
  already signed with it stops verifying. Apple also caps how many distribution
  certificates a team may hold, so one state should own them.
- **Devices cannot be deleted through Apple's API.** Removing an `apple_device`
  disables it and drops it from state, with a warning.
- **`apple_profile.profile_type` selects the distribution method**, and
  `platform` is computed from it rather than set.
- **Do not import an `apple_certificate` you are still using.** `csr_content`
  forces replacement and Apple does not reliably return the original CSR, so the
  next apply reissues — and revokes the original. Read it through the
  `apple_certificates` data source instead.

## Replacing fastlane match

[`examples/signing/`](examples/signing/) is a complete, runnable module that
manages everything `fastlane match` manages in the developer portal — App ID,
capabilities, devices, signing certificates, and one provisioning profile per
distribution method. It emits a **signing bundle**: a JSON document holding the
keys, certificates, and profiles a machine needs.

`applesign` consumes that bundle on stdin and does the part match does locally —
assembling a `.p12`, importing it into a keychain, and installing profiles where
Xcode reads them:

```bash
terraform -chdir=examples/signing output -json signing_bundle | applesign install -       # login keychain
terraform -chdir=examples/signing output -json signing_bundle | applesign install --ci -  # throwaway keychain
```

`make consume` is the same pipe. Because the CLI knows about no secret store and
no backend, any source composes:

```bash
sops -d signing/bundle.enc.json | applesign install -
op read "op://Eng/ios-signing/bundle" | applesign install -
```

Read [the code signing guide](docs/guides/code-signing.md) before pointing this at
a real team. It covers why the Terraform state and the signing bundle are two
different artifacts with different homes, and how to migrate off match by
adopting its existing certificate instead of reissuing — which would revoke it.

## Development

```shell
make            # fmt + lint + install + generate
make build      # go build -v ./...
make test       # unit tests; no credentials needed
make testacc    # acceptance tests against the real API
make generate   # regenerate docs/ from schemas, examples/, and templates/
make validate-examples  # terraform validate over every directory under examples/
```

Run a single test with `go test -v ./internal/apple/ -run TestGetAllPages`.

`go test ./...` is safe without credentials — acceptance tests skip themselves
when the App Store Connect environment variables are absent. Acceptance tests hit
the real API and create billable resources.

[`ACCEPTANCE_TESTING.md`](ACCEPTANCE_TESTING.md) lists every acceptance test,
what it creates in your Apple Developer account, and what it leaves behind. Read
it before pointing the suite at a team you care about: two of the tests register
devices, which Apple cannot delete.

### Documentation

`docs/` is generated by `tfplugindocs` from the `MarkdownDescription` strings on
each schema plus the matching example under `examples/`, so edit those rather
than the generated Markdown. Hand-written guides live in `templates/guides/` and
render into `docs/guides/`. CI fails if `make generate` produces a diff, so
regenerate and commit whenever a schema, example, or guide changes.

`make generate` only runs `terraform fmt` over `examples/`, which cannot catch
configuration the provider schema rejects. `make validate-examples` builds the
provider into a throwaway filesystem mirror and runs `terraform validate` in
every example directory; that is the check that catches an example written
against a schema the provider does not have.

## License

[MPL-2.0](LICENSE).
