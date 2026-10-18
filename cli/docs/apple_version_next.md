## apple version next

Print the next version and build number App Store Connect will accept

### Synopsis

Prints "VERSION BUILD" on stdout, and what it found on stderr. Nothing is changed.

Without VERSION it is the minor after the newest released version: 1.2.0 is followed by
1.3.0. With --yearly-major-from, the first release of a calendar year takes the next
major instead, the years since that one plus one. Before anything is released it is
the project's MARKETING_VERSION. A VERSION given is refused if it has been released, or
is not above the newest released version.

The build is the highest build number already uploaded for that version plus one, or 1.

```
apple version next [VERSION] [flags]
```

### Options

```
      --bundle-id string        the app's bundle ID (env APPLE_BUNDLE_ID)
  -h, --help                    help for next
      --keep-version            keep the project's version and only work out the next build, for a TestFlight build (env APPLE_KEEP_VERSION)
      --platform string         IOS, MAC_OS, TV_OS or VISION_OS (env APPLE_PLATFORM) (default "IOS")
      --project string          the .xcodeproj, read for --keep-version and before the first release (env APPLE_PROJECT)
      --yearly-major-from int   the launch year; the first release of each later year takes the next major (env APPLE_YEARLY_MAJOR_FROM)
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple version](apple_version.md)	 - Work out, read and set the marketing version and build number

