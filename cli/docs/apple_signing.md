## apple signing

Install the certificates and profiles Terraform issued

### Synopsis

The provider issues certificates and provisioning profiles; these commands put them
where Xcode and codesign look. The private keys never pass through Terraform: they are
read here, from a file, the environment, or an encrypted file in the repository.

### Options

```
  -h, --help   help for signing
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple](apple.md)	 - App Store Connect actions to go with the Terraform provider
* [apple signing clean](apple_signing_clean.md)	 - Remove the keychain --ephemeral-keychain created
* [apple signing encrypt](apple_signing_encrypt.md)	 - Encrypt every DIR/<role>.key into a <role>.key.enc that can be committed
* [apple signing install](apple_signing_install.md)	 - Install provisioning profiles and signing identities from a signing document

