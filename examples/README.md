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
* **`data-sources/apple_certificates/data-source.tf`** - Example usage of the Certificates data source
* **`resources/apple_bundle_id/resource.tf`** - Example usage of the Bundle ID resource with capabilities
* **`resources/apple_bundle_id_capability/resource.tf`** - Example usage of the Bundle ID Capability resource
* **`resources/apple_certificate/resource.tf`** - Example usage of the Certificate resource

### Runnable Examples

* **`main.tf`** - Complete example that demonstrates Bundle ID creation, capability management, certificate creation, and data source querying

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
- **Certificate Management**: Creating and managing development and distribution certificates
- **Data Source Usage**: Querying existing Bundle IDs, capabilities, and certificates with filtering and sorting options
- **Multi-platform Support**: Examples for iOS, macOS, tvOS, and watchOS Bundle IDs and certificates
- **Certificate Types**: Examples of different certificate types (development, distribution, Developer ID, etc.)
- **Settings Configuration**: How to configure complex capability settings for services like iCloud, App Groups, and Apple Pay
- **Capability Management**: Full lifecycle management of Bundle ID capabilities including creation, updates, and deletion
- **Certificate Lifecycle**: Certificate creation, querying, and revocation management
- **Security Best Practices**: Handling sensitive certificate content and CSR data

## Important Notes

### Certificate Signing Requests (CSRs)

The certificate examples include placeholder CSR content. In practice, you need to generate real CSRs using tools like OpenSSL:

```bash
# Generate a private key
openssl genrsa -out private.key 2048

# Generate a CSR
openssl req -new -key private.key -out request.csr -subj "/C=US/ST=CA/L=San Francisco/O=Your Company/CN=Your Name"

# Use the CSR content in your Terraform configuration
cat request.csr
```

### Resource Creation

The acceptance tests create real resources in your Apple Developer account and may affect your Bundle ID and certificate quotas. Always review the planned changes before applying.

### Certificate Management

- Certificates cannot be updated after creation - only created or revoked
- Certificate content is marked as sensitive and won't appear in plan/apply output
- Each certificate type has specific use cases and limitations
- Development certificates are tied to specific devices
- Distribution certificates are used for App Store and enterprise distribution