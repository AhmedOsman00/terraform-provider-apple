## apple signing install

Install provisioning profiles and signing identities from a signing document

### Synopsis

Reads a signing document, which is the JSON of a Terraform output shaped like this:

  {
    "certificates": { "<role>": { "certificate_content": "<base64 DER>", "expiration_date": "..." } },
    "profiles": { "<name>": { "uuid": "...", "state": "ACTIVE", "expiration_date": "...",
                               "profile_content": "<base64 .mobileprovision>" } }
  }

profiles may also be a list of objects carrying "name". `terraform output -json <name>`
prints exactly that; --from - reads it from stdin.

--terraform DIR runs `terraform -chdir=DIR output -json signing` and reads that instead,
so no copy of the document is left on disk to go stale or be committed. Terraform does
the backend login, whatever the backend is. --terraform-output names another output.
--from stays for someone without access to the state, who can be sent the file.

Every profile is installed under its UUID, replacing older copies of the same profile.
Every certificate whose private key is found is imported as a signing identity. A role's
key is the first of:

  1. --key ROLE=PATH
  2. APPLE_SIGNING_<ROLE>_KEY, the PEM itself or base64 of it
  3. KEY_DIR/<role>.key
  4. KEY_DIR/<role>.key.enc, decrypted with APPLE_SIGNING_PASSPHRASE or a prompt

--ephemeral-keychain creates a keychain nothing else uses, adds Apple's WWDR
intermediate, imports into it and lets codesign use the keys without a prompt: what a
CI runner needs. `apple signing clean` removes it.

```
apple signing install (--from FILE | --terraform DIR) [flags]
```

### Options

```
      --certificates-only         skip the profiles (env APPLE_CERTIFICATES_ONLY)
      --check-expiry int          warn about a certificate expiring within DAYS, and stop on an expired one (env APPLE_CHECK_EXPIRY)
      --ephemeral-keychain        create a throwaway keychain for a CI runner (env APPLE_EPHEMERAL_KEYCHAIN)
      --from string               the signing document, or - for stdin (env APPLE_FROM)
  -h, --help                      help for install
      --key stringArray           ROLE=PATH, the private key for ROLE; repeatable (env APPLE_KEY)
      --key-dir string            a directory of <role>.key or <role>.key.enc files (env APPLE_KEY_DIR)
      --keychain string           the keychain to import into (default: login, or a temporary one with --ephemeral-keychain) (env APPLE_KEYCHAIN)
      --profile-dir stringArray   where to install profiles, repeatable (default: the two directories Xcode reads) (env APPLE_PROFILE_DIR)
      --profiles-only             skip the certificates (env APPLE_PROFILES_ONLY)
      --terraform string          the Terraform configuration whose output is the signing document (env APPLE_TERRAFORM)
      --terraform-output string   the output --terraform reads (env APPLE_TERRAFORM_OUTPUT) (default "signing")
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple signing](apple_signing.md)	 - Install the certificates and profiles Terraform issued

