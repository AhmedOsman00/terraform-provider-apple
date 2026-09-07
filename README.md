# Terraform Apple Provider

A Terraform provider for managing Apple App Store Connect resources using the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

This provider allows you to manage Apple App Store Connect resources such as Bundle IDs through Terraform configuration, using JWT authentication with the Apple App Store Connect API.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.25

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the provider

### Authentication

The provider requires Apple App Store Connect API credentials:

- **issuer_id**: Your App Store Connect API Key Issuer ID
- **api_key**: Your App Store Connect API Key ID  
- **private_key**: Your App Store Connect API Private Key (PEM format)

These can be configured via provider configuration or environment variables:

```bash
export APPLE_APP_STORE_CONNECT_ISSUER_ID="your-issuer-id"
export APPLE_APP_STORE_CONNECT_API_KEY="your-api-key-id"
export APPLE_APP_STORE_CONNECT_PRIVATE_KEY="$(cat AuthKey_XXXXXXXXXX.p8)"
```

### Example Usage

```terraform
terraform {
  required_providers {
    apple = {
      source = "aostudio.com/aostudio/apple"
    }
  }
}

# Configure the Apple Provider
provider "apple" {
  issuer_id    = "your-issuer-id"
  api_key      = "your-api-key-id"
  private_key  = file("AuthKey_XXXXXXXXXX.p8")
}

# Create a Bundle ID
resource "apple_bundle_id" "example" {
  identifier = "com.example.myapp"
  name       = "My Example App"
  platform   = "IOS"
}

# Query existing Bundle IDs
data "apple_bundle_ids" "ios_apps" {
  platform = "IOS"
}
```

### Replacing fastlane match

`examples/signing/` is a complete, runnable module that manages everything
`fastlane match` manages in the developer portal — App ID, capabilities,
devices, signing certificates, and one provisioning profile per distribution
method — together with `scripts/install-signing.sh`, which does the part match
does on the machine: assembling a `.p12`, importing it into a keychain (login or
a throwaway CI keychain), and installing profiles where Xcode looks for them.

Read `examples/signing/README.md` before pointing it at a real team: Apple caps
how many distribution certificates an account may hold, so one Terraform state
must own them, and that state holds the signing private keys.

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `make generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```