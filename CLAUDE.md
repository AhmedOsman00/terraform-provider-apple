# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Terraform provider for Apple App Store Connect, built using the Terraform Plugin Framework. The provider allows management of Apple App Store Connect resources like Bundle IDs through Terraform configuration.

## Key Architecture Components

### Provider Structure
- **`main.go`**: Entry point that serves the provider with address `theaostudio.com/hashicorp/apple`
- **`internal/provider/`**: Core provider implementation
  - `provider.go`: Main provider configuration, authentication, and resource/data source registration
  - `bundle/`: Modular Bundle ID implementation
    - `resource.go`: Bundle ID resource (create/read/update/delete) with comprehensive error handling
    - `data_source.go`: Bundle IDs data source with advanced filtering, sorting, and limiting capabilities
    - `models.go`: Terraform-specific models for Bundle ID resources and data sources
    - `filters.go`: Advanced filtering logic for Bundle IDs data source
  - Test files: `bundle_resource_test.go`, `bundle_data_source_test.go`

### Apple API Client
- **`internal/apple/`**: Apple App Store Connect API client
  - `client.go`: HTTP client with JWT authentication for Apple API
  - `budleIds.go`: Bundle ID specific API operations (GetBundleIDs, GetBundleID, CreateBundleID, UpdateBundleID, DeleteBundleID, GetBundleIDByIdentifier)
  - `models/`: Structured API data models
    - `bundle_id.go`: Bundle ID specific models (BundleID, BundleIDAttributes, etc.)
    - `common.go`: Generic API models (Response[T], ListResponse[T], Request[T])

### Authentication
The provider uses JWT-based authentication with Apple App Store Connect API:
- Requires issuer_id (UUID format), api_key (10-character alphanumeric), and private_key (PEM format)
- Comprehensive validation with regex patterns for each credential type
- Can be configured via provider configuration or environment variables:
  - `APPLE_APP_STORE_CONNECT_ISSUER_ID`
  - `APPLE_APP_STORE_CONNECT_API_KEY` 
  - `APPLE_APP_STORE_CONNECT_PRIVATE_KEY`
- JWT tokens expire after 20 minutes and are created using ES256 signing
- Optional scope parameter for API key access control

### Data Models
- **BundleID**: Represents an App Store Connect Bundle ID with identifier, name, platform (IOS/MAC_OS/TV_OS/WATCH_OS), and seed_id
- **Platform Types**: IOS, MAC_OS, TV_OS, WATCH_OS (defined as constants)
- Generic `Response[T]` and `ListResponse[T]` wrappers for API responses
- Generic `Request[T]` wrapper for API requests
- Separate create/update request models for Bundle ID operations

## Resource Features

### Bundle ID Resource (`apple_bundle_id`)
- **Full CRUD operations**: Create, Read, Update, Delete with comprehensive error handling
- **Import support**: Import by Apple ID or Bundle identifier (e.g., com.example.myapp)
- **Validation**: Regex validation for identifiers, platform constraints
- **Plan modifiers**: RequiresReplace for identifier/platform changes
- **Logging**: Structured logging throughout operations
- **Error handling**: Specific error messages for common scenarios (duplicate IDs, invalid formats)

### Bundle IDs Data Source (`apple_bundle_ids`)
- **Advanced filtering**: By platform, identifier patterns/prefixes, name patterns
- **Sorting**: By name, identifier, or platform in ascending/descending order
- **Limiting**: Configurable result limits (1-200)
- **Metadata**: Returns total count, filtered count, and last updated timestamp
- **Multiple platforms**: Filter by single platform or list of platforms

## Common Development Commands

### Building and Installation
```bash
# Build the provider
go build -v ./...

# Install the provider locally  
go install -v ./...

# Default make target (fmt, lint, install, generate)
make
```

### Testing
```bash
# Run unit tests
go test -v -cover -timeout=120s -parallel=10 ./...

# Run acceptance tests (creates real resources, costs money)
make testacc
# or
TF_ACC=1 go test -v -cover -timeout 120m ./...
```

### Code Quality
```bash
# Format code
make fmt
# or 
gofmt -s -w -e .

# Run linter
make lint
# or
golangci-lint run
```

### Documentation Generation
```bash
# Generate or update documentation
make generate
```

### Development Dependencies
```bash
# Add new dependency
go get github.com/author/dependency
go mod tidy
```

## Important Notes

- Provider uses Terraform Plugin Framework (not SDK v2)
- Bundle ID implementation is now complete with full CRUD operations and comprehensive validation
- Modular architecture with separate bundle package for better organization
- The provider serves on a custom registry address, not the official Terraform registry
- Environment variables take precedence over hardcoded configuration values
- JWT tokens are created fresh for each provider instance
- Comprehensive logging and error handling throughout
- Import functionality supports both Apple IDs and user-friendly Bundle identifiers

## File Structure Context
- `examples/`: Terraform configuration examples with data source and resource examples
- `docs/`: Generated provider documentation (index, resources, data-sources)
- `tools/`: Go tools and utilities for code generation
- `terraform-registry-manifest.json`: Terraform registry metadata
- `internal/bundle/`: Removed - functionality moved to `internal/provider/bundle/`