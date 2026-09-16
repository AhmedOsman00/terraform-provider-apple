data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_subscription_group" "premium" {
  app_id = data.apple_apps.example.apps[0].id

  # Internal to App Store Connect. Customers never see this.
  reference_name = "Premium"
}

# This is the heading a customer reads above the list of plans when they choose
# between the subscriptions in the group, and when they manage an existing
# subscription in Settings.
resource "apple_subscription_group_localization" "premium_en" {
  group_id = apple_subscription_group.premium.id
  locale   = "en-US"
  name     = "Premium"
}

# custom_app_name overrides the app name shown beside the group. Apple falls
# back to the app's own store name when it is unset.
resource "apple_subscription_group_localization" "premium_ar" {
  group_id        = apple_subscription_group.premium.id
  locale          = "ar-SA"
  name            = "بريميوم"
  custom_app_name = "تطبيق المثال"
}

# The group localization names the group; a subscription localization names one
# subscription inside it. Both are customer-facing, and they are separate
# records.
resource "apple_subscription" "pro_monthly" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.monthly"
  name                = "Pro Monthly"
  subscription_period = "ONE_MONTH"
}

resource "apple_subscription_localization" "pro_monthly_en" {
  subscription_id = apple_subscription.pro_monthly.id
  locale          = "en-US"
  name            = "Pro Monthly"
  description     = "Everything in Pro, billed monthly."
}
