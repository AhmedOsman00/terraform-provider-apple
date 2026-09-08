---
page_title: "Code Signing"
subcategory: ""
description: |-
  Replace fastlane match with Terraform for the developer portal, keeping the
  signing key on your own machine.
---

# Code Signing

`fastlane match` solves a real problem: Apple caps how many distribution
certificates a team may hold, so certificates have to be shared rather than
minted per developer, and match shares them through an encrypted git repository
holding keys, certificates, and profiles together.

This provider covers the same ground with a different split of
responsibilities:

- **Terraform owns the developer portal** — the App ID, its capabilities, the
  devices, the certificates, and one provisioning profile per distribution
  method.
- **You own the private key.** It is generated on your machine and never reaches
  Terraform. Only a certificate signing request is configured, and a CSR is a
  public key plus a signature over it.

The consequence is worth stating plainly: **nothing Terraform stores is secret.**
A certificate and a provisioning profile are public documents, so the state, the
plan output, and every value the module exports are safe to print and safe to
log. The one secret in the workflow is a private key that Terraform never sees.

The repository ships a complete, runnable module at `examples/signing`. Read
[Getting Started](./getting-started.md) first if you have not set up credentials
yet.

## What the module manages

| Resource | Purpose |
|---|---|
| `apple_bundle_id.app` | The App ID |
| `apple_bundle_id_capability.app` | Settings-free capabilities from `var.capabilities` |
| `apple_device.team` | One registration per entry in `var.devices` |
| `apple_certificate.signing` | A certificate per role in `var.csr_contents`, issued from your CSR |
| `data.apple_certificates.adopted` | Existing certificates adopted by serial, read rather than issued |
| `apple_profile.development` / `.ad_hoc` / `.app_store` | One profile per distribution method |

Roles are `development` and `distribution`. Distribution signs everything that
leaves the machine and is required; development signs debug builds onto
registered devices and is optional. Development and Ad Hoc profiles are skipped
when `var.devices` is empty, which is the usual shape for a CI-only release
pipeline.

## Running it

### Generate a key and a CSR, once per role

```bash
openssl genrsa -out distribution.key 2048
openssl req -new -key distribution.key -out distribution.csr \
    -subj "/CN=My App distribution"
```

Apple ignores the CSR subject and issues under your team's own name, so `-subj`
is only there to stop `openssl` prompting.

~> **Keep `distribution.key`.** It is the half that signs, and Apple cannot
reissue it. Losing it means issuing a replacement certificate, and issuing one
revokes the certificate every existing build was signed with.

### Apply

```bash
cd examples/signing
cp terraform.tfvars.example terraform.tfvars   # then point csr_contents at your CSR

export APPLE_APP_STORE_CONNECT_ISSUER_ID=...
export APPLE_APP_STORE_CONNECT_API_KEY=...
export APPLE_APP_STORE_CONNECT_PRIVATE_KEY="$(cat AuthKey_ABCD123456.p8)"

terraform init
terraform apply
```

CSRs can come from the environment instead of a file, which suits CI:

```bash
export TF_VAR_csr_contents='{"distribution":"'"$(cat distribution.csr)"'"}'
```

### Install what it produced

Provisioning profiles are files named by UUID, in the directory Xcode reads:

```bash
mkdir -p ~/Library/MobileDevice/Provisioning\ Profiles

terraform output -json profiles \
  | jq -r '.[] | select(.uuid != "") | "\(.uuid)\t\(.profile_content)"' \
  | while IFS=$'\t' read -r uuid content; do
      printf '%s' "$content" | base64 -d \
        > ~/"Library/MobileDevice/Provisioning Profiles/$uuid.mobileprovision"
    done
```

The certificate has to be paired with your key into a PKCS#12 container before
the keychain will take it:

