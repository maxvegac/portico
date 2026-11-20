# Portico Makefile

.PHONY: build install clean test run lint lint-fix vet fmt fmt-check check setup docker-build docker-run dev

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# Build the Portico CLI
build:
	go build -ldflags "$(LDFLAGS)" -o bin/portico ./src/cmd/portico

# Install Portico CLI
install: build
	sudo cp bin/portico /usr/local/bin/
	sudo chmod +x /usr/local/bin/portico

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	golangci-lint run

# Run linter with fix
lint-fix:
	golangci-lint run --fix

# Run go vet
vet:
	go vet ./...

# Run go fmt
fmt:
	go fmt ./...

# Run go fmt check
fmt-check:
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "The following files are not formatted:"; \
		gofmt -s -l .; \
		exit 1; \
	fi

# Run all checks
check: fmt-check vet lint test

# Run the CLI
run: build
	./bin/portico

# Setup development environment (extracts embedded static files to /home/portico)
setup: build
	./bin/portico init

# Docker operations
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t portico:latest .

docker-run:
	docker run -it --rm portico:latest

# Development
dev: build
	./bin/portico list
