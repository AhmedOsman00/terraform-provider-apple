## Unreleased

FEATURES:

* **`apple terraform export`.** Writes Terraform configuration for what an App
  Store Connect account already holds: a resource block per record carrying
  Apple's values, an `import` block beside each, and `EXPORT_NOTES.md` listing
  what was left out and why. The first `terraform plan` should then import
  everything and change nothing, which is what copying values by hand cannot
  promise.

  ```sh
  apple terraform export --out ./apple --app com.example.app
  ```

  Certificates are read through `apple_certificates` data sources rather than
  imported, because importing one reissues it and revokes the original.
  Screenshots are left out, since Apple keeps their bytes rather than the
  files, and demo account passwords become sensitive variables. `--only`
  limits it to `signing`, `app`, `products`, `testflight` or `team`, and
  `--json` prints the summary and notes as JSON. It only reads, and refuses an
  `--out` that already holds files unless `--force` is given.

## 0.2.0 (October 19, 2026)

FEATURES:

* **`apple build verify`.** Checks an `.xcarchive`, `.ipa` or `.app` against the
  bundle ID, version and build it should carry, and against `--entitlement
  KEY=VALUE` for what it was signed with, reporting every mismatch at once.
  The Info.plist is read in Go, so those checks run on any runner; entitlements
  come from `codesign` and need macOS. Out of an `.ipa` only the executable is
  extracted, and only when an entitlement is checked.

* **`apple build upload`.** Uploads an `.ipa` through App Store Connect's build
  upload API (API 4.1): no `altool`, Transporter or Xcode, just the API key.
  The version, build and bundle ID come from the `.ipa` itself. It waits for
  Apple to accept or refuse the upload, prints Apple's warnings and errors, and
  removes an upload it could not deliver. Follow it with `apple build wait`.
  The file is streamed part by part, never held in memory whole.

* **`apple release submit`.** Submits a version for App Review through the
  review submission API, after checking that the version can be submitted and
  that its build is attached and `VALID`. An open draft submission is reused,
  and one App Review rejected is resubmitted rather than replaced. Apple's
  every objection to the version is printed, not only the first. A version
  already waiting for or in review is reported and left alone, so a pipeline can
  rerun it.

## 0.1.0 (October 18, 2026)

FEATURES:

* **`apple version next`, `set` and `current`.** The next version is the minor
  after the newest released one, or the next major for the first release of a
  year with `--yearly-major-from`. The next build is the highest uploaded for
  that version plus one. `set` writes both onto every target of an Xcode
  project.

* **`apple release preflight`.** Checks a version before a release changes
  anything, and reports every problem at once: the version must be above the
  newest released one, no other version may be in review, and every locale
  App Store Connect lists needs release notes after the first version.

* **`apple build wait`.** Polls until an uploaded build exists or is `VALID`,
  stops at once on `FAILED` or `INVALID`, and sets the TestFlight "What to Test"
  note, retrying after processing when Apple refuses earlier. It renews its
  token, so a wait can outlast the provider's 20-minute one.

* **`apple testflight notes`.** A "What to Test" note from a header, notes and
  the commit subjects, cut to Apple's 4000 characters.

* **`apple signing install`, `clean` and `encrypt`.** Installs the profiles and
  certificates a Terraform output carries, pairing each certificate with a key
  from a flag, the environment, or an encrypted file in the repository.
  `--ephemeral-keychain` sets up a keychain for a CI runner, including Apple's
  WWDR intermediate. Encrypted keys use the `openssl enc -aes-256-cbc -pbkdf2
  -iter 600000` format, so files encrypted by hand still open.

* **`apple screenshots capture`.** Boots a simulator, pins the status bar to
  9:41 and writes a JPEG per screen as you drive the app.
