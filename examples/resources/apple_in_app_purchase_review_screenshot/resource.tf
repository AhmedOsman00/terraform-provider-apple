data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_in_app_purchase" "pro_unlock" {
  app_id               = data.apple_apps.example.apps[0].id
  product_id           = "com.example.app.prounlock"
  name                 = "Pro Unlock"
  in_app_purchase_type = "NON_CONSUMABLE"
}

# What App Review is shown of the purchase: a screenshot of the screen that
# sells it. Editing the image uploads it again; moving or renaming the file does
# not.
resource "apple_in_app_purchase_review_screenshot" "pro_unlock" {
  in_app_purchase_id = apple_in_app_purchase.pro_unlock.id
  file               = "${path.module}/review/pro-unlock.png"
}
