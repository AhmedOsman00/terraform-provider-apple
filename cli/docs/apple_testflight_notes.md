## apple testflight notes

Print a "What to Test" note from git: a header, notes, then the commit subjects

### Synopsis

Prints a TestFlight "What to Test" note: --header, then --notes, then the subjects of
the commits since --since (or the last --limit), cut to Apple's 4000 characters. It reads
only git, so it runs anywhere. Hand the file to `apple build wait --what-to-test`.

```
apple testflight notes [flags]
```

### Options

```
      --header string   the first paragraph, such as which build and lane this is (env APPLE_HEADER)
  -h, --help            help for notes
      --limit int       at most this many commits (env APPLE_LIMIT) (default 20)
      --notes string    a paragraph under the header (env APPLE_NOTES)
      --repo string     the git repository to read (env APPLE_REPO) (default ".")
      --since string    list the commits after this one, up to --limit (env APPLE_SINCE)
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple testflight](apple_testflight.md)	 - Write what TestFlight testers are told

