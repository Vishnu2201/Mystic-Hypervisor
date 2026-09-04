.PHONY: all build test clean build-backend build-cli build-frontend test-backend test-cli test-frontend

GO ?= go
NODE ?= node
NPM ?= npm

all: build test

build: build-backend build-cli build-frontend

build-backend:
	@echo "Building mysticd daemon..."
	cd backend && $(GO) build -v -o bin/mysticd ./cmd/mysticd

build-cli:
	@echo "Building mysticctl CLI..."
	cd cli && $(GO) build -v -o bin/mysticctl ./main.go

build-frontend:
	@echo "Building React frontend..."
	cd frontend && $(NPM) run build

test: test-backend test-cli test-frontend

test-backend:
	@echo "Running backend unit tests..."
	cd backend && $(GO) test -v ./...

test-cli:
	@echo "Running CLI unit tests..."
	cd cli && $(GO) test -v ./...

test-frontend:
	@echo "Checking frontend TypeScript types..."
	cd frontend && $(NPM) run build

clean:
	@echo "Cleaning build artifacts..."
	rm -rf backend/bin cli/bin frontend/dist
