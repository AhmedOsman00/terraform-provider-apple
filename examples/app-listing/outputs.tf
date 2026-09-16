output "app_id" {
  description = "The Apple ID of the app this module manages."
  value       = local.app_id
}

output "app_store_version_id" {
  description = <<-EOT
    The version record everything release-scoped hangs off.

    Useful as the import ID for a review detail, and as the parent of any
    localization added outside this module.
  EOT
  value       = apple_app_store_version.this.id
}

output "app_version_state" {
  description = <<-EOT
    Where the version stands -- PREPARE_FOR_SUBMISSION until it is submitted.

    Apple freezes the metadata once this leaves the editable states, so a plan
    that suddenly cannot write is usually explained here.
  EOT
  value       = apple_app_store_version.this.app_version_state
}

output "app_store_age_rating" {
  description = "The age rating Apple computed from the advisory answers, such as FOUR_PLUS."
  value       = apple_app_info.this.app_store_age_rating
}

output "price_point_id" {
  description = "The price point the app is sold at in the base territory."
  value       = data.apple_app_price_points.base.price_points[0].id
}

output "territory_count" {
  description = "How many storefronts the app is available in."
  value       = length(apple_app_availability.this.territories)
}

output "remaining_manual_steps" {
  description = <<-EOT
    What still has to be done by hand before this version can be submitted.

    App Privacy is the one item in App Store Connect's submission checklist with
    no API behind it at all.
  EOT
  value = [
    "App Privacy: answer the data-collection questionnaire in App Store Connect. Apple publishes no endpoint for it -- there is no appDataUsages resource in the API. It is declared per app rather than per version, so it does not recur.",
    "Screenshots and previews: upload them in App Store Connect. This provider does not manage media assets.",
    "Build: upload it with Xcode, Transporter or fastlane, then attach it to the version.",
    "Submit for review: App Store Connect only, once the above are done.",
  ]
}