```bash
terraform output -json certificates \
  | jq -r '.distribution.certificate_content' | base64 -d \
  | /usr/bin/openssl x509 -inform DER -out distribution.cer.pem

/usr/bin/openssl pkcs12 -export -inkey distribution.key -in distribution.cer.pem \
    -out distribution.p12 -passout pass:temp

/usr/bin/security import distribution.p12 \
    -k ~/Library/Keychains/login.keychain-db -P temp -f pkcs12 \
    -T /usr/bin/codesign -T /usr/bin/security

rm -f distribution.cer.pem distribution.p12
```

`-T` grants the key to `codesign` and `security` specifically, rather than the
blanket `-A` that would let any process use it. macOS prompts once for access the
first time `codesign` uses the key; approve it and it will not ask again.

!> **Use `/usr/bin/openssl`, not whatever is first on `PATH`.** `security import`
rejects the PBES2/AES-256 PKCS#12 container OpenSSL 3 produces by default and
accepts only the legacy PBE-SHA1-RC2 one. macOS ships LibreSSL at
`/usr/bin/openssl`, which emits legacy by default and **does not accept a
`-legacy` flag**; Homebrew's OpenSSL 3 **requires** `-legacy` to emit it. No
single command line works on both, so pin the path.

Then set these in Xcode or on the `xcodebuild` command line:

```
PRODUCT_BUNDLE_IDENTIFIER      = com.example.myapp
CODE_SIGN_STYLE                = Manual
PROVISIONING_PROFILE_SPECIFIER = My App App Store
```

~> **The Apple WWDR intermediate must be present** for issued certificates to
chain to a trusted root; without it `codesign` reports an unhelpful "no identity
found". Xcode installs it, so most machines are fine — check with `security
find-certificate -c "Apple Worldwide Developer Relations"` and get it from
[Apple's certificate authority page](https://www.apple.com/certificateauthority/)
if it is missing.

### Without a key file at all

In **Keychain Access → Certificate Assistant → Request a Certificate From a
Certificate Authority**, choose "Saved to disk". The key is created inside your
login keychain and only the `.certSigningRequest` becomes a file.

Feed that CSR to `var.csr_contents` and install the issued certificate by
double-clicking it:

```bash
terraform output -json certificates \
  | jq -r '.distribution.certificate_content' | base64 -d > distribution.cer
open distribution.cer
```

The keychain pairs it with the key it already holds — no `.p12`, no `openssl`,
and the encoding trap above never arises. The cost is that it is interactive, so
CI still needs the file-based path.

### On a build runner

A runner wants a dedicated keychain rather than the login one, so that a build
cannot be blocked by an interactive prompt:

```bash
security create-keychain -p "$KEYCHAIN_PASSWORD" build.keychain
security set-keychain-settings -u build.keychain           # no auto-lock timeout
security unlock-keychain -p "$KEYCHAIN_PASSWORD" build.keychain
security list-keychains -d user -s build.keychain $(security list-keychains -d user | tr -d '"')
# ... import as above, with -k build.keychain ...
security set-key-partition-list -S apple-tool:,apple:,codesign: \
    -s -k "$KEYCHAIN_PASSWORD" build.keychain
