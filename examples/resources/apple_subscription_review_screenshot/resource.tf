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

# What App Review is shown of the subscription: a screenshot of the purchase
# screen as a customer sees it. Editing the image uploads it again; moving or
# renaming the file does not.
resource "apple_subscription_review_screenshot" "pro_monthly" {
  subscription_id = apple_subscription.pro_monthly.id
  file            = "${path.module}/review/paywall.png"
}
