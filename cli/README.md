# apple

The actions that go with the [`ahmedosman00/apple`](https://registry.terraform.io/providers/ahmedosman00/apple)
Terraform provider. Terraform keeps the state of an app in App Store Connect: its
listing, versions, signing and TestFlight groups. `apple` does the steps a state
cannot describe:

| Command | What it does |
| --- | --- |
| `apple version next` | The next version and build number App Store Connect will accept |
| `apple version set` / `current` | Write or read `MARKETING_VERSION` and `CURRENT_PROJECT_VERSION` |
| `apple release preflight` | Check a release before anything changes: the version, a version stuck in review, release notes per locale |
| `apple release submit` | Submit a version for App Review once Terraform has attached its build |
| `apple build verify` | Check an archive or `.ipa`: bundle ID, version, build and signed entitlements |
| `apple build upload` | Upload an `.ipa` through the App Store Connect API, without `altool` or Xcode |
| `apple build wait` | Wait for an uploaded build to exist or finish processing, and set its "What to Test" |
| `apple testflight notes` | A "What to Test" note from git |
| `apple signing install` / `clean` / `encrypt` | Put the certificates and profiles Terraform issued where Xcode looks, with keys that never pass through Terraform |
| `apple screenshots capture` | App Store screenshots from a simulator with a clean status bar |
| `apple terraform export` | Terraform configuration and `import` blocks for what the account already holds, so the first plan changes nothing |

Building stays with `xcodebuild`: `apple` checks and uploads what it makes, and
never runs it.

## Install

```sh
brew install --cask ahmedosman00/tap/apple   # macOS
# or
go install github.com/AhmedOsman00/terraform-provider-apple/cmd/apple@latest
```

Release archives for macOS and Linux are on the
[releases page](https://github.com/AhmedOsman00/terraform-provider-apple/releases),
under `cli-v*` tags.

## Configuration

Credentials are the provider's:

```sh
export APPLE_APP_STORE_CONNECT_ISSUER_ID=...
export APPLE_APP_STORE_CONNECT_API_KEY=...
export APPLE_APP_STORE_CONNECT_PRIVATE_KEY="$(cat AuthKey_ABCD123456.p8)"
```

Every flag can also be set as `APPLE_<FLAG>`, so CI sets a value once:
`APPLE_BUNDLE_ID` for `--bundle-id`, `APPLE_PROJECT` for `--project`. There is no
configuration file; the bundle ID already lives in your Terraform.

Results go to stdout and progress to stderr. `--json` prints the result as JSON.
A usage error exits 2 and a failure 1.

## A release pipeline

```sh
export APPLE_BUNDLE_ID=com.example.app APPLE_PROJECT=App.xcodeproj

read -r version build < <(apple version next)
apple release preflight "$version" --notes-dir store/release-notes
apple version set "$version" "$build"

apple signing install --terraform terraform/ --key-dir signing --ephemeral-keychain
xcodebuild archive -archivePath build/App.xcarchive ...
apple build verify build/App.xcarchive --version "$version" --build "$build"
xcodebuild -exportArchive -archivePath build/App.xcarchive -exportPath build ...
apple signing clean
apple build verify build/App.ipa --version "$version" --build "$build" \
  --entitlement aps-environment=production
apple build upload build/App.ipa

apple testflight notes --header "Build $version ($build)" > notes.txt
apple build wait "$version" "$build" --what-to-test notes.txt
TF_VAR_app_version="$version" TF_VAR_app_build_number="$build" terraform apply
apple release submit "$version"
```

The [Releasing an app](https://registry.terraform.io/providers/ahmedosman00/apple/latest/docs/guides/releasing)
guide explains each step alongside the provider resources.

## Reference

One page per command, generated from the help: [`docs/`](docs/apple.md).
