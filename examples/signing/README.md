# Signing: a fastlane match replacement

This module manages everything `fastlane match` manages in the Apple developer
portal: the App ID and its capabilities, the registered devices, the signing
certificates, and one provisioning profile per distribution method.

**You generate the private key. Terraform never sees it.** The module takes a
certificate signing request, which is a public key plus a signature over it, and
hands back a certificate and a set of profiles. All three are public documents.
Nothing this configuration stores, prints, or outputs is secret.

That is the whole design. It is what makes the state ordinary, the outputs safe
to log, and the last mile something you can do with two commands.

## What it creates

| Resource | Purpose |
|---|---|
| `apple_bundle_id.app` | The App ID |
| `apple_bundle_id_capability.app` | Settings-free capabilities from `var.capabilities` |
| `apple_device.team` | One registration per entry in `var.devices` |
| `apple_certificate.signing` | A certificate per role in `var.csr_contents`, issued from your CSR |
| `data.apple_certificates.adopted` | Existing certificates named in `var.adopt_certificate_serials`, read rather than issued |
| `apple_profile.development` / `.ad_hoc` / `.app_store` | One profile per distribution method |

Roles are `development` and `distribution`. Distribution signs everything that
leaves the machine, both Ad Hoc and App Store, and is required. Development
signs debug builds onto registered devices and is optional.

Development and Ad Hoc profiles are skipped when `var.devices` is empty, which
is the usual shape for a CI-only release pipeline; the development profile is
skipped again if you declare no development role.

## Usage

### 1. Generate a key and a CSR, once per role

```bash
openssl genrsa -out distribution.key 2048
openssl req -new -key distribution.key -out distribution.csr \
    -subj "/CN=My App distribution"
```

Apple ignores the CSR subject and issues under your team's own name, so the
`-subj` is only there to keep `openssl` from prompting.

Keep `distribution.key`. It is the half that signs, it is the only secret in
this workflow, and Apple cannot reissue it — losing it means issuing a
replacement certificate, which revokes the one every existing build was signed
with.

