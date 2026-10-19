## apple build upload

Upload an .ipa to App Store Connect

### Synopsis

Uploads PATH, an exported .ipa, through App Store Connect's build upload API: no altool,
Transporter or Xcode, so it runs on any runner with the API key. The version, build
number and bundle ID come from the app's Info.plist; --bundle-id, when given, has to
agree with it.

It waits until Apple has accepted the upload (COMPLETE) or refused it (FAILED), and
prints Apple's warnings either way. Processing the build comes after that: follow it
with apple build wait.

```
apple build upload PATH [flags]
```

### Options

```
      --bundle-id string    the app's bundle ID (env APPLE_BUNDLE_ID)
  -h, --help                help for upload
      --interval duration   how often to ask (env APPLE_INTERVAL) (default 15s)
      --platform string     IOS, MAC_OS, TV_OS or VISION_OS (env APPLE_PLATFORM) (default "IOS")
      --timeout duration    give up waiting for Apple's checks after this long (env APPLE_TIMEOUT) (default 20m0s)
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple build](apple_build.md)	 - Check, upload and follow a build through App Store Connect

