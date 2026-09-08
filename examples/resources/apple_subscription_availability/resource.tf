data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_subscription_group" "premium" {
  app_id         = data.apple_apps.example.apps[0].id
  reference_name = "Premium"
}

resource "apple_subscription" "pro_monthly" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.monthly"
  name                = "Pro Monthly"
  subscription_period = "ONE_MONTH"
}

# A subscription has exactly one availability record, and it is a prerequisite
# for pricing: until one exists, Apple rejects apple_subscription_price with a
# 409 that mentions only "an error occurred while processing the pricing
# information" and never names the missing availability.
#
# Territories are named by three-letter Apple code -- USA, GBR, EGY -- not by
# the two-letter ISO 3166-1 alpha-2 code.
#
# This resource cannot be destroyed: Apple publishes neither PATCH nor DELETE
# for an availability, so an update is a POST that supersedes the previous
# record, and terraform destroy drops it from state and warns. To stop selling
# in a territory, remove it from available_territories.
resource "apple_subscription_availability" "pro_monthly" {
  subscription_id              = apple_subscription.pro_monthly.id
  available_in_new_territories = true

  available_territories = [
    "USA",
    "GBR",
    "DEU",
    "FRA",
    "EGY",
  ]
}
