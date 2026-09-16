# Every storefront the App Store operates in. This takes no argument: unlike
# the other App Store Connect data sources, territories hang off nothing --
# every account sees the same list.
data "apple_territories" "all" {}

# Territories trading in one currency, for a decision that only applies to the
# eurozone. Matched case-insensitively.
data "apple_territories" "eurozone" {
  currency = "EUR"
}

# Filter by code. The pattern is a regular expression matched as a substring,
# so anchor it with ^ and $ for an exact match.
data "apple_territories" "gulf" {
  id_pattern = "^(ARE|SAU|KWT|QAT|BHR|OMN)$"
}

# Sort by currency, which groups the storefronts that share one.
data "apple_territories" "by_currency" {
  sort_by    = "currency"
  sort_order = "asc"
}

# The reason the data source exists. An availability resource requires
# available_territories, and Apple's own default of "everywhere" applies only to
# a product whose availability has never been set -- so once Terraform owns the
# record, selling everywhere means naming every territory. Reading them keeps
# the list current when Apple opens a storefront; a literal list does not.
data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_subscription_group" "example" {
  app_id         = data.apple_apps.example.apps[0].id
  reference_name = "Premium"
}

resource "apple_subscription" "example" {
  group_id            = apple_subscription_group.example.id
  product_id          = "com.example.app.pro.monthly"
  name                = "Pro Monthly"
  subscription_period = "ONE_MONTH"
}

resource "apple_subscription_availability" "worldwide" {
  subscription_id              = apple_subscription.example.id
  available_territories        = data.apple_territories.all.ids
  available_in_new_territories = true
}
