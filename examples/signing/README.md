# Signing: a fastlane match replacement

This module manages everything `fastlane match` manages in the Apple developer
portal, and the `applesign` command in this repository covers the part match
does on the local machine: assembling a `.p12`, importing it into a keychain,
and dropping provisioning profiles where Xcode looks for them.

Terraform owns the portal. `applesign` owns the machine. Nothing writes key
material to your working directory.

## What it creates

| Resource | Purpose |
|---|---|
| `apple_bundle_id.app` | The App ID |
| `apple_bundle_id_capability.app` | Settings-free capabilities from `var.capabilities` |
| `apple_device.team` | One registration per entry in `var.devices` |
| `tls_private_key.signing` + `tls_cert_request.signing` | Signing keys and CSRs, generated locally; a key given in `var.private_keys` is used instead |
| `apple_certificate.signing` | Development and distribution certificates, for roles this module issues |
| `data.apple_certificates.adopted` | Existing certificates named in `var.adopt_certificate_serials`, read rather than issued |
| `apple_profile.development` / `.ad_hoc` / `.app_store` | One profile per distribution method |

Development and Ad Hoc profiles are skipped when `var.devices` is empty, which
is the usual shape for a CI-only release pipeline. See "Migrating off match
without reissuing" for adopting certificates a team already has.

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars   # then edit it

export APPLE_APP_STORE_CONNECT_ISSUER_ID=...
export APPLE_APP_STORE_CONNECT_API_KEY=...
export APPLE_APP_STORE_CONNECT_PRIVATE_KEY="$(cat AuthKey_XXXXXXXXXX.p8)"

terraform init
terraform apply

go install ./cmd/applesign   # from the repository root, once

terraform output -json signing_bundle | applesign install -            # login keychain
terraform output -json signing_bundle | applesign install --ci -       # throwaway keychain
terraform output -json signing_bundle | applesign install --dry-run -  # show, install nothing
```

`make consume` from the repository root is the same pipe, without needing
`applesign` on `PATH`.

`applesign` reads the bundle as JSON on stdin and knows nothing about where that
JSON came from, so Terraform is only one possible source — see "Two artifacts,
distributed differently" below. It needs only `security`, which macOS ships, and installs only on
macOS; `--dry-run` inspects a bundle anywhere.

On a build runner:

```yaml
- run: terraform -chdir=examples/signing init && terraform -chdir=examples/signing apply -auto-approve
- run: terraform -chdir=examples/signing output -json signing_bundle | applesign install --ci -
- run: xcodebuild -workspace MyApp.xcworkspace -scheme MyApp archive ...
```

`--ci` creates a dedicated keychain, unlocks it for the session without an
auto-lock timeout, prepends it to the search list, and sets the key partition
list so `codesign` does not block on an interactive prompt. On the login
keychain that prompt appears once and you approve it by hand — `applesign`
cannot set the partition list there, because doing so needs your login
password.

## Two artifacts, distributed differently

Applying this module produces two things that look alike and are not: the
**Terraform state** and the **signing bundle**. They have different audiences
and different homes, and conflating them is how a signing setup ends up either
unusable or over-shared.

|  | Terraform state | Signing bundle |
|---|---|---|
| Who needs it | one person, or one CI job | everyone who signs |
| What it is | the resource graph, private keys included | a JSON document |
| Where it lives | a backend with locking and versioning | anywhere files live, encrypted |
| What reading it grants | rotation and revocation | the ability to sign |

### The state: one owner, a real backend

`tls_private_key` keeps its key in state in plaintext, and only one state can
own a team's certificates — Apple caps how many you may hold. That makes the
state the crown jewel, and exactly one person or automation should touch it.
This is the same secret match protects with a passphrase over an encrypted git
repository; the trust model changes rather than disappears, and your backend is
now what protects it.

Give the signing configuration **its own dedicated state**, separate from your
application infrastructure. Then "can read this state" means precisely "can sign
and revoke for this team", which is a permission worth granting deliberately.
Mixed into a state that half the team plans against, it is not.

**Turn on versioning.** Losing this state means issuing a replacement
certificate, and issuing one revokes the old certificate — every build signed
with it stops verifying. Versioning is the difference between an annoyance and a
day spent re-signing releases.

**There is no `git` backend, and there cannot be a sane one.** State needs
locking and atomic updates; git offers neither. Two people applying at once
against a state file in a repository get a merge conflict over a JSON blob
describing private keys, and whoever loses has a certificate orphaned at Apple.
What the hosts do offer instead:

- **GitLab** has [managed Terraform state](https://docs.gitlab.com/user/infrastructure/iac/terraform_state/):
  the `http` backend, with locking, in every tier. Nothing extra to run.
- **GitHub** has no equivalent. [HCP Terraform](https://developer.hashicorp.com/terraform/cloud-docs)'s
  free tier is the zero-friction choice — it locks, versions, and encrypts, and
  there is no object storage to learn. Use `s3` with SSE, bucket versioning and
  locking (`use_lockfile`, or a DynamoDB table before Terraform 1.10) if you are
  already an AWS shop.

A local `terraform.tfstate` on a laptop is not a home for a team's distribution
identity.

### The bundle: files, so it travels like files

The bundle is just JSON, and `applesign` reads it on stdin precisely so that it
can arrive from anywhere:

```bash
sops -d signing/bundle.enc.json | applesign install -
op read "op://Eng/ios-signing/bundle" | applesign install -
terraform output -json signing_bundle | applesign install -
```

Only the last of those needs the state. A consumer needs no backend credentials,
no Terraform, and no provider binary — only the bundle and `applesign`.

**Encrypted in git is match's model done properly, and it is the default worth
reaching for.** Developers already have the repository, and the bundle is
versioned next to the code it signs. Use [sops](https://github.com/getsops/sops)
with [age](https://github.com/FiloSottile/age) keys rather than match's single
shared passphrase: everyone gets their own key, so removing someone means
re-encrypting to the remaining keys. With a shared passphrase, revoking access
means rotating the secret and redistributing it to everybody, which is why in
practice it never happens.

```bash
# Whoever owns the state, after an apply:
terraform output -json signing_bundle \
  | sops -e --input-type json --output-type json /dev/stdin > signing/bundle.enc.json
