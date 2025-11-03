.PHONY: help start stop up down test clean setup-local-db show-sql-setup health colima-start colima-stop colima-status

# Colors for output
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo '${GREEN}Available commands:${RESET}'
	@echo ''
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  ${YELLOW}%-20s${RESET} %s\n", $$1, $$2}'
	@echo ''

# Main commands
start: ## Start infrastructure and all services automatically
	@echo '${GREEN}════════════════════════════════════════════════${RESET}'
	@echo '${GREEN}  Starting Event-Driven Payment System${RESET}'
	@echo '${GREEN}════════════════════════════════════════════════${RESET}'
	@echo ''
	@if make up 2>/dev/null; then \
		echo ''; \
	else \
		echo ''; \
		echo '${YELLOW}⚠ Colima/Docker failed. Trying to use local PostgreSQL...${RESET}'; \
		echo ''; \
		if lsof -i :5432 >/dev/null 2>&1; then \
			echo '${GREEN}✓ Local PostgreSQL detected on port 5432${RESET}'; \
			echo '${YELLOW}  Make sure database is set up: ${GREEN}make setup-local-db${RESET}'; \
			echo ''; \
		else \
			echo '${YELLOW}⚠ No PostgreSQL found. Please:${RESET}'; \
			echo '  1. Start Colima: ${GREEN}make colima-start${RESET} or ${GREEN}colima start${RESET}'; \
			echo '  2. Setup local DB: ${GREEN}make setup-local-db${RESET}'; \
			exit 1; \
		fi; \
	fi
	@echo ''
	@echo '${GREEN}Starting all services...${RESET}'
	@echo ''
	@if command -v tmux >/dev/null 2>&1; then \
		echo '${GREEN}Using tmux to run all services...${RESET}'; \
		tmux new-session -d -s event-saga -n orchestrator 'cd $(PWD) && go run ./cmd/orchestrator/main.go' \; \
		split-window -h 'cd $(PWD) && go run ./cmd/wallet/main.go' \; \
		split-window -v 'cd $(PWD) && go run ./cmd/external-payment/main.go' \; \
		split-window -v 'cd $(PWD) && go run ./cmd/metrics/main.go' \; \
		select-layout tiled \; \
		set-option -g mouse on 2>/dev/null || true \; \
		attach-session -t event-saga; \
	else \
		echo '${YELLOW}tmux not found. Starting services in background...${RESET}'; \
		mkdir -p logs; \
		nohup go run ./cmd/orchestrator/main.go > logs/orchestrator.log 2>&1 & echo $$! > logs/orchestrator.pid; \
		sleep 1; \
		nohup go run ./cmd/wallet/main.go > logs/wallet.log 2>&1 & echo $$! > logs/wallet.pid; \
		sleep 1; \
		nohup go run ./cmd/external-payment/main.go > logs/external.log 2>&1 & echo $$! > logs/external.pid; \
		sleep 1; \
		nohup go run ./cmd/metrics/main.go > logs/metrics.log 2>&1 & echo $$! > logs/metrics.pid; \
		sleep 2; \
		echo ''; \
		echo '${GREEN}✓ All services started in background!${RESET}'; \
		echo '  View logs: ${GREEN}tail -f logs/*.log${RESET}'; \
		echo '  Stop: ${GREEN}make stop${RESET}'; \
	fi
	@echo ''
	@echo '${GREEN}✓ System is ready!${RESET}'
	@echo '  Test API: ${GREEN}curl http://localhost:8080/health${RESET}'
	@echo '  Health: ${GREEN}make health${RESET}'
	@echo '  Stop: ${GREEN}make stop${RESET}'
	@echo ''

