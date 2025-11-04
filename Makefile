# Portico Makefile

.PHONY: build install clean test run

# Build the Portico CLI
build:
	go build -o bin/portico ./src/cmd/portico

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

# Setup development environment
setup:
	mkdir -p /home/portico/{apps,reverse-proxy,static}
	cp static/index.html /home/portico/static/
	cp static/config.yml /home/portico/
	cp static/docker-compose.yml /home/portico/reverse-proxy/
	chown -R portico:portico /home/portico

# Generate example docker compose
example:
	portico apps create sample-app
	cp examples/sample-app/* /home/portico/apps/sample-app/

# Docker operations
docker-build:
	docker build -t portico:latest .

docker-run:
	docker run -it --rm portico:latest

# Development
dev: build
	./bin/portico apps list
