## apple signing encrypt

Encrypt every DIR/<role>.key into a <role>.key.enc that can be committed

### Synopsis

Encrypts each private key with APPLE_SIGNING_PASSPHRASE, or a passphrase asked for twice;
an empty answer generates one. Existing .key.enc files must open with the same passphrase,
so a team keeps a single secret. The format is `openssl enc -aes-256-cbc -pbkdf2 -iter
600000 -a`, so OpenSSL can open the files too.

```
apple signing encrypt --key-dir DIR [flags]
```

### Options

```
  -h, --help             help for encrypt
      --key-dir string   the directory holding <role>.key (env APPLE_KEY_DIR)
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple signing](apple_signing.md)	 - Install the certificates and profiles Terraform issued

