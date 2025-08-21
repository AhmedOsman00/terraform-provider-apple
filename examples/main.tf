terraform {
  required_providers {
    apple = {
      source = "theaostudio.com/hashicorp/apple"
    }
  }
}

# Configure the Apple Provider
provider "apple" {
  # Authentication via environment variables:
  # APPLE_APP_STORE_CONNECT_ISSUER_ID
  # APPLE_APP_STORE_CONNECT_API_KEY
  # APPLE_APP_STORE_CONNECT_PRIVATE_KEY
}

# Create a new Bundle ID
resource "apple_bundle_id" "example_app" {
  identifier = "com.example.myapp"
  name       = "My Example App"
  platform   = "IOS"
}

# Query existing Bundle IDs
data "apple_bundle_ids" "ios_apps" {
  platform = "IOS"
}

# Output the new Bundle ID details
output "new_bundle_id" {
  value = {
    id         = apple_bundle_id.example_app.id
    identifier = apple_bundle_id.example_app.identifier
    name       = apple_bundle_id.example_app.name
  }
}

# Output existing Bundle IDs
output "existing_ios_bundle_ids" {
  value = [for bid in data.apple_bundle_ids.ios_apps.bundle_ids : {
    identifier = bid.identifier
    name       = bid.name
    platform   = bid.platform
  }]
}