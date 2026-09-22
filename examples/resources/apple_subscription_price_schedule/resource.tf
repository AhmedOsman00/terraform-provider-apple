data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_subscription_group" "premium" {
  app_id         = data.apple_apps.example.apps[0].id
  reference_name = "Premium"
}

resource "apple_subscription" "pro_annual" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.annual"
  name                = "Pro Annual"
  subscription_period = "ONE_YEAR"
}

# Availability first, and it has to cover every territory the schedule prices:
# Apple refuses a price in a territory the subscription is not available in, and
# says so without naming either. Apple's default of everywhere applies only
# until this resource exists, so selling everywhere means naming everywhere.
data "apple_territories" "all" {}

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

# What 99.99 in the United States is worth in every other storefront. There is
# no customer_price to filter the catalogue on for the other 174 territories,
# because 99.99 in the USA is neither 99.99 nor a round number anywhere else --
# this is the mapping App Store Connect's own price matrix is built from.
data "apple_subscription_price_point_equalizations" "annual" {
  price_point_id = data.apple_subscription_price_points.annual_base.price_points[0].id
}

# Whether Apple includes the base territory in the equalizations is Apple's
# call, so it is merged in rather than assumed -- merge keeps the later value,
# and both are the same price point.
locals {
  annual_price_points = merge(
    { USA = data.apple_subscription_price_points.annual_base.price_points[0].id },
    data.apple_subscription_price_point_equalizations.annual.price_point_ids,
  )
}

# Every territory, one API call.
#
# The same fan-out through apple_subscription_price is a for_each producing one
# resource -- and one POST /v1/subscriptionPrices -- per storefront, plus a
# refresh that lists the subscription's whole price collection once per
# resource, because Apple publishes no GET for a single price. This sends the
# set as a single PATCH /v1/subscriptions/{id} and reads it back in one listing.
resource "apple_subscription_price_schedule" "pro_annual" {
  subscription_id = apple_subscription.pro_annual.id

  prices = [
    for territory, price_point in local.annual_price_points : {
      territory_id   = territory
      price_point_id = price_point
    }
  ]

  depends_on = [apple_subscription_availability.pro_annual]
}

# A schedule is replaced wholesale on every write, so a price increase is an
# entry added to the same set rather than a resource of its own. start_date is
# a plain date, never a timestamp; preserve_current_price holds existing
# subscribers at what they already pay rather than moving them to the new price.
resource "apple_subscription" "pro_monthly" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.monthly"
  name                = "Pro Monthly"
  subscription_period = "ONE_MONTH"
}

resource "apple_subscription_availability" "pro_monthly" {
  subscription_id       = apple_subscription.pro_monthly.id
  available_territories = ["USA", "GBR"]
}

data "apple_subscription_price_points" "monthly_usa" {
  subscription_id = apple_subscription.pro_monthly.id
  territories     = ["USA"]
  customer_price  = "9.99"
}

data "apple_subscription_price_points" "monthly_gbr" {
  subscription_id = apple_subscription.pro_monthly.id
  territories     = ["GBR"]
  customer_price  = "9.99"
}

data "apple_subscription_price_points" "monthly_usa_increase" {
  subscription_id = apple_subscription.pro_monthly.id
  territories     = ["USA"]
  customer_price  = "12.99"
}

resource "apple_subscription_price_schedule" "pro_monthly" {
  subscription_id = apple_subscription.pro_monthly.id

  prices = [
    {
      territory_id   = "USA"
      price_point_id = data.apple_subscription_price_points.monthly_usa.price_points[0].id
    },
    {
      territory_id   = "GBR"
      price_point_id = data.apple_subscription_price_points.monthly_gbr.price_points[0].id
    },
    {
      territory_id           = "USA"
      price_point_id         = data.apple_subscription_price_points.monthly_usa_increase.price_points[0].id
      start_date             = "2027-01-01"
      preserve_current_price = true
    },
  ]

  depends_on = [apple_subscription_availability.pro_monthly]
}
