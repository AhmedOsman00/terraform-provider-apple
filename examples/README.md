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
* **`data-sources/apple_devices/data-source.tf`** - Example usage of the Devices data source
* **`data-sources/apple_profiles/data-source.tf`** - **NEW** Example usage of the Provisioning Profiles data source
* **`resources/apple_bundle_id/resource.tf`** - Example usage of the Bundle ID resource with capabilities
* **`resources/apple_bundle_id_capability/resource.tf`** - Example usage of the Bundle ID Capability resource
* **`resources/apple_certificate/resource.tf`** - Example usage of the Certificate resource
* **`resources/apple_device/resource.tf`** - Example usage of the Device resource
* **`resources/apple_profile/resource.tf`** - **NEW** Example usage of the Provisioning Profile resource

### Runnable Examples

* **`main.tf`** - Complete example that demonstrates Bundle ID creation, capability management, certificate creation, device registration, **provisioning profile creation**, and data source querying
* **`profile/main.tf`** - **NEW** Comprehensive provisioning profile management example with multiple platforms and distribution methods

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
- **Device Management**: Registering and managing iOS, macOS, tvOS, watchOS, and visionOS devices for development and testing
- **Provisioning Profile Management**: **NEW** Creating and managing provisioning profiles for different distribution methods
- **Data Source Usage**: Querying existing Bundle IDs, capabilities, certificates, devices, and **profiles** with filtering and sorting options
- **Multi-platform Support**: Examples for iOS, macOS, tvOS, watchOS, and visionOS resources
- **Certificate Types**: Examples of different certificate types (development, distribution, Developer ID, etc.)
- **Device Types**: Examples of registering different device types (iPhone, iPad, Mac, Apple TV, Apple Watch, Vision Pro)
- **Profile Types**: **NEW** Examples of different provisioning profile types (development, App Store, Ad Hoc, enterprise distribution)
- **Settings Configuration**: How to configure complex capability settings for services like iCloud, App Groups, and Apple Pay
- **Capability Management**: Full lifecycle management of Bundle ID capabilities including creation, updates, and deletion
- **Certificate Lifecycle**: Certificate creation, querying, and revocation management
- **Device Registration**: Device registration with UDID validation and platform-specific requirements
- **Profile Lifecycle**: **NEW** Provisioning profile creation, updates, import, and deletion
- **Resource Relationships**: **NEW** How to link Bundle IDs, certificates, and devices through provisioning profiles
- **Distribution Workflows**: **NEW** Complete workflows for development, testing (Ad Hoc), and App Store distribution
- **Security Best Practices**: Handling sensitive certificate content, CSR data, device UDIDs, and profile content

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

### Device UDIDs

The device examples include placeholder UDID values. In practice, you need to obtain real UDIDs from your devices:

#### iOS Devices (iPhone, iPad, iPod touch, Apple Watch)
- Use Xcode: Window > Devices and Simulators, select your device
- Use iTunes/Finder: Connect device and view device information
- UDID format: 40-character hexadecimal string (e.g., `12345678-90123456789012345678901234567890`)

#### macOS Devices
- Use System Information: Apple menu > About This Mac > More Info > System Report > Hardware
- Use Terminal: `system_profiler SPHardwareDataType | grep "Hardware UUID"`
- UDID format: UUID format (e.g., `550e8400-e29b-41d4-a716-446655440000`)

#### Apple TV
- Settings > General > About > Identifier
- UDID format: UUID format

#### Apple Vision Pro  
- Settings > General > About > Identifier
- UDID format: UUID format

### Provisioning Profiles

The provisioning profile examples demonstrate the complete workflow for code signing and app distribution:

#### Profile Types
- **Development Profiles**: For running apps on registered devices during development
- **App Store Distribution**: For distributing apps through the App Store (no devices required)
- **Ad Hoc Distribution**: For distributing apps to specific devices outside the App Store
- **Enterprise Distribution**: For internal distribution within organizations

#### Key Relationships
- Each profile must be linked to exactly one Bundle ID
- Each profile requires one or more certificates (appropriate for the profile type)
- Development and Ad Hoc profiles require registered devices
- App Store and Enterprise profiles don't require devices

#### Profile Management
- Profile names can be updated after creation
- Other profile attributes (Bundle ID, certificates, devices) cannot be changed
- Profiles have expiration dates and need to be regenerated periodically
- Profiles can be imported by Apple ID or profile name

#### Example Workflow
```hcl
# 1. Create Bundle ID
resource "apple_bundle_id" "app" {
  identifier = "com.example.myapp"
  name       = "My App"
  platform   = "IOS"
}

# 2. Create development certificate (with real CSR)
resource "apple_certificate" "development" {
  certificate_type = "IOS_DEVELOPMENT"
  csr_content      = var.development_csr
}

# 3. Register test devices
resource "apple_device" "test_iphone" {
  name     = "Test iPhone"
  udid     = var.test_device_udid
  platform = "IOS"
}

# 4. Create development profile linking all components
resource "apple_profile" "development" {
  name         = "Development Profile"
  platform     = "IOS"
  bundle_id    = apple_bundle_id.app.id
  certificates = [apple_certificate.development.id]
  devices      = [apple_device.test_iphone.id]
}
```

### Resource Creation

The acceptance tests create real resources in your Apple Developer account and may affect your Bundle ID, certificate, device, and **provisioning profile quotas**. Always review the planned changes before applying.

### Certificate Management

- Certificates cannot be updated after creation - only created or revoked
- Certificate content is marked as sensitive and won't appear in plan/apply output
- Each certificate type has specific use cases and limitations
- Development certificates are tied to specific devices
- Distribution certificates are used for App Store and enterprise distribution

### Device Management

- Devices cannot be deleted via the API - they must be removed manually from the Apple Developer portal
- Device UDIDs are unique and cannot be changed after registration
- Each Apple Developer account has limits on the number of devices that can be registered
- Device names can be updated, but UDIDs and platforms cannot be changed
- iOS platform includes iPhone, iPad, iPod touch, and Apple Watch devices

### Provisioning Profile Management

- Provisioning profiles link Bundle IDs, certificates, and devices for code signing
- Profile names can be updated, but other attributes require replacement
- Profiles have expiration dates (typically 1 year) and must be renewed
- Development profiles require registered devices; App Store profiles do not
- Each profile type has specific use cases and certificate requirements
- Profile content is Base64-encoded and marked as sensitive
- Profiles can be imported by Apple ID or name for managing existing resources