stop: ## Stop all services
	@echo '${YELLOW}Stopping all services...${RESET}'
	@if [ -d logs ]; then \
		for pidfile in logs/*.pid; do \
			if [ -f $$pidfile ]; then \
				pid=$$(cat $$pidfile); \
				if kill -0 $$pid 2>/dev/null; then \
					kill $$pid 2>/dev/null || true; \
				fi; \
				rm -f $$pidfile; \
			fi; \
		done; \
		echo '${GREEN}✓ Background services stopped${RESET}'; \
	fi
	@if tmux has-session -t event-saga 2>/dev/null; then \
		tmux kill-session -t event-saga; \
		echo '${GREEN}✓ Tmux session stopped${RESET}'; \
	fi

# Infrastructure
colima-status: ## Check Colima status
	@if command -v colima >/dev/null 2>&1; then \
		colima status || echo '${YELLOW}⚠ Colima is not running${RESET}'; \
	else \
		echo '${YELLOW}⚠ Colima is not installed${RESET}'; \
		echo '  Install with: ${GREEN}brew install colima${RESET}'; \
		exit 1; \
	fi

colima-start: ## Start Colima if not running
	@if ! command -v colima >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ Colima is not installed${RESET}'; \
		echo '  Install with: ${GREEN}brew install colima${RESET}'; \
		exit 1; \
	fi
	@if colima status >/dev/null 2>&1; then \
		echo '${GREEN}✓ Colima is already running${RESET}'; \
	else \
		echo '${GREEN}Starting Colima...${RESET}'; \
		colima start; \
	fi

colima-stop: ## Stop Colima
	@if command -v colima >/dev/null 2>&1; then \
		if colima status >/dev/null 2>&1; then \
			echo '${YELLOW}Stopping Colima...${RESET}'; \
			colima stop; \
		else \
			echo '${YELLOW}Colima is not running${RESET}'; \
		fi; \
	else \
		echo '${YELLOW}⚠ Colima is not installed${RESET}'; \
	fi

up: ## Start PostgreSQL and Redpanda with Colima/Docker Compose
	@echo '${GREEN}Checking Colima status...${RESET}'
	@if ! command -v colima >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ Colima is not installed${RESET}'; \
		echo '  Install with: ${GREEN}brew install colima${RESET}'; \
		echo '  Or use local PostgreSQL: ${GREEN}make setup-local-db${RESET}'; \
		exit 1; \
	fi
	@if ! colima status >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ Colima is not running. Starting Colima...${RESET}'; \
		colima start || { \
			echo '${YELLOW}⚠ Failed to start Colima${RESET}'; \
			echo '  Use local PostgreSQL: ${GREEN}make setup-local-db${RESET}'; \
			exit 1; \
		}; \
		sleep 3; \
	fi
	@echo '${GREEN}✓ Colima is running${RESET}'
	@echo '${GREEN}Starting PostgreSQL and Redpanda...${RESET}'
	@export DOCKER_HOST=unix://$$HOME/.colima/default/docker.sock && \
	 export DOCKER_CONFIG=$$HOME/.docker && \
	 docker-compose up -d || { \
		echo '${YELLOW}⚠ docker-compose failed. Use ${GREEN}make setup-local-db${RESET} for local PostgreSQL${RESET}'; \
		exit 1; \
	}
	@export DOCKER_HOST=unix://$$HOME/.colima/default/docker.sock && \
	 export DOCKER_CONFIG=$$HOME/.docker && \
	 echo '${GREEN}Waiting for PostgreSQL...${RESET}' && \
	 timeout=30 && \
	 while [ $$timeout -gt 0 ]; do \
		if docker-compose exec -T postgres pg_isready -U event_saga >/dev/null 2>&1; then \
			break; \
		fi; \
		echo -n '.'; \
		sleep 1; \
		timeout=$$((timeout - 1)); \
	done && \
	echo '' && \
	if docker-compose exec -T postgres pg_isready -U event_saga >/dev/null 2>&1; then \
		echo '${GREEN}✓ PostgreSQL is ready!${RESET}'; \
	else \
		echo '${YELLOW}⚠ PostgreSQL may not be ready yet${RESET}'; \
	fi && \
	echo '${GREEN}Waiting for Redpanda...${RESET}' && \
	timeout=20 && \
	while [ $$timeout -gt 0 ]; do \
		if docker-compose exec -T redpanda rpk cluster health >/dev/null 2>&1; then \
			break; \
		fi; \
		echo -n '.'; \
		sleep 1; \
		timeout=$$((timeout - 1)); \
	done && \
	echo '' && \
	if docker-compose exec -T redpanda rpk cluster health >/dev/null 2>&1; then \
		echo '${GREEN}✓ Redpanda is ready!${RESET}'; \
	else \
		echo '${YELLOW}⚠ Redpanda may not be ready yet${RESET}'; \
	fi

down: ## Stop PostgreSQL and Redpanda containers
	@echo '${YELLOW}Stopping PostgreSQL and Redpanda...${RESET}'
	@export DOCKER_HOST=unix://$$HOME/.colima/default/docker.sock && \
	 export DOCKER_CONFIG=$$HOME/.docker && \
	 docker-compose down

# Database setup
setup-local-db: ## Setup database using local PostgreSQL
	@echo '${GREEN}Setting up database with local PostgreSQL...${RESET}'
	@if ! command -v psql >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ psql not found. Install with: ${GREEN}brew install postgresql${RESET}'; \
		echo '  Or use: ${GREEN}make show-sql-setup${RESET} for manual setup'; \
		exit 1; \
	fi
	@psql -h localhost -p 5432 -U postgres -c "CREATE DATABASE event_saga_db;" 2>/dev/null || \
	 echo '${YELLOW}Note: Database might already exist${RESET}'
	@psql -h localhost -p 5432 -U postgres -c "CREATE USER event_saga WITH PASSWORD '$${POSTGRES_PASSWORD:-event_saga_pass}';" 2>/dev/null || \
	 echo '${YELLOW}Note: User might already exist${RESET}'
	@psql -h localhost -p 5432 -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE event_saga_db TO event_saga;" 2>/dev/null || true
	@psql -h localhost -p 5432 -U postgres -d event_saga_db -c "GRANT ALL ON SCHEMA public TO event_saga; ALTER SCHEMA public OWNER TO event_saga;" 2>/dev/null || true
	@export PGPASSWORD=$${POSTGRES_PASSWORD:-event_saga_pass} && \
	 psql -h localhost -p 5432 -U event_saga -d event_saga_db -f migrations/001_create_events_table.sql >/dev/null 2>&1 && \
	 psql -h localhost -p 5432 -U event_saga -d event_saga_db -f migrations/002_create_error_logs_table.sql >/dev/null 2>&1 && \
	 echo '${GREEN}✓ Database setup complete!${RESET}'

show-sql-setup: ## Show SQL commands for manual database setup
	@echo '${GREEN}Manual Database Setup:${RESET}'
	@echo ''
	@echo '${YELLOW}As postgres user:${RESET}'
	@echo '  CREATE DATABASE event_saga_db;'
	@echo '  CREATE USER event_saga WITH PASSWORD '\''YOUR_PASSWORD_HERE'\'';'
	@echo '  GRANT ALL PRIVILEGES ON DATABASE event_saga_db TO event_saga;'
	@echo '  \\c event_saga_db'
	@echo '  GRANT ALL ON SCHEMA public TO event_saga;'
	@echo '  ALTER SCHEMA public OWNER TO event_saga;'
	@echo ''
	@echo '${YELLOW}Then run migrations:${RESET}'
	@echo '  cat migrations/001_create_events_table.sql | psql -U event_saga -d event_saga_db'
	@echo '  cat migrations/002_create_error_logs_table.sql | psql -U event_saga -d event_saga_db'
	@echo ''

# Testing
test: ## Run all tests
	go test ./... -v

# Utilities
health: ## Check health of all services
	@echo '${GREEN}Checking services...${RESET}'
	@curl -s http://localhost:8080/health 2>/dev/null && echo '${GREEN}✓ Orchestrator${RESET}' || echo '${YELLOW}✗ Orchestrator${RESET}'
	@curl -s http://localhost:8081/health 2>/dev/null && echo '${GREEN}✓ Wallet${RESET}' || echo '${YELLOW}✗ Wallet${RESET}'
	@curl -s http://localhost:8082/health 2>/dev/null && echo '${GREEN}✓ External Payment${RESET}' || echo '${YELLOW}✗ External Payment${RESET}'
	@curl -s http://localhost:8083/health 2>/dev/null && echo '${GREEN}✓ Metrics${RESET}' || echo '${YELLOW}✗ Metrics${RESET}'

clean: stop ## Clean build artifacts and stop services
	@echo '${YELLOW}Cleaning up...${RESET}'
	@rm -rf bin/ logs/
	@rm -f coverage.out coverage.html
	@make down 2>/dev/null || true
	@echo '${GREEN}✓ Cleanup complete${RESET}'
