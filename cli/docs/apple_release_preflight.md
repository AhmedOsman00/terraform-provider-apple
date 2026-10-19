## apple release preflight

Check that App Store Connect will take VERSION, before anything is changed

### Synopsis

Checks, and reports every problem rather than the first:

  - VERSION has not been released, and is above the newest released version
  - no other version is in review, which would block the next one until it is
    withdrawn or released
  - release notes exist for every locale, at NOTES_DIR/VERSION/LOCALE.txt, within
    Apple's 4000 characters. Apple rejects notes on a first version and requires them on
    every one after, so a first version needs none.

The locales are the ones App Store Connect has for the app's latest version, unless
--locales names them. A version being prepared under another number is reported, not
refused: the provider renames it in place.

```
apple release preflight VERSION [flags]
```

### Options

```
      --bundle-id string   the app's bundle ID (env APPLE_BUNDLE_ID)
  -h, --help               help for preflight
      --locales strings    the locales that need notes, instead of the ones App Store Connect lists (env APPLE_LOCALES)
      --notes-dir string   the directory holding VERSION/LOCALE.txt release notes (env APPLE_NOTES_DIR)
      --platform string    IOS, MAC_OS, TV_OS or VISION_OS (env APPLE_PLATFORM) (default "IOS")
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple release](apple_release.md)	 - Check an App Store release before it starts, and submit it for review

