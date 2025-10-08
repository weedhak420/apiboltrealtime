# Makefile for Bolt Tracker API

# Variables
APP_NAME := bolt-tracker
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_VERSION := $(shell go version | cut -d' ' -f3)
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Build flags
BUILD_FLAGS := -a -installsuffix cgo
CGO_ENABLED := 0
GOOS := linux

# Directories
BIN_DIR := bin
DIST_DIR := dist
DOCKER_DIR := .

# Colors
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

.PHONY: help build clean test lint fmt vet docker-build docker-run docker-push deps tidy run dev

# Default target
help: ## Show this help message
	@echo "$(BLUE)Bolt Tracker API - Build System$(NC)"
	@echo ""
	@echo "$(YELLOW)Available targets:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: clean deps ## Build the application
	@echo "$(BLUE)Building $(APP_NAME)...$(NC)"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build $(LDFLAGS) $(BUILD_FLAGS) -o $(BIN_DIR)/$(APP_NAME) .
	@echo "$(GREEN)Build completed: $(BIN_DIR)/$(APP_NAME)$(NC)"

build-all: clean deps ## Build for multiple platforms
	@echo "$(BLUE)Building for multiple platforms...$(NC)"
	@mkdir -p $(DIST_DIR)
	@echo "Building for Linux AMD64..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build $(LDFLAGS) $(BUILD_FLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 .
	@echo "Building for Linux ARM64..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 go build $(LDFLAGS) $(BUILD_FLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 .
	@echo "Building for Windows AMD64..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 go build $(LDFLAGS) $(BUILD_FLAGS) -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe .
	@echo "Building for macOS AMD64..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) $(BUILD_FLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 .
	@echo "Building for macOS ARM64..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) $(BUILD_FLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 .
	@echo "$(GREEN)Multi-platform build completed$(NC)"

# Development targets
dev: ## Run in development mode with hot reload
	@echo "$(BLUE)Starting development server...$(NC)"
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "$(YELLOW)Air not found, installing...$(NC)"; \
		go install github.com/cosmtrek/air@latest; \
		air; \
	fi

run: build ## Build and run the application
	@echo "$(BLUE)Running $(APP_NAME)...$(NC)"
	./$(BIN_DIR)/$(APP_NAME)

# Testing targets
test: ## Run tests
	@echo "$(BLUE)Running tests...$(NC)"
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests with coverage
	@echo "$(BLUE)Generating coverage report...$(NC)"
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

test-benchmark: ## Run benchmark tests
	@echo "$(BLUE)Running benchmark tests...$(NC)"
	go test -bench=. -benchmem ./...

# Code quality targets
lint: ## Run linter
	@echo "$(BLUE)Running linter...$(NC)"
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not found, installing...$(NC)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.54.2; \
		golangci-lint run; \
	fi

fmt: ## Format code
	@echo "$(BLUE)Formatting code...$(NC)"
	go fmt ./...

vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	go vet ./...

# Dependency management
deps: ## Download dependencies
	@echo "$(BLUE)Downloading dependencies...$(NC)"
	go mod download

tidy: ## Tidy dependencies
	@echo "$(BLUE)Tidying dependencies...$(NC)"
	go mod tidy

# Docker targets
docker-build: ## Build Docker image
	@echo "$(BLUE)Building Docker image...$(NC)"
	docker build -t $(APP_NAME):$(VERSION) .
	docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest
	@echo "$(GREEN)Docker image built: $(APP_NAME):$(VERSION)$(NC)"

docker-run: docker-build ## Build and run Docker container
	@echo "$(BLUE)Running Docker container...$(NC)"
	docker run --rm -p 8000:8000 -p 9090:9090 $(APP_NAME):latest

docker-push: docker-build ## Push Docker image to registry
	@echo "$(BLUE)Pushing Docker image...$(NC)"
	docker push $(APP_NAME):$(VERSION)
	docker push $(APP_NAME):latest

# Docker Compose targets
compose-up: ## Start all services with Docker Compose
	@echo "$(BLUE)Starting services with Docker Compose...$(NC)"
	docker-compose up -d

compose-down: ## Stop all services
	@echo "$(BLUE)Stopping services...$(NC)"
	docker-compose down

compose-logs: ## Show logs from all services
	@echo "$(BLUE)Showing logs...$(NC)"
	docker-compose logs -f

compose-restart: ## Restart all services
	@echo "$(BLUE)Restarting services...$(NC)"
	docker-compose restart

# Database targets
db-migrate: ## Run database migrations
	@echo "$(BLUE)Running database migrations...$(NC)"
	@if [ -f "migrations/up.sql" ]; then \
		mysql -h localhost -u root -p bolt_tracker < migrations/up.sql; \
	else \
		echo "$(YELLOW)No migrations found$(NC)"; \
	fi

db-seed: ## Seed database with test data
	@echo "$(BLUE)Seeding database...$(NC)"
	@if [ -f "seeds/seed.sql" ]; then \
		mysql -h localhost -u root -p bolt_tracker < seeds/seed.sql; \
	else \
		echo "$(YELLOW)No seed data found$(NC)"; \
	fi

# Monitoring targets
monitor: ## Start monitoring stack
	@echo "$(BLUE)Starting monitoring stack...$(NC)"
	docker-compose up -d prometheus grafana jaeger

# Security targets
security-scan: ## Run security scan
	@echo "$(BLUE)Running security scan...$(NC)"
	@if command -v gosec > /dev/null; then \
		gosec ./...; \
	else \
		echo "$(YELLOW)gosec not found, installing...$(NC)"; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
		gosec ./...; \
	fi

# Cleanup targets
clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html
	go clean

clean-docker: ## Clean Docker artifacts
	@echo "$(BLUE)Cleaning Docker artifacts...$(NC)"
	docker system prune -f
	docker volume prune -f

# Release targets
release: clean test lint build-all ## Create release
	@echo "$(BLUE)Creating release...$(NC)"
	@mkdir -p $(DIST_DIR)
	@cd $(DIST_DIR) && \
		for file in $(APP_NAME)-*; do \
			tar -czf $$file.tar.gz $$file; \
		done
	@echo "$(GREEN)Release created in $(DIST_DIR)/$(NC)"

# Info targets
info: ## Show build information
	@echo "$(BLUE)Build Information:$(NC)"
	@echo "  App Name: $(APP_NAME)"
	@echo "  Version: $(VERSION)"
	@echo "  Build Time: $(BUILD_TIME)"
	@echo "  Go Version: $(GO_VERSION)"
	@echo "  Build Flags: $(BUILD_FLAGS)"
	@echo "  LDFLAGS: $(LDFLAGS)"

# Install targets
install: build ## Install the application
	@echo "$(BLUE)Installing $(APP_NAME)...$(NC)"
	sudo cp $(BIN_DIR)/$(APP_NAME) /usr/local/bin/
	@echo "$(GREEN)Installed $(APP_NAME) to /usr/local/bin/$(NC)"

# Uninstall targets
uninstall: ## Uninstall the application
	@echo "$(BLUE)Uninstalling $(APP_NAME)...$(NC)"
	sudo rm -f /usr/local/bin/$(APP_NAME)
	@echo "$(GREEN)Uninstalled $(APP_NAME)$(NC)"

# Default target
.DEFAULT_GOAL := help
