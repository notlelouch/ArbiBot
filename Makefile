# Define the name of the binary
BINARY_NAME=arbibot

# Define the build targets for different operating systems
build:
	GOARCH=amd64 GOOS=darwin go build -o bin/${BINARY_NAME}-darwin ./cmd/arbibot

build-linux:
	GOARCH=amd64 GOOS=linux go build -o bin/${BINARY_NAME}-linux ./cmd/arbibot

build-windows:
	GOARCH=amd64 GOOS=windows go build -o bin/${BINARY_NAME}-windows.exe ./cmd/arbibot

# Build for all platforms
build-all: build build-linux build-windows

# Target to run the application
run: build
	./bin/${BINARY_NAME}-darwin

# Clean target
clean:
	go clean
	rm -f bin/${BINARY_NAME}-darwin bin/${BINARY_NAME}-linux bin/${BINARY_NAME}-windows.exe

# Test target
test:
	go test ./...

# Test coverage target
test_coverage:
	go test ./... -coverprofile=coverage.out

# Dependency management target
dep:
	go mod download

# Linting target (requires golangci-lint)
lint:
	golangci-lint run --enable-all

# Help command to display available targets
help:
	@echo "Available commands:"
	@echo "  make build        - Build the application for macOS"
	@echo "  make build-linux  - Build the application for Linux"
	@echo "  make build-windows - Build the application for Windows"
	@echo "  make build-all    - Build the application for all platforms"
	@echo "  make run          - Build and run the application (macOS)"
	@echo "  make clean        - Remove built binaries"
	@echo "  make test         - Run tests"
	@echo "  make test_coverage - Run tests with coverage"
	@echo "  make dep          - Download dependencies"
	@echo "  make lint         - Run linter"
