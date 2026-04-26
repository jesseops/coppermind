.PHONY: all build test test-v lint clean css dev

# Binary output
BINARY := bin/coppermind
GO := go
GOFLAGS := -trimpath
LDFLAGS := -s -w

# Tailwind
TAILWIND_INPUT := internal/web/static/css/input.css
TAILWIND_OUTPUT := internal/web/static/css/app.css

all: css build

build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/coppermind

test:
	$(GO) test ./...

test-v:
	$(GO) test -v ./...

lint:
	$(GO) vet ./...

clean:
	rm -rf bin/
	rm -f $(TAILWIND_OUTPUT)

css:
	@echo "Tailwind CSS build (placeholder — will be configured in task 18)"
	@mkdir -p internal/web/static/css
	@touch $(TAILWIND_OUTPUT)

dev: css build
	./$(BINARY) serve

# Format all Go files
fmt:
	$(GO) fmt ./...

# Run tests with race detector
test-race:
	$(GO) test -race ./...
