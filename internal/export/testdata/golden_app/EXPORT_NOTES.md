# Export notes

This configuration was written by `apple terraform export` from what App Store Connect holds.
Every resource has an `import` block beside it, so nothing is created:

```sh
terraform init
terraform plan
```

The plan should report **29 to import, 0 to add, 0 to change, 0 to destroy**. Anything else is something the export got wrong. Do not apply a plan that destroys or replaces anything; fix the configuration first. Once applied, the `import` blocks can be deleted.

## Imported

| Resource | Count |
|---|---|
| `apple_app_age_rating_declaration` | 1 |
| `apple_app_availability` | 1 |
| `apple_app_info` | 1 |
| `apple_app_info_localization` | 1 |
| `apple_app_price_schedule` | 1 |
| `apple_app_settings` | 1 |
| `apple_app_store_review_detail` | 1 |
| `apple_app_store_version` | 2 |
| `apple_app_store_version_localization` | 1 |
| `apple_beta_app_localization` | 1 |
| `apple_beta_app_review_detail` | 1 |
| `apple_beta_group` | 2 |
| `apple_beta_tester` | 1 |
| `apple_bundle_id` | 2 |
| `apple_in_app_purchase` | 1 |
| `apple_in_app_purchase_localization` | 1 |
| `apple_in_app_purchase_price_schedule` | 1 |
| `apple_profile` | 2 |
| `apple_subscription` | 1 |
| `apple_subscription_availability` | 1 |
| `apple_subscription_group` | 1 |
| `apple_subscription_group_localization` | 1 |
| `apple_subscription_introductory_offer_schedule` | 1 |
| `apple_subscription_localization` | 1 |
| `apple_subscription_price_schedule` | 1 |

## Needs attention

- `data.apple_certificates.example_distribution_distribution`: Certificate "Example Distribution" (serial 6A1B2C3D4E5F) is read through a data source, so Terraform will not renew it: early_renewal_hours acts only on an apple_certificate Terraform owns. To hand it over (provider 0.8.2 or later), import it as an apple_certificate with id "6A1B2C3D4E5F" and csr_content set to a CSR generated from the private key it was issued for, check that the plan updates it in place rather than replacing it (a replacement revokes it), then point the profiles that use it at the resource's id instead of data.apple_certificates.example_distribution_distribution.certificates[0].id. The Code Signing guide walks through it.
- `apple_app_age_rating_declaration.com_example_app`: 9 age rating questions are unanswered at Apple and are left out here. Leaving one out keeps Apple's answer rather than answering NONE, and an unanswered questionnaire blocks submission.
- `apple_app_store_version.com_example_app_ios`: Apple holds an earliest release date of 2026-11-01T08:00:00Z, which an import does not record. Add earliest_release_date to keep it; leaving it out leaves Apple's value alone.
- `apple_app_store_review_detail.com_example_app_ios`: A demo account is required, and Apple does not return its password. Set var.app_review_com_example_app_ios_demo_password (it is sensitive, and still stored in state in plain text); until it is set, Terraform leaves the password Apple holds alone.
- `apple_app_store_version.com_example_app_mac_os`: com.example.app 2.0 is READY_FOR_DISTRIBUTION, so Apple accepts no changes to it. Changing version_string replaces it with a new version, which is how the next release is cut.
- `apple_app_price_schedule.com_example_app`: This changes what com.example.app costs. Apple publishes no DELETE for a price schedule, so a change applied here cannot be undone by removing it.
- `apple_subscription_price_schedule.com_example_app_monthly`: The price in effect in each territory of com.example.app.monthly is listed, a scheduled change included; Apple's ended and preserved price records are not. Once the subscription is approved, changing a price_point_id schedules a dated price change rather than replacing the price, and a live price cannot be removed.

## Not in the configuration

- Bundle ID com-example-app-tests looks as though it belongs to com.example.app, but it is outside com.example.app.*, so it was not exported with it. If it does belong to the app, export it with --only bundle-ids and no --app.
- Device "Old iPad" (00008030-000B000000000000) is disabled, so it was left out.
- com.example.app 1.10 (IOS): screenshot sets were left out. Apple keeps a screenshot's bytes, not the file it came from, so an apple_app_screenshot_set needs the image files on disk; add one per display type pointing at them.
- com.example.app: App Privacy, app preview videos, the build upload, export compliance and submission have no resource; see remaining_manual_steps in examples/app-listing.
- subscription group "Premium": the approved localizations for en-US were left out. Each has an edit beside it that Apple has not approved yet, and the edit is what was exported: it is the copy that can be changed, and it replaces the approved one once Apple approves it.
- com.example.app.monthly: the approved localizations for en-US were left out. Each has an edit beside it that Apple has not approved yet, and the edit is what was exported: it is the copy that can be changed, and it replaces the approved one once Apple approves it.
- com.example.app: App Review screenshots of subscriptions and in-app purchases were left out. Apple keeps the bytes, not the file, so they need the image on disk; add an apple_subscription_review_screenshot or apple_in_app_purchase_review_screenshot pointing at it.
- `apple_beta_group.com_example_app_public_beta`: 1 testers who joined "Public Beta" through its public link were left out.
- com.example.app: builds given to beta groups (apple_beta_group_build) and per-build "What to Test" notes (apple_beta_build_localization) were left out. Both belong to one build and change with every upload, so they are better written next to the pipeline that produces the build.
