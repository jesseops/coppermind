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

# Build Docker image
docker:
	docker build -t coppermind:latest .

# Cross-compile releases
release:
	@mkdir -p bin/
	GOOS=linux   GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/coppermind-linux-amd64 ./cmd/coppermind
	GOOS=linux   GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/coppermind-linux-arm64 ./cmd/coppermind
	GOOS=darwin  GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/coppermind-darwin-amd64 ./cmd/coppermind
	GOOS=darwin  GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/coppermind-darwin-arm64 ./cmd/coppermind
