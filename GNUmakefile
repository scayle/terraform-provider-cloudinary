default: build

.PHONY: build
build:
	go build ./...

.PHONY: install
install:
	go install .

.PHONY: test
test:
	go test ./... -timeout=120s

# Acceptance tests talk to a mocked Provisioning API by default (see
# internal/provider/*_test.go). Set the CLOUDINARY_* variables and point
# CLOUDINARY_API_BASE_URL at the real API to run them against Cloudinary.
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v -timeout=120m

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: lint
lint:
	go vet ./...
	gofmt -s -l .

.PHONY: generate
generate:
	go generate ./...
