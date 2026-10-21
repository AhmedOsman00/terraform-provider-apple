## apple terraform export

Write .tf files and import blocks from the live account, so the first plan changes nothing

### Synopsis

Reads the App Store Connect account and writes Terraform configuration for it into --out:
a resource block per record holding the value Apple holds, an import block beside each,
and EXPORT_NOTES.md listing what was left out and why. `terraform plan` in that directory
should then import everything and change nothing, which is the point: values copied by
hand are how a first apply overwrites a price or replaces a product.

Certificates are read through data sources rather than imported, since importing one
reissues it and revokes the original. Screenshots are left out, since Apple keeps their
bytes and not the files. Demo account passwords become sensitive variables.

--only takes signing, app, products, testflight and team. app, products and testflight are
per app: every app on the team, or those named by --app. Nothing is written to Apple, and
terraform is not run.

```
apple terraform export [flags]
```

### Options

```
      --app strings    an app to export, by App Store Connect ID or bundle identifier; repeat for several (default every app) (env APPLE_APP)
      --force          write into --out even when it already holds files, replacing any of the same name (env APPLE_FORCE)
  -h, --help           help for export
      --only strings   export only these parts: signing, app, products, testflight, team (default all) (env APPLE_ONLY)
      --out string     the directory to write into; it must be empty or not exist yet (env APPLE_OUT)
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple terraform](apple_terraform.md)	 - Write Terraform configuration for what the account already holds

