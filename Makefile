# Define the name of the binary
BINARY_NAME=arbitage-build

# Define the build targets for different operating systems
build:
	go build -o ${BINARY_NAME} .

# Target to run the application
run: build
	./${BINARY_NAME}

# Clean target
clean:
	go clean
	rm -f ${BINARY_NAME}-linux ${BINARY_NAME}-darwin ${BINARY_NAME}-windows

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
	@echo "  make build        - Build the application"
	@echo "  make run          - Build and run the application"
	@echo "  make clean        - Remove built binaries"
	@echo "  make test         - Run tests"
	@echo "  make test_coverage - Run tests with coverage"
	@echo "  make dep          - Download dependencies"
	@echo "  make lint         - Run linter"
