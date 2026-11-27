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

Without --app and --only, the whole account is exported. --app (by App Store Connect ID or
bundle identifier, repeatable) exports those apps' own parts: the listing, products and
TestFlight, the Bundle IDs under the app's own (com.acme.app and com.acme.app.*, except one
that is another app's), and the profiles issued for them. The certificates and devices those
profiles use are read through data sources, and the account's merchant IDs, pass type IDs,
users and invitations are left out unless --only names them. An explicit --only always wins:
--app com.acme.app --only users exports only users.

--list reads the account the same way and prints what would be written, by app and part,
without writing anything; --out is then not needed.

Parts (--only):
  bundle-ids     under --app: the named apps' own
                 apple_bundle_id, apple_bundle_id_capability
  certificates   under --app: only what their profiles use
                 data.apple_certificates
  devices        under --app: only what their profiles use, as data sources;
                             every device, as resources, when --only names it
                 apple_device, data.apple_devices
  merchant-ids   under --app: only when --only names it
                 apple_merchant_id
  pass-type-ids  under --app: only when --only names it
                 apple_pass_type_id
  profiles       under --app: the named apps' own
                 apple_profile
  listing        under --app: the named apps' own
                 apple_app_settings, apple_app_info, apple_app_info_localization,
                 apple_app_age_rating_declaration, apple_app_store_version,
                 apple_app_store_version_localization, apple_app_store_review_detail,
                 apple_app_price_schedule, apple_app_availability
  products       under --app: the named apps' own
                 apple_subscription_group, apple_subscription_group_localization,
                 apple_subscription, apple_subscription_localization,
                 apple_subscription_availability, apple_subscription_price_schedule,
                 apple_subscription_introductory_offer_schedule, apple_in_app_purchase,
                 apple_in_app_purchase_localization,
                 apple_in_app_purchase_price_schedule, apple_in_app_purchase_availability
  testflight     under --app: the named apps' own
                 apple_beta_group, apple_beta_tester, apple_beta_app_localization,
                 apple_beta_app_review_detail
  users          under --app: only when --only names it
                 apple_user
  invitations    under --app: only when --only names it
                 apple_user_invitation
  Aliases: signing = bundle-ids, profiles, certificates, devices, merchant-ids,
  pass-type-ids; team = users, invitations; app = listing.

Nothing is written to Apple, and terraform is not run.

```
apple terraform export [flags]
```

### Options

```
      --app strings          an app to export, by App Store Connect ID or bundle identifier; repeat for several (default every app, and the whole account) (env APPLE_APP)
      --force[=true|false]   write into --out even when it already holds files, replacing any of the same name (env APPLE_FORCE)
  -h, --help[=true|false]    help for export
      --list[=true|false]    print what would be written, by app and part, and write nothing (env APPLE_LIST)
      --only strings         export only these parts, or the aliases signing, team and app; see the list above (default all, or under --app every part that is not the account's) (env APPLE_ONLY)
      --out string           the directory to write into; it must be empty or not exist yet (not needed with --list) (env APPLE_OUT)
```

### Options inherited from parent commands

```
      --json[=true|false]   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple terraform](apple_terraform.md)	 - Write Terraform configuration for what the account already holds

