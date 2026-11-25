default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

# `make generate` only runs `terraform fmt` over examples/, which cannot catch
# configuration the provider schema rejects. This builds the provider and runs
# `terraform validate` in every example directory.
validate-examples:
	./scripts/validate-examples.sh

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -race -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

# Acceptance tests with the environment from the macOS Keychain
# (scripts/acc-env.sh). RUN narrows to a -run pattern, PKG to a package:
#   make testacc-local RUN=TestAccCertificateResource_adoptByCSR
RUN ?= .
PKG ?= ./...
testacc-local:
	scripts/acc-env.sh run env TF_ACC=1 go test -v -cover -timeout 120m -run '$(RUN)' $(PKG)

.PHONY: fmt lint test testacc testacc-local build install generate validate-examples
