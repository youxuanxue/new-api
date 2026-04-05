FRONTEND_DIR = ./web
BACKEND_DIR = .

.PHONY: all build-frontend start-backend build-tt build-upstream

all: build-frontend start-backend

build-frontend:
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

# TT build (includes TT-specific features)
build-tt:
	@echo "Building TT version..."
	@cd $(BACKEND_DIR) && go build -tags tt -o new-api-tt main.go

# Upstream build (clean, PR-ready)
build-upstream:
	@echo "Building upstream version..."
	@cd $(BACKEND_DIR) && go build -o new-api main.go

# Run TT dev server
dev-tt:
	@echo "Starting TT dev server..."
	@cd $(BACKEND_DIR) && go run -tags tt main.go
