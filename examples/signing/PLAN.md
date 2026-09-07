# Plan: replacing fastlane match

Working plan for the signing work in this directory. Delete this file once the
four steps below have landed.

## Where things stand

Done and verified:

- `apple_profile` gained a required `profile_type` (`platform` is now computed).
  Creation previously sent `platform` where Apple requires `profileType`, so no
  profile could be created and the distribution method could not be expressed.
- To-many relationships (`certificates`, `devices`) were serialized as arrays of
  to-one identifiers; Apple requires `{"data":[...]}`. `devices` is now omitted
  rather than sent empty.
- `profile_content` is marked sensitive.
- `internal/apple/profiles_test.go` pins the create request body, credential-free.
- This module: App ID, capabilities, devices, keys/CSRs, certificates, and
  development / Ad Hoc / App Store profiles.
- **Step 1 below is done.** `cmd/applesign` installs certificates into a
  keychain (login or a throwaway `--ci` one) and profiles into both Xcode
  profile directories, reading the bundle on stdin.
  `scripts/install-signing.sh` is deleted and `make consume` is the pipe.
  Verified end to end: `security find-identity` reports the imported identity,
  so the legacy PKCS#12 encoding go-pkcs12 emits is one `security import`
  accepts.
- **Step 2 below is done.** The bundle is assembled in `bundle.tf` as
  `local.signing_bundle`; `outputs.tf` only exposes it. Verified with
  `terraform validate`: a `terraform_data` resource in a separate file consumes
  `jsonencode(local.signing_bundle)`, and the same reference to
  `output.signing_bundle` fails as "Reference to undeclared resource".
- **Step 3 below is done.** README section "Two artifacts, distributed
  differently": dedicated versioned state with a locking backend, why no git
  backend exists, GitLab managed state and HCP Terraform per host, and the
  sops+age encrypted-in-git flow as the default consumer path.

- **Step 4 below is done.** `var.private_keys` supplies an existing key per
  role, and `var.adopt_certificate_serials` reads an existing certificate
  through the data source instead of issuing one. Verified against a harness
  that exercises the real branching with the `tls` provider: defaults generate
  and issue both roles; a supplied key skips generation for that role only;
  adoption skips key, CSR and certificate for that role; adopting without a key
  fails the output precondition; both variable validations reject an unknown
  role.

All four steps have landed. What remains below is the open-issues list, which is
the only reason this file still exists.

## Steps

### 1. `cmd/applesign`, replacing the shell script -- DONE

A Go CLI in this repo, built by the existing `go install`. `applesign install -`
reads the signing bundle as JSON on **stdin**. Delete
`scripts/install-signing.sh`. Add a `make consume` target.

