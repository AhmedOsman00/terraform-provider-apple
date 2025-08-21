# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Terraform provider for Apple App Store Connect, built using the Terraform Plugin Framework. The provider allows management of Apple App Store Connect resources like Bundle IDs through Terraform configuration.

## Key Architecture Components

### Provider Structure
- **`main.go`**: Entry point that serves the provider with address `theaostudio.com/hashicorp/apple`
- **`internal/provider/`**: Core provider implementation
  - `provider.go`: Main provider configuration, authentication, and resource/data source registration
  - `bundleID_resource.go`: Bundle ID resource implementation (create/read/update/delete)
  - `bundleIDs_data_source.go`: Bundle IDs data source for querying existing Bundle IDs
  - Legacy example files for scaffolding reference

### Apple API Client
- **`internal/apple/`**: Apple App Store Connect API client
  - `client.go`: HTTP client with JWT authentication for Apple API
  - `models.go`: Data models for API requests/responses (BundleID, generic Response/Request wrappers)
  - `budleIds.go`: Bundle ID specific API operations (GetBundleIDs, CreateBundleID)

### Authentication
The provider uses JWT-based authentication with Apple App Store Connect API:
- Requires issuer_id, api_key, and private_key (PEM format)
- Can be configured via provider configuration or environment variables:
  - `APPLE_APP_STORE_CONNECT_ISSUER_ID`
  - `APPLE_APP_STORE_CONNECT_API_KEY` 
  - `APPLE_APP_STORE_CONNECT_PRIVATE_KEY`
- JWT tokens expire after 20 minutes and are created using ES256 signing

### Data Models
- **BundleID**: Represents an App Store Connect Bundle ID with identifier, name, platform (iOS/macOS/tvOS/watchOS), and seed_id
- Generic `Response[T]` and `ListResponse[T]` wrappers for API responses
- Generic `Request[T]` wrapper for API requests

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
- Bundle ID resource implementation is incomplete - Create method has leftover code from scaffolding template
- The provider serves on a custom registry address, not the official Terraform registry
- Environment variables take precedence over hardcoded configuration values
- JWT tokens are created fresh for each provider instance

## File Structure Context
- `examples/`: Terraform configuration examples
- `docs/`: Generated provider documentation  
- `tools/`: Go tools and utilities for code generation
- `terraform-registry-manifest.json`: Terraform registry metadata