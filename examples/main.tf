terraform {
  required_providers {
    apple = {
      source = "theaostudio.com/hashicorp/apple"
    }
  }
}

provider "apple" {
  scope = ["GET /v1/bundleIds"]
}

data "bundle_ids" "ids" {}

output "bundle_identifiers" {
  value = data.bundle_ids.ids
}