As built: `bundle.go` (bundle JSON), `identity.go` (decode, key/certificate
match check, PKCS#12), `keychain.go` (`/usr/bin/security`), `profile.go`
(profile directories), `install.go` (orchestration). Everything except the
`security` calls is unit-tested. `pkcs12.LegacyRC2` from
`software.sslmate.com/src/go-pkcs12` is the encoder the keychain accepts;
`pkcs12.Modern` is what OpenSSL 3 emits by default and `security import`
rejects it.

Stdin is the load-bearing design decision: the CLI must know about no secret
store and no backend, so any source composes as a pipe.

Go buys three things over the bash version: unit-testable p12 assembly, no `jq`
or `openssl` dependency (Go emits PKCS#12 legacy encoding natively, which is what
the macOS keychain wants), and no LibreSSL-vs-OpenSSL-3 divergence — that bit the
script twice, in subject parsing and in `-legacy` detection.

It still shells out to `/usr/bin/security` for the keychain import; there is no
cgo-free Go binding for the Security framework.

Port from the script, which has the working logic: base64 DER decode, DER to PEM,
PKCS#12 assembly, `security import` with `-T` ACLs, `set-key-partition-list` for
CI keychains, profile install to both directories, and the Apple WWDR
intermediate check.

### 2. Store-agnostic bundle -- DONE

`outputs.tf` builds the bundle inline inside the `output` block, and outputs are
not referenceable by resources in the same config. Move it to
`local.signing_bundle` so a user can wire it to any secret resource
(`aws_secretsmanager_secret_version`, `vault_kv_secret_v2`, …) without editing
the module.

As built: `bundle.tf` holds `local.all_profiles` and `local.signing_bundle`, and
the module still declares no secret resource of its own -- that would force an
AWS or Vault provider dependency on teams that only pipe the output into
applesign.

### 3. README: the distribution model -- DONE

The core idea: **Terraform state and the signing bundle are two different
artifacts, distributed differently.**

- **State** goes in a real backend, touched by exactly one person or CI job.
  There is no `git`/`github`/`gitlab` backend and there cannot be a sane one —
  state needs locking and atomic updates, git offers neither. GitLab teams can
  use GitLab managed Terraform state (the `http` backend). GitHub teams have no
  equivalent; HCP Terraform's free tier is the zero-friction pick over `s3` for
  a team that does not want to learn object storage.
- **The bundle** is files, so it distributes however the team likes — including
  an encrypted blob committed to a git repo, which is match's model done
  properly. `sops` with `age` keys beats match's shared passphrase: per-person
  keys mean revoking access does not require rotating a shared secret.

  ```
  sops -d signing/bundle.enc.json | applesign install -
  op read "op://Eng/ios-signing/bundle" | applesign install -
  terraform output -json signing_bundle | applesign install -
  ```

Consumers therefore need no backend credentials, no Terraform, and no provider
binary. Note that GitHub Actions secrets are write-only outside workflows, so
they work for CI but not for laptops.

Also document: give the signing config its **own dedicated state** (so "can read
this state" is exactly "can read the signing material"), and turn on versioning —
losing this state means reissuing the certificate, which revokes the old one and
breaks existing builds.

### 4. Optional private key variable -- DONE

`tls_private_key` always generates, and `apple_certificate` import reconstructs
`csr_content` from Apple's read response, which current API versions do not
reliably populate. Since `csr_content` forces replacement, an imported
certificate gets reissued and the old one revoked.

So migrating off match currently requires a planned cutover. Accepting the
private key as an optional variable lets a team feed in the key match already
holds and adopt the existing certificate instead.

As built, the key alone is not enough. Import still reissues, for exactly the
reason above, so adoption reads the existing certificate through
`data.apple_certificates` (filtered by serial number) rather than importing it,
and `local.certificates` merges issued and adopted certificates into one shape
so profiles and the bundle cannot tell them apart.

One trap worth remembering: `for_each` rejects anything derived from a sensitive
value, and `var.private_keys` must be sensitive. The role names are unwrapped
once, in `local.supplied_key_roles`, via `nonsensitive(keys(var.private_keys))`
-- the keys themselves stay sensitive.

## Known unrelated issue -- FIXED

An uncommitted change had replaced the committed
`theaostudio.com/hashicorp/apple` with `aostudio.com/apple`, which is not a
valid provider source address: it has two parts where Terraform requires
`hostname/namespace/type`. `terraform init` failed with `Invalid provider
namespace`, and the provider binary refused to start at all (`unable to validate
Address`), so nothing in `examples/` could run.

The address is now `aostudio.com/aostudio/apple` in `main.go:53`, every example,
`README.md`, `docs/index.md`, and `CLAUDE.md`. Verified: the binary serves, and
`examples/signing` inits and validates under a matching `dev_overrides`.

## Known unrelated issue: the examples do not validate -- FIXED

All seventeen directories under `examples/` now pass `terraform validate`. The
eight that failed did so in four distinct ways:

- **`settings { ... }` blocks** -- `resources/apple_bundle_id`,
  `resources/apple_bundle_id_capability`, `resources/apple_merchant_id`, and
  `data-sources/apple_merchant_ids`. The schema is a `ListNestedAttribute`, so
  it is `settings = [{ ... }]`.
- **UDIDs that fail the validator** -- `resources/apple_device`. The validator
  was wrong, not the examples: it accepted only 40-hex or a UUID and rejected
  the 8-16 hex form (`00008030-000A4D8E0AB8802E`) that every A12 and later
  device reports, which `terraform.tfvars.example` in this directory uses.
  Rather than add the third shape, `device.UDIDValidator` was loosened to a
  charset and length check. Pinning the shape bought a plan-time error over an
  apply-time one -- `Create` already turns Apple's 400 into an "Invalid Device
  Data" diagnostic -- and cost a provider release whenever the guess went
  stale. `internal/provider/device/models_test.go` pins all three shapes, plus
  the fact that an unrecognized shape is Apple's to reject, not ours.
- **Attributes the schema does not have** -- `data-sources/apple_bundle_ids`
  used `identifier_contains` / `name_contains` (now `identifier_prefix` and a
  `name_pattern` glob); `data-sources/apple_devices` filtered on `WATCH_OS`,
  which is not a device platform (watches register under `IOS`);
  `resources/apple_bundle_id` set the read-only `seed_id`;
  `resources/apple_bundle_id_capability` used `BACKGROUND_MODES`, which is an
  entitlement rather than a Bundle ID capability.
- **References that do not resolve** -- `data-sources/apple_merchant_ids`
  referenced an undeclared input variable, and `resources/apple_pass_type_id`
  called `file()` on a path that is not there. Both now declare the variable
  they use.

`make generate` only runs `terraform fmt`, so none of this was reachable from
CI. `make validate-examples` (`scripts/validate-examples.sh`) builds the
provider into a throwaway filesystem mirror, generates a `required_providers`
block for the directories that have none, and runs `terraform init` +
`terraform validate` in each. The `Validate Examples` job in
`.github/workflows/test.yml` runs it. `dev_overrides` is deliberately not used:
it makes `terraform init` refuse to run, and these directories need init.

## Out of scope for now

Multi-app and non-iOS platforms (the module hardcodes one bundle ID and `IOS`),
Enterprise and Developer ID profile types, and profile-level early renewal.
Certificate rotation already cascades to profiles, because replacing a
certificate changes its ID and `apple_profile.certificates` forces replacement.
