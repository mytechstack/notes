.PHONY: help setup build up down logs clean test status dev

help:
	@echo "OPA Database Integration Commands:"
	@echo "  setup     - Initial project setup"
	@echo "  build     - Build all Docker images"
	@echo "  up        - Start all services"
	@echo "  down      - Stop all services"
	@echo "  dev       - Start development environment"
	@echo "  logs      - Show logs"
	@echo "  test      - Run tests"
	@echo "  status    - Check service status"
	@echo "  clean     - Clean up everything"

setup:
	@chmod +x scripts/*.sh
	@./scripts/setup.sh

build:
	@docker-compose build

up: build
	@docker-compose up -d
	@echo "Services started. Run 'make status' to check health."

down:
	@docker-compose down

dev: setup up
	@echo "Development environment ready!"
	@make status

logs:
	@docker-compose logs -f

test:
	@./scripts/test.sh

status:
	@echo "=== Service Status ==="
	@docker-compose ps
	@echo ""
	@curl -s http://localhost:8080/health | jq -r '"Bundle Server: " + .status' 2>/dev/null || echo "Bundle Server: Not responding"
	@curl -s http://localhost:8181/health | jq -r '"OPA: " + .status' 2>/dev/null || echo "OPA: Not responding"

clean:
	@./scripts/cleanup.sh

query-rbac:
	@curl -X POST http://localhost:8181/v1/data/rbac/allow \
		-H "Content-Type: application/json" \
		-d '{"input": {"user": {"roles": ["admin"]}, "required_role": "user"}}' | jq

restart: down up