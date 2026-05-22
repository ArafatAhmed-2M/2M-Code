.PHONY: build install test run-dev clean lint

# Build the 2M Code binary
build:
	go build -o bin/2m ./cmd/2m

# Install the binary to /usr/local/bin
install: build
	cp bin/2m /usr/local/bin/2m

# Run all tests
test:
	go test ./...
	cd agent_engine && python -m pytest

# Run in development mode (pass ARGS for CLI arguments)
# Example: make run-dev ARGS="team list"
run-dev:
	go run ./cmd/2m $(ARGS)

# Run the Python agent engine standalone (for development)
run-engine:
	cd agent_engine && python server.py

# Lint all code
lint:
	go vet ./...
	cd agent_engine && python -m black --check .

# Format Python code
format:
	cd agent_engine && python -m black .

# Clean build artifacts
clean:
	rm -rf bin/

# Build for all platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build -o bin/2m-darwin-amd64 ./cmd/2m
	GOOS=darwin GOARCH=arm64 go build -o bin/2m-darwin-arm64 ./cmd/2m
	GOOS=linux GOARCH=amd64 go build -o bin/2m-linux-amd64 ./cmd/2m
	GOOS=windows GOARCH=amd64 go build -o bin/2m-windows-amd64.exe ./cmd/2m