git commit -m "Rotate signing bundle" signing/bundle.enc.json

# Everyone else, whenever it changes:
sops -d signing/bundle.enc.json | applesign install -
```

Note that the plaintext never lands in a file: it goes from Terraform to sops
through a pipe, and only ciphertext is written.

**GitHub Actions secrets work for CI and not for laptops.** They cannot be read
back outside a workflow run, so they will serve a runner and leave your
developers with nothing. Pick sops or a password manager for people, and use
Actions secrets for the runner only if you want a second copy.

### Publishing it to a secret store

The bundle is assembled in `bundle.tf` as `local.signing_bundle`, not inside the
output, because outputs cannot be referenced by resources in the same
configuration. Add your own file next to the module and wire it wherever your
team keeps secrets:

```hcl
# secret.tf -- yours, not part of the module
resource "aws_secretsmanager_secret_version" "signing" {
  secret_id     = aws_secretsmanager_secret.signing.id
  secret_string = jsonencode(local.signing_bundle)
}
```

```hcl
resource "vault_kv_secret_v2" "signing" {
  mount     = "secret"
  name      = "ios-signing"
  data_json = jsonencode(local.signing_bundle)
}
```

`jsonencode(local.signing_bundle)` is the same JSON `terraform output -json
signing_bundle` prints, so whatever the store hands back is what `applesign
install -` reads.

The module deliberately declares no secret resource of its own: doing so would
force a provider dependency — AWS, Vault, whichever — on every team, including
those that only ever pipe the output straight into `applesign`.

## Read this before pointing it at a real team

**Certificates are a scarce, shared resource.** Apple caps you at a small number
of distribution certificates per account. This is the entire reason match
centralizes them in one encrypted repo instead of letting each developer mint
their own. The same constraint applies here: **one Terraform state owns the
certificates for a team.** Everyone else consumes the outputs, or reads existing
certificates through the `apple_certificates` and `apple_profiles` data sources.
Running `terraform apply` on a personal copy of this module, per developer, will
exhaust the limit.

**Terraform state holds the private keys.** Restrict read access to it as
tightly as you would the match repository, and give it a backend that locks and
versions — see "Two artifacts, distributed differently" above.

**`signing_bundle` is a sensitive output.** Pipe it into `applesign`; do not
redirect it to a file you keep. `terraform output -json signing_bundle` prints
private keys.

**The Apple WWDR intermediate must be present** for issued certificates to chain
to a trusted root. Xcode installs it, so most machines are fine; `applesign`
warns if it is missing and tells you where to get it.

## Migrating off match without reissuing

Pointing this module at a team that already uses match, and letting it issue its
own certificates, revokes the ones match handed out — every build already signed
with them stops verifying. Importing the existing certificate does not help
either: `csr_content` forces replacement on `apple_certificate`, and Apple does
not reliably return the CSR a certificate was issued from, so an imported
certificate is replaced on the next apply, which revokes the original.

So adopt it instead. Give the module the serial number of the certificate match
issued and the private key match holds for it:

```hcl
private_keys = {
  distribution = file("${path.module}/match-distribution.key")
}

