## apple completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	apple completion fish | source

To load completions for every new session, execute once:

	apple completion fish > ~/.config/fish/completions/apple.fish

You will need to start a new shell for this setup to take effect.


```
apple completion fish [flags]
```

### Options

```
  -h, --help[=true|false]              help for fish
      --no-descriptions[=true|false]   disable completion descriptions
```

### Options inherited from parent commands

```
      --json[=true|false]   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple completion](apple_completion.md)	 - Generate the autocompletion script for the specified shell