There is a variant that never puts the key in a file at all; see
[Keychain Access instead of openssl](#keychain-access-instead-of-openssl).

### 2. Apply

```bash
cp terraform.tfvars.example terraform.tfvars   # then point csr_contents at your CSR

export APPLE_APP_STORE_CONNECT_ISSUER_ID=...
export APPLE_APP_STORE_CONNECT_API_KEY=...
export APPLE_APP_STORE_CONNECT_PRIVATE_KEY="$(cat AuthKey_XXXXXXXXXX.p8)"

terraform init
terraform apply
```

CSRs can come from the environment instead of a file, which is convenient in CI:

```bash
export TF_VAR_csr_contents='{"distribution":"'"$(cat distribution.csr)"'"}'
```

### 3. Install the result

```bash
terraform output -json profiles \
  | jq -r '.[] | select(.uuid != "") | "\(.uuid)\t\(.profile_content)"' \
  | while IFS=$'\t' read -r uuid content; do
      mkdir -p ~/Library/MobileDevice/Provisioning\ Profiles
      printf '%s' "$content" | base64 -d \
        > ~/"Library/MobileDevice/Provisioning Profiles/$uuid.mobileprovision"
    done

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

macOS prompts once for keychain access the first time `codesign` uses the new
key; approve it and it will not ask again.

Then set these in Xcode or on the `xcodebuild` command line:

```
PRODUCT_BUNDLE_IDENTIFIER      = com.example.myapp
CODE_SIGN_STYLE                = Manual
PROVISIONING_PROFILE_SPECIFIER = My App App Store
```

!> **Use `/usr/bin/openssl`, not whatever is first on `PATH`.** `security
import` rejects the PBES2/AES-256 PKCS#12 container that OpenSSL 3 produces by
default and accepts only the legacy PBE-SHA1-RC2 one. macOS ships LibreSSL at
`/usr/bin/openssl`, which emits legacy by default and **does not accept a
`-legacy` flag**; Homebrew's OpenSSL 3 **requires** `-legacy` to emit it. There
is no single command line that works on both, so pin the path.

The Apple WWDR intermediate must also be present or the identity will not chain
to a trusted root, and `codesign` reports that as an unhelpful "no identity
found". Xcode installs it, so most machines are fine:

```bash
security find-certificate -c "Apple Worldwide Developer Relations" >/dev/null \
  || echo "missing -- get it from https://www.apple.com/certificateauthority/"
```

### On a build runner

```yaml
- run: terraform -chdir=examples/signing init && terraform -chdir=examples/signing apply -auto-approve
- run: ./scripts/install-signing.sh     # the commands above, in a file
- run: xcodebuild -workspace MyApp.xcworkspace -scheme MyApp archive ...
```

A runner usually wants a dedicated keychain rather than the login one, so that a
build cannot be blocked by an interactive prompt:

```bash
security create-keychain -p "$KEYCHAIN_PASSWORD" build.keychain
security set-keychain-settings -u build.keychain           # no auto-lock timeout
security unlock-keychain -p "$KEYCHAIN_PASSWORD" build.keychain
security list-keychains -d user -s build.keychain $(security list-keychains -d user | tr -d '"')
# ... import as above, with -k build.keychain ...
security set-key-partition-list -S apple-tool:,apple:,codesign: \
    -s -k "$KEYCHAIN_PASSWORD" build.keychain
```

`set-key-partition-list` is the step that stops `codesign` from raising the
access prompt. It needs the keychain password, which is why it only works on a
keychain the job created.

## What has to be shared, and what does not

Apple caps how many distribution certificates a team may hold. That cap is the
entire reason match centralizes them instead of letting each developer mint their
own, and it applies here unchanged: **one Terraform state owns the certificates
for a team**, and a per-developer copy of this module will exhaust the limit.

What follows from that is a single sentence: the distribution **key** has to
reach everyone who signs releases.

|  | Needs protecting | How it travels |
|---|---|---|
| `distribution.key` | **Yes.** It is the only secret. | 1Password, sops, a CI secret |
| Terraform state | No secret material | any backend; locking still matters |
| `terraform output` values | No secret material | print them, log them, commit them |
| `distribution.csr` | No | commit it next to the module |

This is deliberately a much smaller problem than the one match solves. The key
is about 1.7 KB, it changes roughly once a year, and it is opaque — there is no
JSON to diff, no state to lock, and no risk that reading it tells you anything
about the rest of your infrastructure. A 1Password item or a single sops-encrypted
file is enough.

Development keys need none of this. Apple is far more permissive about
development certificates, so each developer can generate their own key, submit
their own CSR, and never share anything.

**The state still deserves locking and versioning**, just not secrecy. Two
people applying at once can still orphan a certificate at Apple, and losing the
state still means reconciling by hand against the portal. Use a real backend —
GitLab's [managed Terraform state](https://docs.gitlab.com/user/infrastructure/iac/terraform_state/),
[HCP Terraform](https://developer.hashicorp.com/terraform/cloud-docs)'s free
tier, or `s3` with versioning and `use_lockfile`. What you no longer need is a
dedicated state whose read access is a company-wide security boundary.

## Keychain Access instead of openssl

The strongest variant skips the key file entirely. In **Keychain Access →
Certificate Assistant → Request a Certificate From a Certificate Authority**,
choose "Saved to disk". The key is created inside your login keychain and only
the `.certSigningRequest` becomes a file.

Feed that CSR to `var.csr_contents`, then install the issued certificate by
double-clicking it:

```bash
terraform output -json certificates \
  | jq -r '.distribution.certificate_content' | base64 -d > distribution.cer
open distribution.cer
```

The keychain pairs it with the key it already holds. No `.p12`, no `openssl`,
and the PKCS#12 encoding trap above never comes up. The cost is that it is
interactive, so CI still needs the file-based path.

## Migrating off match without reissuing

Pointing this module at a team that already uses match, and letting it issue its
own certificate, revokes the one match handed out — every build already signed
with it stops verifying. Importing the existing certificate does not help
either: `csr_content` forces replacement on `apple_certificate`, and Apple does
not reliably return the CSR a certificate was issued from, so an imported
certificate is replaced on the next apply, which revokes the original.

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

Keep using the private key match already holds for it — decrypt it out of the
repository with the match passphrase and store it wherever you decided the key
lives:

```bash
openssl rsa -in certs/distribution/ABCDE12345.p12.key -out distribution.key
```

**Adoption is deliberately a one-way door held open.** `early_renewal_hours`
does not apply to an adopted certificate: renewing means issuing a replacement,
and issuing revokes the original. When the certificate nears expiry, or when the
team is ready to stop depending on match, generate a CSR, put it in
`csr_contents`, drop the role from `adopt_certificate_serials`, and let Terraform
issue a fresh one. That is the cutover — but it now happens on a date you choose
rather than the moment you first ran `terraform apply`.

## Behaviour that differs from match

**Adding a device regenerates profiles automatically.** `devices` forces
replacement on `apple_profile`, so a new entry in `var.devices` produces a new
profile with a new UUID on the next apply. There is no equivalent of
`--force_for_new_devices` to remember.

**Certificates rotate before expiry, not at it.** `var.early_renewal_hours`
(default 30 days) makes a routine apply replace a certificate inside its renewal
window. The window is only evaluated when Terraform runs, so schedule a periodic
plan/apply if you rely on it. Replacement reuses the CSR you supplied, so the new
certificate covers the same key and every machine that already installed it can
keep signing once the new certificate is installed.

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
- Key distribution. The module deliberately knows nothing about where your
  private key lives, which is what lets it impose no secret-store dependency.
- App Store Connect app records, TestFlight, and uploads. This module stops at
  code signing — `xcrun altool` or `fastlane deliver` still handle delivery.
