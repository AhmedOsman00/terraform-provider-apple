## apple build verify

Check that an archive or .ipa is the build you meant to make

### Synopsis

Reads the app inside PATH, an .xcarchive, an .ipa or an .app, and checks it against the
flags given, reporting every mismatch rather than the first:

  --bundle-id    CFBundleIdentifier
  --version      CFBundleShortVersionString
  --build        CFBundleVersion
  --entitlement  KEY=VALUE, an entitlement the app was signed with. A boolean matches
                 true or false, and a list matches when VALUE is one of its entries.
                 Repeat it for more.

Nothing is unzipped but the executable, and only for --entitlement. Entitlements are read
with codesign, so that check needs macOS; the others run anywhere.

```
apple build verify PATH [flags]
```

### Options

```
      --build string              the CFBundleVersion to expect (env APPLE_BUILD)
      --bundle-id string          the CFBundleIdentifier to expect (env APPLE_BUNDLE_ID)
      --entitlement stringArray   an entitlement to expect, as KEY=VALUE; repeatable (env APPLE_ENTITLEMENT)
  -h, --help                      help for verify
      --version string            the CFBundleShortVersionString to expect (env APPLE_VERSION)
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple build](apple_build.md)	 - Check, upload and follow a build through App Store Connect

