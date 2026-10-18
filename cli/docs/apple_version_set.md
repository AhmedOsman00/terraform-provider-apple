## apple version set

Write MARKETING_VERSION and CURRENT_PROJECT_VERSION on every target

### Synopsis

Writes the pair onto every build configuration in the project. It asks Apple nothing;
take the pair from `apple version next`.

```
apple version set VERSION BUILD [flags]
```

### Options

```
  -h, --help             help for set
      --project string   the .xcodeproj (env APPLE_PROJECT)
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple version](apple_version.md)	 - Work out, read and set the marketing version and build number

