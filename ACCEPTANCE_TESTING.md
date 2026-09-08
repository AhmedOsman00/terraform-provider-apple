# Acceptance testing against a real Apple account

Every acceptance test in `internal/provider` talks to the live App Store Connect
API. This file records what each one creates in your Apple Developer account,
where it shows up in the portal, and what it leaves behind.

Read [Devices](#devices--the-only-irreversible-tests) before you export
credentials. It is the only section describing changes you cannot undo.

The credential-free tests (`internal/apple`, the `filters_test.go` files,
`capability_models_test.go`, `renewal_test.go`, `models_test.go`) never reach
Apple and are not covered here.

## Is any of this going to get the account suspended?

No. The suite does none of the things Apple acts on: no credential
brute-forcing, no scraping, no bulk account creation, no circumvention. A full
run is on the order of a few hundred API calls spread over many minutes, well
under the rate ceiling, and `doRequest` reports a 401 rather than retrying, so
there is no retry storm to mistake for an attack.

There is one clause worth knowing about: the device tests register fabricated
UDIDs, and the Developer Program License Agreement frames device registration as
being for devices you own or control. In practice that is a paperwork concern
rather than an enforcement one — but it is the only place the suite touches the
agreement, and the only permanent change it makes.

Run the suite against a dedicated or secondary team where you can. Not because
of suspension, but because device slots and certificate slots are finite and
shared with whatever ships from that team.

## What a clean full run costs

Assuming every test passes and every destroy completes:

| | Count | |
|---|---|---|
| **Permanent** | 2 | Device registrations that can never be removed — one iOS, one Mac. Terraform disables them; they still occupy slots until the membership year renews. |
| **Transient peak** | 2 | Certificates alive at once, at most. Tests run sequentially and each revokes what it issued. |
| **Net zero** | 5 | Resource kinds fully cleaned up: Bundle IDs, capabilities, Merchant IDs, Pass Type IDs, profiles. |
| **Touch nothing** | 19 | Tests rejected at plan time or read-only. |

### Certificates come back; devices do not

This is the one distinction that matters when reading the tables below.

For **certificates**, the cap is on how many are active *at once*, not how many
you have ever issued. `Delete` calls `DELETE /v1/certificates/{id}`, which is
Apple's revoke endpoint (`internal/apple/certificates.go`). A revoked
certificate drops off the list immediately and the slot is free that instant.
There is no lifetime counter. `testAccCheckCertificateDestroy` asserts this by
checking the certificate is no longer *listed* rather than GETting it by ID.

For **devices**, the cap is on how many you have ever registered in the
membership year. Apple has no delete at all, so the provider disables instead
(`PATCH … status: DISABLED`). Disabling releases the device from provisioning
profiles but does **not** give the slot back. The count resets only at annual
renewal.

Note also that revoking a certificate invalidates every provisioning profile
built against it — signed ad hoc and enterprise builds already in the field stop
launching, though App Store builds are fine since Apple re-signs those. The
suite only ever revokes certificates it issued itself, so your own certificates
and their profiles are untouched. It is still worth internalising for normal use
of the provider: `terraform destroy` on an `apple_certificate` is a revocation,
not a soft delete.

## Where each thing appears in the portal

Everything lands under **Certificates, Identifiers & Profiles** on
developer.apple.com. Nothing appears in App Store Connect itself — no apps, no
builds, no submissions.

| Resource | Portal location |
|---|---|
| Bundle IDs | Identifiers → filter "App IDs" |
| Capabilities | Identifiers → open the App ID → Capabilities checklist |
| Merchant IDs | Identifiers → filter "Merchant IDs" |
| Pass Type IDs | Identifiers → filter "Pass Type IDs" |
| Certificates | Certificates (revoked ones drop off the list) |
| Devices | Devices → toggle "Include disabled devices" |
| Profiles | Profiles |

## Devices — the only irreversible tests

Neither device test declares a `CheckDestroy`, and both use hard-coded UDIDs.

| Test | Residue | Creates at Apple | What you see in the portal |
|---|---|---|---|
| `TestAccDeviceResource` | **Permanent** | Registers iOS UDID `00008030-000A4D8E0AB8802E` as "Test Device", imports it, renames it to "Updated Test Device", then disables it. | **Devices** gains a permanent iPhone row named *Updated Test Device*, status *Disabled*. One iOS slot consumed for the membership year. |
| `TestAccDeviceResourceMacOS` | **Permanent** | Registers Mac UDID `550e8400-e29b-41d4-a716-446655440000` as "Test Mac", then disables it. | **Devices** gains a permanent Mac row named *Test Mac*, status *Disabled*. One Mac slot consumed. |
| `TestAccDevicesDataSource` | Read-only | Nothing. Lists every device in the team. | No change. |
| `TestAccDevicesDataSourceFiltered` | Read-only | Nothing. Lists devices, filters to `IOS` in memory. | No change. |
| `TestAccDevicesDataSourceWithLimit` | Read-only | Nothing. Lists devices, takes 5. | No change. |

The two resource tests are effectively one-shot. A second run tries to register
the same UDIDs and the provider maps Apple's 409 to *"Device Already Exists"*,
so expect them to fail on every run after the first unless you re-enable and
reuse the existing entries. Skip them with `-skip 'TestAccDeviceResource'`.

## Certificates

CSRs are generated in Go per test (`testAccCertificateCSR`), so no key material
touches your keychain. A run killed between create and destroy leaves a live
certificate you must revoke by hand.

| Test | Residue | Creates at Apple | What you see in the portal |
|---|---|---|---|
| `TestAccCertificateResource_basic` | Slot, returned | Issues 1 × `IOS_DEVELOPMENT`; imports it twice (by serial, by display name); revokes. | One Apple Development certificate mid-run, gone at the end. |
| `TestAccCertificateResource_earlyRenewal` | Slot, returned | Issues 1 × `IOS_DEVELOPMENT`, updates `early_renewal_hours` in place, then sets it to 99999 h — wider than a certificate's one-year life — forcing a reissue. 2 issued in sequence, 2 revoked. | Two Apple Development certificates appear and disappear one after the other, with different serials. |
| `TestAccCertificateResource_requiresReplace` | Slot, returned | Issues `IOS_DEVELOPMENT`, then replaces it with `IOS_DISTRIBUTION`. Both revoked. | One Development then one Distribution certificate. The only certificate test that consumes a *distribution* slot. |
| `TestAccCertificateResource_validation` | Plan-only | Nothing. Bad type, bad CSR and negative hours are all rejected by validators. | No change. |
| `TestAccCertificatesDataSource_basic` | Slot, returned | Issues 1 × `IOS_DEVELOPMENT`, reads it back through four data sources, revokes. | One Apple Development certificate appears and disappears. |
| `TestAccCertificatesDataSource_validation` | Plan-only | Nothing. | No change. |

Across the whole suite (certificate plus profile tests) roughly nine
certificates are issued and revoked, but never more than two are alive at once —
no test calls `t.Parallel()`. If the team is already at its
distribution-certificate cap, the profile tests fail to create rather than
evicting anything.

## Bundle IDs

`testAccCheckBundleIDDestroy` asserts nothing — it is a stub — so a failed
delete would go unnoticed by the test.

| Test | Residue | Creates at Apple | What you see in the portal |
|---|---|---|---|
| `TestAccBundleIDResource_basic` | Net zero | `com.test.terraform-example` (iOS), renamed "Test Example" → "Updated Example". | One App ID appears, its name changes, then it is deleted. |
| `TestAccBundleIDResource_platforms` | Net zero | Three subtests: `com.test.platform-ios`, `-mac-os`, `-universal`. | Three App IDs appear and are deleted, one per subtest. |
| `TestAccBundleIDResource_validation` | Plan-only | Nothing — bad identifier, empty name, bad platform all fail validation. | No change. |
| `TestAccBundleIDResource_requiresReplace` | Net zero | `com.test.original`, then replaced by `com.test.changed`. | Two App IDs appear in sequence, both deleted. |
| `TestAccBundleIDResource_disappears` | Net zero | `com.test.terraform-disappear`, deleted at Apple behind Terraform's back to force drift. | One App ID appears and is deleted mid-test; the destroy finds it already gone. |
| `TestAccBundleIDsDataSource_basic` | Net zero | `com.test.datasource`, plus an unfiltered list of every Bundle ID in the team. | One App ID appears and is deleted. |
| `TestAccBundleIDsDataSource_empty` | Read-only | Nothing. Lists every Bundle ID. | No change. |
| `TestAccBundleIDsDataSource_filtering` | Net zero | `com.test.filter.ios` (iOS) and `com.test.filter.macos` (macOS). | Two App IDs appear and are deleted. |
| `TestAccBundleIDsDataSource_sorting` | Net zero | `com.test.sort.aaa` and `com.test.sort.zzz`. | Two App IDs appear and are deleted. |
| `TestAccBundleIDsDataSource_limit` | Read-only | Nothing. Lists Bundle IDs, takes 2. | No change. |
| `TestAccBundleIDsDataSource_validation` | Plan-only | Nothing. | No change. |

`_platforms` derives its fixtures through `testAccBundleIDPlatformFixture`
rather than interpolating the platform constant directly. `BundleIdentifierValidator`
is `^[a-zA-Z0-9.-]+\.[a-zA-Z0-9.-]+$`, so `com.test.MAC_OS` would fail at plan
time, and Apple's App ID name accepts only alphanumerics and spaces, so the name
has to lose the underscore too. Keep both substitutions if you touch that test.

The subtests cover `IOS`, `MAC_OS` and `UNIVERSAL`, which is everything Apple
accepts: `TV_OS` and `WATCH_OS` are rejected with a 409 naming the three valid
values, and tvOS and watchOS App IDs are created as `UNIVERSAL`. Apple then
stores *every* Bundle ID as `UNIVERSAL` whatever was sent, so `platform` records
what was configured and import ignores it.

## Bundle ID capabilities

Each test creates its own parent Bundle ID. Both are deletable, so a completed
run leaves nothing.

| Test | Residue | Creates at Apple | What you see in the portal |
|---|---|---|---|
| `TestAccBundleIDCapabilityResource_basic` | Net zero | `com.test.terraform-capability` + Push Notifications; then imports it as `<bundle>/<capability>`. | An App ID appears with *Push Notifications* ticked, then both are deleted. |
| `TestAccBundleIDCapabilityResource_importBareID` | Net zero | Same Bundle ID + Push Notifications, imported by bare capability ID. | Same as above. `bundle_id` is recovered from the capability ID, which Apple composes as `<bundleID>_<TYPE>`, and confirmed against the parent before it is written, so the import is complete rather than partial. |
| `TestAccBundleIDCapabilityResource_settings` | Net zero | Same Bundle ID + Data Protection, flipping the level from *Complete Protection* to *Protected Until First User Auth* in place. | The App ID's *Data Protection* capability shows a changed protection level mid-run. |
| `TestAccBundleIDCapabilityResource_requiresReplace` | Net zero | Same Bundle ID; Push Notifications replaced by HealthKit. | The capability tick moves from *Push Notifications* to *HealthKit*. |
| `TestAccBundleIDCapabilityResource_validation` | Plan-only | Nothing. | No change. |
| `TestAccBundleIDCapabilitiesDataSource_basic` | Net zero | `com.test.terraform-capability-ds` + Push Notifications + HealthKit; reads back via four data sources. | An App ID with two capabilities ticked, then deleted. |
| `TestAccBundleIDCapabilitiesDataSource_validation` | Plan-only | Nothing. | No change. |

Two things about capabilities are worth knowing before reading a failure here.
Apple exposes no `GET` for a single capability — it answers 403 `does not allow
'GET_INSTANCE'` — so every read goes through the parent Bundle ID's collection,
and that collection rejects a `limit` parameter. And Apple applies a capability
write as a read-modify-write over the whole set without locking, so two
concurrent creates on one Bundle ID both return 201 while only one survives; the
provider serializes capability writes per Bundle ID to prevent it.

Apple also enables some capabilities on a new Bundle ID by itself
(`IN_APP_PURCHASE` among them), which is why the data source test asserts a
floor on the count rather than an exact number.

## Merchant IDs

Deletable. Apple rejects duplicates, so leftovers block re-runs.

| Test | Residue | Creates at Apple | What you see in the portal |
|---|---|---|---|
| `TestAccMerchantIDResource_basic` | Net zero | `merchant.com.test.terraform`, renamed "Terraform Test Merchant" → "Renamed Test Merchant". | A Merchant ID appears, its display name changes, then it is deleted. |
| `TestAccMerchantIDResource_importByAppleID` | Net zero | `merchant.com.test.terraform-import`. | One Merchant ID appears and is deleted. |
| `TestAccMerchantIDResource_requiresReplace` | Net zero | `merchant.com.test.terraform-before`, replaced by `…-after`. | Two Merchant IDs appear in sequence, both deleted. |
| `TestAccMerchantIDResource_validation` | Plan-only | Nothing. | No change. |
| `TestAccMerchantIDsDataSource_basic` | Net zero | `merchant.com.test.terraform-ds`. | One Merchant ID appears and is deleted. |
| `TestAccMerchantIDsDataSource_filtering` | Net zero | `…terraform-ds-alpha` and `…terraform-ds-beta`. | Two Merchant IDs appear and are deleted. |
| `TestAccMerchantIDsDataSource_validation` | Plan-only | Nothing. | No change. |

## Pass Type IDs

`testAccCheckPassTypeIDDestroy` only inspects Terraform state, so it cannot
catch a Pass Type ID that survived at Apple — verify in the portal after a run.

| Test | Residue | Creates at Apple | What you see in the portal |
|---|---|---|---|
| `TestAccPassTypeIDResource_basic` | Net zero | `pass.com.test.terraform-example`, renamed "Test Example Pass" → "Updated Example Pass". | A Pass Type ID appears, its name changes, then it is deleted. |
| `TestAccPassTypeIDResource_importByIdentifier` | Net zero | `pass.com.test.terraform-import`. | One Pass Type ID appears and is deleted. |
| `TestAccPassTypeIDResource_invalidIdentifier` | Plan-only | Nothing — identifier must start with `pass.`. | No change. |
| `TestAccPassTypeIDResource_invalidName` | Plan-only | Nothing. | No change. |
| `TestAccPassTypeIDsDataSource_basic` | Read-only | Nothing. Lists every Pass Type ID. | No change. |
| `TestAccPassTypeIDsDataSource_withFilters` | Read-only | Nothing. | No change. |
| `TestAccPassTypeIDsDataSource_withSorting` | Read-only | Nothing. | No change. |
| `TestAccPassTypeIDsDataSource_withLimit` | Read-only | Nothing. | No change. |

## Provisioning profiles

The heaviest tests: each builds a Bundle ID, an `IOS_DISTRIBUTION` certificate
and a profile. `IOS_APP_STORE` is deliberate — development and ad hoc profiles
require devices, and devices are permanent.

| Test | Residue | Creates at Apple | What you see in the portal |
|---|---|---|---|
| `TestAccProfileResource_basic` | Slot, returned | `com.test.terraform-profile` + 1 distribution certificate + profile "Terraform Test Profile"; imported by ID and by name. | An App ID, a Distribution certificate and an *App Store* profile all appear, then all three are removed. |
| `TestAccProfileResource_rename` | Slot, returned | Same trio; profile renamed "Terraform Rename Before" → "Terraform Rename After", which reissues it. | The first profile disappears and a second appears under the new name. Apple rejects `PATCH /v1/profiles` with 403 `does not allow 'UPDATE'`, so `name` is `RequiresReplace`. |
| `TestAccProfileResource_requiresReplace` | Slot, returned | Two Bundle IDs (`…-first`, `…-second`) + 1 certificate; the profile is moved between them, forcing a recreate. | Two App IDs, one certificate, and a profile that is deleted and reissued against the second App ID. |
| `TestAccProfileResource_validation` | Plan-only | Nothing. | No change. |
| `TestAccProfilesDataSource_basic` | Slot, returned | `com.test.terraform-profile-ds` + 1 certificate + profile "Terraform DS Profile"; read back through five data sources. | An App ID, a Distribution certificate and a profile appear, then all are removed. |
| `TestAccProfilesDataSource_validation` | Plan-only | Nothing. | No change. |

## Provider

| Test | Residue | Creates at Apple | What you see in the portal |
|---|---|---|---|
| `TestAccProvider` | No calls | Applies an empty config. The JWT is minted locally; no request is sent. | No change. |
| `TestAccProvider_ConfigurationValidation` | No calls | Nothing — three deliberately malformed credential sets, all rejected by validators before any request. Runs even without credentials. | No change. |

## If a run is interrupted

Identifiers are fixed rather than randomised, so leftovers block the next run.
Delete these in the portal before retrying:

```
Identifiers → App IDs
  com.test.terraform-example         com.test.original
  com.test.changed                   com.test.disappear
  com.test.datasource                com.test.filter.ios
  com.test.filter.macos              com.test.sort.aaa
  com.test.sort.zzz                  com.test.platform-ios
  com.test.platform-mac-os           com.test.platform-tv-os
  com.test.platform-watch-os         com.test.terraform-capability
  com.test.terraform-capability-ds   com.test.terraform-profile
  com.test.terraform-profile-first   com.test.terraform-profile-second
  com.test.terraform-profile-ds

Identifiers → Merchant IDs
  merchant.com.test.terraform          merchant.com.test.terraform-import
  merchant.com.test.terraform-before   merchant.com.test.terraform-after
  merchant.com.test.terraform-ds       merchant.com.test.terraform-ds-alpha
  merchant.com.test.terraform-ds-beta

Identifiers → Pass Type IDs
  pass.com.test.terraform-example      pass.com.test.terraform-import

Profiles
  Terraform Test Profile               Terraform Rename Before / After
  Terraform Replace Profile            Terraform DS Profile

Certificates
  Any certificate whose CSR common name is
  "terraform-provider-apple acceptance test"  → revoke it
```

## How to run it

These tests never run automatically. In CI they are a manual
`workflow_dispatch` of **Acceptance Tests** (`.github/workflows/acceptance.yml`),
which takes a Terraform version, a `-run` pattern, and an `include_device_tests`
toggle. Locally, export the three credentials the provider reads, then start
with the tests that leave nothing behind before committing the device slots.

```bash
export APPLE_APP_STORE_CONNECT_ISSUER_ID=...
export APPLE_APP_STORE_CONNECT_API_KEY=...
export APPLE_APP_STORE_CONNECT_PRIVATE_KEY="$(cat AuthKey_XXXXXXXXXX.p8)"

# 1. Read-only first — proves the credentials work, creates nothing.
TF_ACC=1 go test -v ./internal/provider/ -timeout 30m \
  -run 'TestAccProvider|DataSource_validation|DataSource_empty|DevicesDataSource'

# 2. Everything except the two permanent device tests.
TF_ACC=1 go test -v ./internal/provider/ -timeout 120m \
  -run 'TestAcc' -skip 'TestAccDeviceResource'

# 3. Only when you have accepted the permanent slots.
TF_ACC=1 go test -v ./internal/provider/ -timeout 30m -run 'TestAccDeviceResource'
```

Step 3 passes once per team and never again: the UDIDs are hardcoded and destroy
only disables the device, so a second run gets Apple's 409 back as
`Device Already Exists`. The CI equivalent is step 2 —
`.github/workflows/acceptance.yml` passes `-skip 'TestAccDeviceResource'` unless
its `include_device_tests` input is set.

`make testacc` runs everything, devices included. Prefer the staged commands
above for a first run against a new team.

Do not add `-parallel`: the suite uses fixed identifiers that would collide.
For the same reason CI runs one Terraform version per dispatch rather than a
matrix, and its `concurrency` group queues a second run instead of starting it
alongside the first.

If Go's test timeout kills a run mid-apply, Terraform never destroys — that is
the main way you end up with the leftovers listed above.

## Keeping this file honest

It is hand-written and nothing verifies it. When you add or rename an
acceptance test, update the matching table. The counts in
[What a clean full run costs](#what-a-clean-full-run-costs) come from the
per-section tables, so recompute them if a residue classification changes.
