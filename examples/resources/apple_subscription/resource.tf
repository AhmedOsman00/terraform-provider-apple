data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_subscription_group" "premium" {
  app_id         = data.apple_apps.example.apps[0].id
  reference_name = "Premium"
}

# A subscription on its own is not sellable: Apple reports it as
# MISSING_METADATA until it has at least one localization and at least one
# price. Both are separate resources -- see apple_subscription_localization and
# apple_subscription_price.
resource "apple_subscription" "pro_monthly" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.monthly"
  name                = "Pro Monthly"
  subscription_period = "ONE_MONTH"
  family_sharable     = true
  group_level         = 1
  review_note         = "Sign in with the demo account, then open Settings > Upgrade."
}

# A second subscription in the same group at the same level is a crossgrade:
# a customer moving between them keeps one entitlement rather than two.
resource "apple_subscription" "pro_yearly" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.yearly"
  name                = "Pro Yearly"
  subscription_period = "ONE_YEAR"
  family_sharable     = true
  group_level         = 1
}

# A lower service level. Moving from level 1 to level 2 is a downgrade, which
# Apple applies at the next renewal rather than immediately.
resource "apple_subscription" "basic_monthly" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.basic.monthly"
  name                = "Basic Monthly"
  subscription_period = "ONE_MONTH"
  group_level         = 2
}
