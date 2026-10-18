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