adopt_certificate_serials = {
  distribution = "6F1B2C3D4E5A7B8C"
}
```

A role named in `adopt_certificate_serials` is read through the
`apple_certificates` data source rather than created, so Terraform never revokes
it. Everything downstream — profiles, the signing bundle, `applesign` — treats
an adopted certificate exactly like an issued one. Roles you leave out are
issued as usual, so adopting distribution while Terraform issues development is
a perfectly good half-way state.

Both maps are keyed by role, `development` or `distribution`. Supplying a key
without adopting a certificate is also valid: the key is used for the CSR
instead of a generated one.

**Getting the two values out of match.** The serial number is on the certificate
in match's repository:

```bash
openssl x509 -in certs/distribution/ABCDE12345.cer -inform DER -noout -serial
```

and the key is next to it, decrypted with the match passphrase:

```bash
openssl rsa -in certs/distribution/ABCDE12345.p12.key -out match-distribution.key
```

Feed that file into `private_keys` and delete it once the bundle is stored;
after the first apply, the key lives in Terraform state.

**Adoption is deliberately a one-way door held open.** `early_renewal_hours`
does not apply to an adopted certificate: renewing means issuing a replacement,
and issuing revokes the original. When the certificate nears expiry, or when the
team is ready to stop depending on match, drop the role from
`adopt_certificate_serials` and let Terraform issue a fresh one. That is the
cutover — but it now happens on a date you choose rather than the moment you
first ran `terraform apply`.

## Behaviour that differs from match

**Adding a device regenerates profiles automatically.** `devices` forces
replacement on `apple_profile`, so a new entry in `var.devices` produces a new
profile with a new UUID on the next apply. There is no equivalent of
`--force_for_new_devices` to remember.

**Certificates rotate before expiry, not at it.** `var.early_renewal_hours`
(default 30 days) makes a routine apply replace a certificate inside its renewal
window. The window is only evaluated when Terraform runs, so schedule a periodic
plan/apply if you rely on it.

**`match nuke` is `terraform destroy`.** Deleting an `apple_certificate` revokes
it at Apple.

**Capabilities are applied before profiles are generated.** Profiles snapshot
entitlements at generation time, so the profile resources depend on the
capability resources. If you add a capability outside this module, regenerate the
profiles or they will carry stale entitlements.

## What is not covered

- Capabilities that take settings (`ICLOUD`, `APP_GROUPS`, `APPLE_PAY`,
  `ASSOCIATED_DOMAINS`) need nested `settings` blocks; declare them directly.
  See `examples/resources/apple_bundle_id_capability`.
- Non-iOS platforms. The module hardcodes `IOS`; macOS and tvOS need their own
  certificate and profile types.
- More than one app. The module manages a single bundle ID; a second app needs a
  second instance of it, with its own `profile_name_prefix`.
- Enterprise (`IOS_APP_INHOUSE`) and Developer ID (`MAC_APP_DIRECT`) profile
  types. The provider supports both — declare `apple_profile` directly.
- Profile-level early renewal. `var.early_renewal_hours` applies to certificates
  only. It does not need a profile equivalent: replacing a certificate changes
  its ID, and `apple_profile.certificates` forces replacement, so rotation
  already cascades to every profile that uses it.
- App Store Connect app records, TestFlight, and uploads. This module stops at
  code signing — `xcrun altool` or `fastlane deliver` still handle delivery.
