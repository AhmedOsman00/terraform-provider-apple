## apple completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(apple completion bash)

To load completions for every new session, execute once:

#### Linux:

	apple completion bash > /etc/bash_completion.d/apple

#### macOS:

	apple completion bash > $(brew --prefix)/etc/bash_completion.d/apple

You will need to start a new shell for this setup to take effect.


```
apple completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple completion](apple_completion.md)	 - Generate the autocompletion script for the specified shell