```

`set-key-partition-list` is what stops `codesign` from raising the access prompt.
It needs the keychain password, which is why it only works on a keychain the job
created; on the login keychain the prompt is approved by hand, once.

## What has to be shared, and what does not

Apple caps how many distribution certificates a team may hold. That cap is the
entire reason match centralizes them, and it applies here unchanged.

!> **One Terraform state owns the certificates for a team.** Everyone else
consumes its outputs, or reads existing certificates through the
`apple_certificates` and `apple_profiles` data sources. Running `terraform apply`
on a personal copy of the module, per developer, will exhaust the limit.

What follows is a single sentence: the distribution **key** has to reach everyone
who signs releases.

|  | Needs protecting | How it travels |
|---|---|---|
| `distribution.key` | **Yes.** It is the only secret. | 1Password, sops, a CI secret |
| Terraform state | No secret material | any backend; locking still matters |
| `terraform output` values | No secret material | print them, log them, commit them |
| `distribution.csr` | No | commit it next to the module |

This is a much smaller problem than the one match solves. The key is about 1.7 KB,
it changes roughly once a year, and it is opaque — no JSON to diff, no state to
lock, and no risk that reading it reveals anything else about your
infrastructure. A single 1Password item or one sops-encrypted file is enough.

Development keys need none of this: Apple is far more permissive about
development certificates, so each developer can generate their own key and CSR
and share nothing.

**The state still deserves locking and versioning**, just not secrecy. Two people
applying at once can still orphan a certificate at Apple, and losing the state
still means reconciling by hand against the portal. Use
[GitLab managed state](https://docs.gitlab.com/user/infrastructure/iac/terraform_state/),
[HCP Terraform](https://developer.hashicorp.com/terraform/cloud-docs)'s free
tier, or `s3` with versioning and `use_lockfile`. What you no longer need is a
state whose read access is a company-wide security boundary.

## Migrating off match without reissuing

Pointing the module at a team that already uses match, and letting it issue its
own certificate, revokes the one match handed out — every build already signed
with it stops verifying. Importing the existing certificate does not help either:
`csr_content` forces replacement on `apple_certificate`, and Apple does not
reliably return the CSR a certificate was issued from, so an imported certificate
is replaced on the next apply, which revokes the original.

So adopt it instead. Give the module the serial number of the certificate match
issued:

```hcl
adopt_certificate_serials = {
  distribution = "6F1B2C3D4E5A7B8C"
}
```

A role named there is read through the `apple_certificates` data source rather
than created, so Terraform never revokes it, and it needs no CSR. Everything
downstream treats an adopted certificate exactly like an issued one. Roles you
leave out are issued from your CSR as usual, so adopting distribution while
Terraform issues development is a perfectly good half-way state.

The serial number is on the certificate in match's repository:

```bash
openssl x509 -in certs/distribution/ABCDE12345.cer -inform DER -noout -serial
```

Keep using the key match already holds for it, decrypted with the match
passphrase and stored wherever you decided the key lives:

```bash
openssl rsa -in certs/distribution/ABCDE12345.p12.key -out distribution.key
```

**Adoption is deliberately a one-way door held open.** `early_renewal_hours` does
not apply to an adopted certificate: renewing means issuing a replacement, and
issuing revokes the original. When the certificate nears expiry, or when the team
is ready to stop depending on match, generate a CSR, put it in `csr_contents`,
drop the role from `adopt_certificate_serials`, and let Terraform issue a fresh
one. That is the cutover — on a date you choose rather than the moment you first
ran `terraform apply`.

## Behaviour that differs from match

**Adding a device regenerates profiles automatically.** `devices` forces
replacement on `apple_profile`, so a new entry in `var.devices` produces a new
profile with a new UUID on the next apply. There is no equivalent of
`--force_for_new_devices` to remember.

**Certificates rotate before expiry, not at it.** `var.early_renewal_hours`
(default 30 days) makes a routine apply replace a certificate inside its renewal
window. The window is only evaluated when Terraform runs, so schedule a periodic
plan/apply if you rely on it. Replacement reuses the CSR you supplied, so the
renewed certificate covers the same key.

**`match nuke` is `terraform destroy`.** Deleting an `apple_certificate` revokes
it at Apple.

**Capabilities are applied before profiles are generated.** Profiles snapshot
entitlements at generation time, so the profile resources depend on the
capability resources. If you add a capability outside the module, regenerate the
profiles or they will carry stale entitlements.

## What the module does not cover

- Capabilities that take settings (`ICLOUD`, `APP_GROUPS`, `APPLE_PAY`,
  `ASSOCIATED_DOMAINS`) need nested `settings` blocks; declare them directly.
  See the `apple_bundle_id_capability` resource page.
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
- Key distribution. The module knows nothing about where your private key lives,
  which is what lets it impose no secret-store dependency.
- App Store Connect app records, TestFlight, and uploads. The module stops at
  code signing — `xcrun altool` or `fastlane deliver` still handle delivery.
