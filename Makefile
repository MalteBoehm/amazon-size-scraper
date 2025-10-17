.PHONY: build run test clean deps fmt lint install-tools

# Variables
BINARY_NAME=amazon-scraper
BINARY_PATH=bin/$(BINARY_NAME)
MAIN_PATH=cmd/scraper/main.go

# Build the application
build:
	@echo "Building..."
	@go build -o $(BINARY_PATH) $(MAIN_PATH)
	@go build -o bin/size-scraper cmd/size-scraper/main.go

# Run the application
run: build
	@echo "Running..."
	@./$(BINARY_PATH)

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.txt ./...

# Run tests with coverage report
test-coverage: test
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.txt coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install gotest.tools/gotestsum@latest

# Install Playwright browsers
install-playwright:
	@echo "Installing Playwright browsers..."
	@go run github.com/playwright-community/playwright-go/cmd/playwright install

# Development mode with hot reload (requires air)
dev:
	@air -c .air.toml

# Docker build
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(BINARY_NAME):latest .

# Lifecycle consumer
lifecycle-consumer:
	@echo "Running lifecycle consumer..."
	@DB_PASSWORD=postgres DB_PORT=5433 go run cmd/lifecycle-consumer/main.go

# Help
help:
	@echo "Available targets:"
	@echo "  build            - Build the application"
	@echo "  run              - Build and run the application"
	@echo "  test             - Run tests"
	@echo "  test-coverage    - Run tests with coverage report"
	@echo "  clean            - Clean build artifacts"
	@echo "  deps             - Download and tidy dependencies"
	@echo "  fmt              - Format code"
	@echo "  lint             - Run linter"
	@echo "  install-tools    - Install development tools"
	@echo "  install-playwright - Install Playwright browsers"
	@echo "  dev              - Run in development mode with hot reload"
	@echo "  docker-build     - Build Docker image"
	@echo "  lifecycle-consumer - Run the product lifecycle consumer"