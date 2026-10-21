# Written by `apple terraform export`. Credentials come from the
# APPLE_APP_STORE_CONNECT_* environment variables, as they did for the export.

terraform {
  required_version = ">= 1.5"
  required_providers {
    apple = {
      source  = "ahmedosman00/apple"
      version = "~> 0.1"
    }
  }
}

provider "apple" {
}
