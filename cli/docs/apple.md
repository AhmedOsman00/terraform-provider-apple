## apple

App Store Connect actions to go with the Terraform provider

### Synopsis

apple performs the steps Terraform cannot model: working out the next version and build,
checking a release before it starts, waiting for a build, installing signing and taking
screenshots, and writing configuration for what an account already holds.
Terraform keeps the state; this does the actions.

Credentials are the provider's: APPLE_APP_STORE_CONNECT_ISSUER_ID,
APPLE_APP_STORE_CONNECT_API_KEY and APPLE_APP_STORE_CONNECT_PRIVATE_KEY.
Every flag can also be set as APPLE_<FLAG>, such as APPLE_BUNDLE_ID for --bundle-id.

### Options

```
  -h, --help   help for apple
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple build](apple_build.md)	 - Check, upload and follow a build through App Store Connect
* [apple completion](apple_completion.md)	 - Generate the autocompletion script for the specified shell
* [apple release](apple_release.md)	 - Check an App Store release before it starts, and submit it for review
* [apple screenshots](apple_screenshots.md)	 - Take App Store screenshots on a simulator
* [apple signing](apple_signing.md)	 - Install the certificates and profiles Terraform issued
* [apple terraform](apple_terraform.md)	 - Write Terraform configuration for what the account already holds
* [apple testflight](apple_testflight.md)	 - Write what TestFlight testers are told
* [apple version](apple_version.md)	 - Work out, read and set the marketing version and build number

