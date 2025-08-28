# Apple Provider Examples

This directory contains examples for the Terraform Apple provider that demonstrate how to manage Apple App Store Connect resources. These examples are used for documentation and can also be run manually via the Terraform CLI.

## Prerequisites

Before running these examples, you'll need Apple App Store Connect API credentials:

- **Issuer ID**: Your App Store Connect API issuer ID (UUID format)
- **API Key**: Your 10-character alphanumeric API key ID  
- **Private Key**: Your App Store Connect API private key (.p8 file)

Set these as environment variables:

```bash
export APPLE_APP_STORE_CONNECT_ISSUER_ID="your-issuer-id"
export APPLE_APP_STORE_CONNECT_API_KEY="your-api-key-id"
export APPLE_APP_STORE_CONNECT_PRIVATE_KEY="$(cat AuthKey_XXXXXXXXXX.p8)"
```

## Example Files

The documentation generation tool looks for files in the following locations by default. All other *.tf files besides the ones mentioned below are ignored by the documentation tool. This is useful for creating examples that can run and/or are testable even if some parts are not relevant for the documentation.

### Documentation Examples

* **`provider/provider.tf`** - Example configuration for the Apple provider (used on the provider index page)
* **`data-sources/apple_bundle_ids/data-source.tf`** - Example usage of the Bundle IDs data source
* **`data-sources/apple_bundle_id_capabilities/data-source.tf`** - Example usage of the Bundle ID Capabilities data source
* **`resources/apple_bundle_id/resource.tf`** - Example usage of the Bundle ID resource with capabilities
* **`resources/apple_bundle_id_capability/resource.tf`** - Example usage of the Bundle ID Capability resource

### Runnable Examples

* **`main.tf`** - Complete example that demonstrates Bundle ID creation, capability management, and data source querying

## Running the Examples

1. **Set up authentication** (see Prerequisites above)

2. **Initialize Terraform**:
   ```bash
   terraform init
   ```

3. **Plan the deployment**:
   ```bash
   terraform plan
   ```

4. **Apply the configuration**:
   ```bash
   terraform apply
   ```

## What These Examples Demonstrate

- **Provider Configuration**: How to configure the Apple provider with authentication
- **Bundle ID Management**: Creating, updating, and managing Apple App Store Connect Bundle IDs
- **Bundle ID Capabilities**: Adding and configuring app capabilities like push notifications, iCloud, Apple Pay, etc.
- **Data Source Usage**: Querying existing Bundle IDs and capabilities with filtering and sorting options
- **Multi-platform Support**: Examples for iOS, macOS, tvOS, and watchOS Bundle IDs
- **Settings Configuration**: How to configure complex capability settings for services like iCloud, App Groups, and Apple Pay
- **Capability Management**: Full lifecycle management of Bundle ID capabilities including creation, updates, and deletion

## Note

The acceptance tests create real resources in your Apple Developer account and may affect your Bundle ID quota. Always review the planned changes before applying.