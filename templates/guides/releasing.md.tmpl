---
page_title: "Releasing an App"
subcategory: ""
description: |-
  Ship a version to the App Store with the provider holding the listing and
  the apple CLI doing the steps a state cannot describe.
---

# Releasing an App

A release is part state, part action. The listing, the version record, its
release notes and the attached build are all **state**, and the provider manages
them. Working out the next version number, uploading a binary, waiting for Apple
to process it and installing the signing identity are **actions**: they happen
once, they have no record to converge on, and a resource for any of them would
replay it on every apply.

The [`apple` CLI](https://github.com/AhmedOsman00/terraform-provider-apple/tree/main/cli)
does those actions. It uses the provider's App Store Connect client and the same
three credentials, and touches nothing Terraform manages.

```sh
brew install --cask ahmedosman00/tap/apple
# or
go install github.com/AhmedOsman00/terraform-provider-apple/cmd/apple@latest
```

## The configuration

The release-specific values come from the environment, so one configuration
describes every release:

```terraform
data "apple_apps" "app" {
  bundle_id = "com.example.app"
}

variable "app_version" {
  type = string
}

variable "app_build_number" {
  type    = string
  default = null
}

resource "apple_app_store_version" "app" {
  app_id         = data.apple_apps.app.apps[0].id
  platform       = "IOS"
  version_string = var.app_version
  build_number   = var.app_build_number
  release_type   = "MANUAL"
}

resource "apple_app_store_version_localization" "app" {
  for_each = toset(["en-US"])

  app_store_version_id = apple_app_store_version.app.id
  locale               = each.key
  description          = file("${path.module}/store/description/${each.key}.txt")
  whats_new = (
    fileexists("${path.module}/store/release-notes/${var.app_version}/${each.key}.txt")
    ? trimspace(file("${path.module}/store/release-notes/${var.app_version}/${each.key}.txt"))
    : null
  )
}
```

`whats_new` is null when there is no file because Apple **rejects** release
notes on a first version and **requires** them on every version after.

## The steps

Set the values every command shares once:

```sh
export APPLE_BUNDLE_ID=com.example.app
export APPLE_PROJECT=App.xcodeproj
```

### 1. Work out the version, and check it

```sh
read -r version build < <(apple version next)
apple release preflight "$version" --notes-dir store/release-notes
```

`version next` takes the minor after the newest released version (add
`--yearly-major-from 2026` to take a new major each calendar year) and the build
after the highest one uploaded. `release preflight` stops before anything changes
when the version is not above the newest release, when another version is still
in review, or when a locale App Store Connect lists has no release notes.

### 2. Write it into the project

```sh
apple version set "$version" "$build"
git commit -am "Release $version ($build)"
```

### 3. Cut the version

```sh
TF_VAR_app_version="$version" terraform apply
```

When the version in state has been released, changing `version_string` replaces
it: the shipped version is dropped from state and left untouched at Apple, and
the next one is created. Its localizations, screenshot sets and review detail
are replaced with it, adopting the copies Apple starts every new version with, so
only what changed is sent. A version still being prepared is renamed in place
instead. There is no `terraform state rm` step.

### 4. Build, sign and upload

```sh
apple signing install --from <(terraform output -json signing) --key-dir signing --ephemeral-keychain
xcodebuild archive -project App.xcodeproj -scheme App -archivePath build/App.xcarchive
apple build verify build/App.xcarchive --version "$version" --build "$build"
xcodebuild -exportArchive -archivePath build/App.xcarchive -exportOptionsPlist ExportOptions.plist \
  -exportPath build
apple signing clean
apple build verify build/App.ipa --version "$version" --build "$build" \
  --entitlement aps-environment=production
apple build upload build/App.ipa
```

`signing install` reads the certificates and profiles from a Terraform output
(see [Code Signing](./code-signing.md) for the module), and the private keys from
files, the environment, or `.key.enc` files encrypted with `apple signing
encrypt`. `build verify` checks that the archive and the exported `.ipa` carry
the version and build just set and were signed with the entitlements a release
needs — a push entitlement still on `development` is the usual catch — and
reports every mismatch at once. `build upload` sends the `.ipa` through App Store
Connect's build upload API with the same API key, so the export no longer has
to: leave `destination` at `export` in `ExportOptions.plist`. Exporting with
`destination` set to `upload` still works if you prefer it.

### 5. Wait for processing

```sh
apple testflight notes --header "Build $version ($build)" > notes.txt
apple build wait "$version" "$build" --what-to-test notes.txt
```

Apple processes a build for five to thirty minutes, and a version refuses one
that is not `VALID`. `build wait` returns once it is, and stops at once if Apple
marks the build `INVALID`.

### 6. Attach the build

```sh
TF_VAR_app_version="$version" TF_VAR_app_build_number="$build" terraform apply
```

Submitting for review stays in App Store Connect. Submission is an event, not a
state: a resource for it would cancel a live review on destroy and resubmit on
apply.

## In CI

Every command prints its result on stdout and its progress on stderr, takes
`--json`, and exits 2 on a usage error and 1 on a failure, so each step is one
line in a workflow. Every flag also reads `APPLE_<FLAG>`; set `APPLE_BUNDLE_ID` at
the workflow level and no step repeats it.
