## apple build wait

Wait for an uploaded build to exist, or to finish processing

### Synopsis

Polls App Store Connect until the build exists (--until exists) or Apple has finished
processing it (--until valid, the default). A build has to be VALID before a version
can take it. Only the API is involved, so it runs on any runner.

--what-to-test sets the build's TestFlight "What to Test" from a file. If Apple refuses
while the build is still processing, it waits for VALID and tries again.

```
apple build wait VERSION BUILD [flags]
```

### Options

```
      --bundle-id string      the app's bundle ID (env APPLE_BUNDLE_ID)
  -h, --help                  help for wait
      --interval duration     how often to ask (env APPLE_INTERVAL) (default 1m0s)
      --locale string         the locale of the "What to Test" note (env APPLE_LOCALE) (default "en-US")
      --platform string       IOS, MAC_OS, TV_OS or VISION_OS (env APPLE_PLATFORM) (default "IOS")
      --timeout duration      give up after this long (env APPLE_TIMEOUT) (default 40m0s)
      --until string          exists or valid (env APPLE_UNTIL) (default "valid")
      --what-to-test string   a file to set as the build's TestFlight "What to Test" (env APPLE_WHAT_TO_TEST)
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple build](apple_build.md)	 - Follow an uploaded build through App Store Connect

