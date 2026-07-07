.PHONY: help up down restart logs ps build pull clean smoke health

# Default: show available commands
help:
	@echo "Anti-Fraud Detector - local commands"
	@echo ""
	@echo "  make up        Start the whole stack (build if needed)"
	@echo "  make down      Stop the stack"
	@echo "  make restart   Restart all services"
	@echo "  make logs      Follow logs from all services"
	@echo "  make ps        Show running containers"
	@echo "  make build     Rebuild all images"
	@echo "  make pull      Pull latest images"
	@echo "  make smoke     Run smoke tests against health endpoints"
	@echo "  make clean     Stop and remove volumes (deletes data)"
	@echo ""

# Copy .env if missing, then start
up:
	@[ -f .env ] || cp .env.example .env
	docker compose up --build -d
	@echo ""
	@echo "Frontend:      http://localhost"
	@echo "API Gateway:   http://localhost:8080"
	@echo "RabbitMQ UI:   http://localhost:15672  (guest/guest)"
	@echo "MinIO Console: http://localhost:9001   (minioadmin/minioadmin)"

down:
	docker compose down

restart:
	docker compose restart

logs:
	docker compose logs -f

ps:
	docker compose ps

build:
	docker compose build

pull:
	docker compose pull

# Stop and delete volumes (wipes Postgres + MinIO data)
clean:
	docker compose down -v

# Smoke test: hit health endpoints through the gateway
smoke:
	@bash scripts/smoke-test.sh

health: smoke
