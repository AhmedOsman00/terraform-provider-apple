default: fmt lint install generate

# Overridable so that `make consume` can drive an installed applesign binary,
# or a signing configuration kept outside this repository.
APPLESIGN ?= go run ./cmd/applesign
SIGNING_DIR ?= examples/signing

build:
	go build -v ./...

install: build
	go install -v ./...

# Install the signing assets the examples/signing module produced onto this
# machine. applesign reads the bundle on stdin, so any other source -- sops, a
# password manager -- substitutes for the terraform command on the left.
consume:
	terraform -chdir=$(SIGNING_DIR) output -json signing_bundle | $(APPLESIGN) install -

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

.PHONY: fmt lint test testacc build install generate validate-examples consume
