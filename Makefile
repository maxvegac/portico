# Portico Makefile

.PHONY: build install clean test run

# Build the Portico CLI
build:
	go build -o bin/portico ./cmd/portico

# Install Portico CLI
install: build
	sudo cp bin/portico /usr/local/bin/
	sudo chmod +x /usr/local/bin/portico

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test ./...

# Run the CLI
run: build
	./bin/portico

# Setup development environment
setup:
	mkdir -p /home/portico/{apps,reverse-proxy,static}
	cp static/index.html /home/portico/static/
	chown -R portico:portico /home/portico

# Generate example docker-compose
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
