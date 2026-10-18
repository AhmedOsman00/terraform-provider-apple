# Create a Merchant ID for Apple Pay
resource "apple_merchant_id" "example_merchant" {
  identifier   = "merchant.com.example.myapp"
  display_name = "My App Store"
}

# Create another Merchant ID for a different service
resource "apple_merchant_id" "premium_service" {
  identifier   = "merchant.com.example.myapp.premium"
  display_name = "My App Premium Service"
}

# Create a Merchant ID for subscription payments
resource "apple_merchant_id" "subscriptions" {
  identifier   = "merchant.com.example.myapp.subscriptions"
  display_name = "My App Subscriptions"
}

# Example using the Merchant ID in a Bundle ID capability
# Note: This assumes you also have a Bundle ID resource defined
resource "apple_bundle_id" "example_app" {
  identifier = "com.example.myapp"
  name       = "My Example App"
  platform   = "IOS"
}

# Apple Pay takes no settings. The API cannot associate a Merchant ID with an
# App ID, so the Merchant ID created above is attached in the Developer portal.
resource "apple_bundle_id_capability" "apple_pay" {
  bundle_id       = apple_bundle_id.example_app.id
  capability_type = "APPLE_PAY"
}