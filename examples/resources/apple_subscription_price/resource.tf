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

# A subscription cannot be priced in a territory it is not available in. Apple
# rejects the price with a 409 that names neither availability nor the
# territory, so the availability has to come first -- and nothing in a price
# references it, which is why the dependency is declared by hand below.
resource "apple_subscription_availability" "pro_monthly" {
  subscription_id       = apple_subscription.pro_monthly.id
  available_territories = ["USA"]
}

# A price is never a number. Apple publishes a catalogue of price points, each
# fixing a customer price and the developer proceeds for one territory, and a
# price references one of them.
#
# Always pass territories: the unfiltered catalogue covers every territory the
# App Store sells in, and the filter is applied by Apple rather than in memory.
data "apple_subscription_price_points" "usd_9_99" {
  subscription_id = apple_subscription.pro_monthly.id
  territories     = ["USA"]
  customer_price  = "9.99"
}

resource "apple_subscription_price" "pro_monthly_usa" {
  subscription_id = apple_subscription.pro_monthly.id
  price_point_id  = data.apple_subscription_price_points.usd_9_99.price_points[0].id

  depends_on = [apple_subscription_availability.pro_monthly]
}

# Pricing every territory the subscription sells in, from that one price.
#
# apple_subscription_price_points answers "what may this cost in these
# territories", which leaves a caller pricing 175 storefronts to decide each
# one: there is no customer_price to filter on, because 9.99 in the United
# States is neither 9.99 nor a round number anywhere else. Apple holds that
# mapping -- it is what App Store Connect's price matrix is built from -- and
# apple_subscription_price_point_equalizations reads it.
resource "apple_subscription" "pro_annual" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.annual"
  name                = "Pro Annual"
  subscription_period = "ONE_YEAR"
}

data "apple_territories" "all" {}

# Availability first, and it has to cover every territory the fan-out prices:
# Apple refuses a price in a territory the subscription is not available in, and
# says so without naming either. Apple's default of everywhere applies only
# until this resource exists, so selling everywhere means naming everywhere.
resource "apple_subscription_availability" "pro_annual" {
  subscription_id       = apple_subscription.pro_annual.id
  available_territories = data.apple_territories.all.ids
}

# The one number in the configuration.
data "apple_subscription_price_points" "annual_base" {
  subscription_id = apple_subscription.pro_annual.id
  territories     = ["USA"]
  customer_price  = "99.99"
}

data "apple_subscription_price_point_equalizations" "annual" {
  price_point_id = data.apple_subscription_price_points.annual_base.price_points[0].id
}

# price_point_ids maps a territory code to the price point equalized for it, so
# the fan-out is a for_each rather than a resource per storefront. Whether Apple
# includes the base territory is Apple's call, so it is merged in rather than
# assumed -- merge keeps the later value, and both are the same price point.
locals {
  annual_price_points = merge(
    { USA = data.apple_subscription_price_points.annual_base.price_points[0].id },
    data.apple_subscription_price_point_equalizations.annual.price_point_ids,
  )
}

resource "apple_subscription_price" "pro_annual" {
  for_each = local.annual_price_points

  subscription_id = apple_subscription.pro_annual.id
  price_point_id  = each.value
  territory_id    = each.key

  depends_on = [apple_subscription_availability.pro_annual]
}

# A price scheduled for a future date. start_date is a plain date, never a
# timestamp. preserve_current_price holds existing subscribers at what they
# already pay rather than moving them to the new price.
data "apple_subscription_price_points" "usd_12_99" {
  subscription_id = apple_subscription.pro_monthly.id
  territories     = ["USA"]
  customer_price  = "12.99"
}

resource "apple_subscription_price" "pro_monthly_usa_increase" {
  subscription_id        = apple_subscription.pro_monthly.id
  price_point_id         = data.apple_subscription_price_points.usd_12_99.price_points[0].id
  start_date             = "2027-01-01"
  preserve_current_price = true

  depends_on = [apple_subscription_availability.pro_monthly]
}
