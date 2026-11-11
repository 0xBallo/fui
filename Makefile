GO      ?= go
PKGS    ?= ./...
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: fmt fmt-check generate lint test check bench build

fmt:
	gofumpt -w .

fmt-check:
	@out=$$(gofumpt -l .); if [ -n "$$out" ]; then echo "unformatted:"; echo "$$out"; exit 1; fi

generate:
	$(GO) generate ./...

lint:
	golangci-lint run $(PKGS)

test:
	$(GO) test -race -count=1 $(PKGS)

# generate must not leave a diff once the repo has history
check: fmt-check generate lint test
	@git diff --exit-code --stat -- . ':!go.sum' || (echo "generated code out of date"; exit 1)

bench:
	$(GO) test -run '^$$' -bench . -benchmem $(PKGS)

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o dist/ ./cmd/...
