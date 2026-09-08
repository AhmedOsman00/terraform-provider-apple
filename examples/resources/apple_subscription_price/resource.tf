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
