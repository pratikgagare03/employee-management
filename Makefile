# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=employee-management
BINARY_UNIX=$(BINARY_NAME)_unix

# Build the application
build:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/main.go

# Run the application
run:
	$(GOCMD) run ./cmd/main.go

# Clean build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

# Run tests
test:
	$(GOTEST) -v ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Cross compilation for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v ./cmd/main.go

# Install the application
install:
	$(GOGET) ./...
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/main.go

# Format code
fmt:
	$(GOCMD) fmt ./...

# Docker commands
docker-build:
	docker build -t employee-management:latest .

docker-run:
	docker run -p 8081:8081 --env-file .env.docker employee-management:latest

docker-compose-up:
	docker-compose up --build -d

docker-compose-down:
	docker-compose down -v

docker-compose-logs:
	docker-compose logs -f

docker-clean:
	docker-compose down -v --rmi all --remove-orphans

# Help
help:
	@echo "Available commands:"
	@echo "  build        - Build the application"
	@echo "  run          - Run the application"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build files"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  fmt          - Format code"
	@echo "  help         - Show this help"

.PHONY: build run clean test test-coverage deps build-linux install fmt db-setup docker-up docker-down help
