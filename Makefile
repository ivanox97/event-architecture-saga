.PHONY: help start stop up down test clean setup-local-db show-sql-setup health colima-start colima-stop colima-status podman-start podman-stop podman-status up-podman up-colima down-podman add-funds test-payment test-payment-status test-balance test-payment-card test-refund

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
		echo '${YELLOW}⚠ Failed to start infrastructure with Podman${RESET}'; \
		echo ''; \
		echo '${YELLOW}Please ensure:${RESET}'; \
		echo '  1. Podman is installed: ${GREEN}brew install podman podman-compose${RESET}'; \
		echo '  2. Podman machine is running: ${GREEN}make podman-start${RESET}'; \
		echo '  3. If permissions issue: ${GREEN}./fix-podman-permissions.sh${RESET}'; \
		echo ''; \
		echo '${YELLOW}Or use local PostgreSQL: ${GREEN}make setup-local-db${RESET}'; \
		exit 1; \
	fi
	@echo ''
	@export PGPASSWORD=$${POSTGRES_PASSWORD:-event_saga_pass} && \
	 echo '${GREEN}Setting up database migrations...${RESET}' && \
	 psql -h localhost -p 5433 -U event_saga -d event_saga_db -f migrations/001_create_events_table.sql >/dev/null 2>&1 && \
	 psql -h localhost -p 5433 -U event_saga -d event_saga_db -f migrations/002_create_error_logs_table.sql >/dev/null 2>&1 && \
	 echo '${GREEN}✓ Migrations completed${RESET}' || \
	 echo '${YELLOW}⚠ Migrations may have already been applied${RESET}'
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
	@pkill -f "go run" 2>/dev/null && echo '${GREEN}✓ All go run processes stopped${RESET}' || echo '${YELLOW}No go run processes found${RESET}'
	@echo '${YELLOW}Freeing service ports...${RESET}'
	@for port in 8080 8081 8082 8083; do \
		pid=$$(lsof -ti:$$port 2>/dev/null); \
		if [ -n "$$pid" ]; then \
			kill -9 $$pid 2>/dev/null && echo '${GREEN}✓ Port '$$port' freed${RESET}' || true; \
		fi; \
	done
	@echo '${YELLOW}Stopping Podman containers...${RESET}'
	@podman-compose down 2>/dev/null && echo '${GREEN}✓ Podman containers stopped${RESET}' || echo '${YELLOW}⚠ Could not stop Podman containers (may not be running)${RESET}'

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

# Podman commands
podman-status: ## Check Podman machine status
	@if command -v podman >/dev/null 2>&1; then \
		if podman machine list 2>/dev/null | grep -q "running"; then \
			echo '${GREEN}✓ Podman machine is running${RESET}'; \
			podman machine list; \
		else \
			echo '${YELLOW}⚠ Podman machine is not running${RESET}'; \
			podman machine list 2>/dev/null || true; \
		fi; \
	else \
		echo '${YELLOW}⚠ Podman is not installed${RESET}'; \
		echo '  Install with: ${GREEN}brew install podman podman-compose${RESET}'; \
		exit 1; \
	fi

podman-start: ## Start Podman machine if not running
	@if ! command -v podman >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ Podman is not installed${RESET}'; \
		echo '  Install with: ${GREEN}brew install podman podman-compose${RESET}'; \
		exit 1; \
	fi
	@if podman info >/dev/null 2>&1; then \
		echo '${GREEN}✓ Podman is ready${RESET}'; \
	elif podman machine list 2>/dev/null | grep -q "running"; then \
		echo '${GREEN}✓ Podman machine is already running${RESET}'; \
	else \
		echo '${GREEN}Starting Podman machine...${RESET}'; \
		if podman machine list 2>/dev/null | grep -q "podman-machine-default"; then \
			podman machine start 2>&1 || { \
				echo '${YELLOW}⚠ Failed to start existing machine. Trying to init new one...${RESET}'; \
				podman machine init --now 2>&1 || { \
					echo ''; \
					echo '${YELLOW}⚠ Failed to start Podman machine${RESET}'; \
					echo ''; \
					echo '${YELLOW}Common issues:${RESET}'; \
					echo '  1. Fix permissions: ${GREEN}./fix-podman-permissions.sh${RESET}'; \
					echo '  2. Or manually: ${GREEN}sudo chown -R $$USER:$$USER ~/.config${RESET}'; \
					echo '  3. Try manually: ${GREEN}podman machine init --now${RESET}'; \
					echo '  4. Check status: ${GREEN}podman machine list${RESET}'; \
					exit 1; \
				}; \
			}; \
		else \
			podman machine init --now 2>&1 || { \
				echo ''; \
				echo '${YELLOW}⚠ Failed to initialize Podman machine${RESET}'; \
				echo ''; \
				echo '${YELLOW}Common issues:${RESET}'; \
				echo '  1. Fix permissions: ${GREEN}./fix-podman-permissions.sh${RESET}'; \
				echo '  2. Or manually: ${GREEN}sudo chown -R $$USER:$$USER ~/.config${RESET}'; \
				echo '  3. Try manually: ${GREEN}podman machine init --now${RESET}'; \
				echo '  4. Alternative: Use Colima or local PostgreSQL'; \
				exit 1; \
			}; \
		fi; \
		sleep 3; \
	fi

podman-stop: ## Stop Podman machine
	@if command -v podman >/dev/null 2>&1; then \
		if podman machine list 2>/dev/null | grep -q "running"; then \
			echo '${YELLOW}Stopping Podman machine...${RESET}'; \
			podman machine stop; \
		else \
			echo '${YELLOW}Podman machine is not running${RESET}'; \
		fi; \
	else \
		echo '${YELLOW}⚠ Podman is not installed${RESET}'; \
	fi

up: ## Start PostgreSQL and Redpanda with Podman
	@echo '${GREEN}Checking Podman status...${RESET}'
	@if ! command -v podman >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ Podman is not installed${RESET}'; \
		echo ''; \
		echo '${YELLOW}Install Podman:${RESET}'; \
		echo '  macOS: ${GREEN}brew install podman podman-compose${RESET}'; \
		echo '  Linux: ${GREEN}sudo apt install podman podman-compose${RESET} o ${GREEN}sudo dnf install podman podman-compose${RESET}'; \
		exit 1; \
	fi
	@if ! command -v podman-compose >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ podman-compose is not installed${RESET}'; \
		echo '  Install with: ${GREEN}brew install podman-compose${RESET}'; \
		exit 1; \
	fi
	@if ! podman info >/dev/null 2>&1; then \
		if ! podman machine list 2>/dev/null | grep -q "running"; then \
			echo '${YELLOW}⚠ Podman machine is not running. Starting Podman machine...${RESET}'; \
			$(MAKE) podman-start || { \
				echo '${YELLOW}⚠ Failed to start Podman machine${RESET}'; \
				exit 1; \
			}; \
		fi; \
	fi
	@echo '${GREEN}✓ Podman is ready${RESET}'
	@echo '${GREEN}Starting PostgreSQL and Redpanda...${RESET}'
	@podman-compose up -d || { \
		echo '${YELLOW}⚠ podman-compose failed${RESET}'; \
		echo '  Make sure Podman machine is running: ${GREEN}make podman-start${RESET}'; \
		exit 1; \
	}
	@echo '${GREEN}Waiting for PostgreSQL...${RESET}' && \
	 timeout=30 && \
	 while [ $$timeout -gt 0 ]; do \
		if podman-compose exec -T postgres pg_isready -U event_saga >/dev/null 2>&1; then \
			break; \
		fi; \
		echo -n '.'; \
		sleep 1; \
		timeout=$$((timeout - 1)); \
	done && \
	echo '' && \
	if podman-compose exec -T postgres pg_isready -U event_saga >/dev/null 2>&1; then \
		echo '${GREEN}✓ PostgreSQL is ready!${RESET}'; \
	else \
		echo '${YELLOW}⚠ PostgreSQL may not be ready yet${RESET}'; \
	fi && \
	echo '${GREEN}Waiting for Redpanda...${RESET}' && \
	timeout=20 && \
	while [ $$timeout -gt 0 ]; do \
		if podman-compose exec -T redpanda rpk cluster health >/dev/null 2>&1; then \
			break; \
		fi; \
		echo -n '.'; \
		sleep 1; \
		timeout=$$((timeout - 1)); \
	done && \
	echo '' && \
	if podman-compose exec -T redpanda rpk cluster health >/dev/null 2>&1; then \
		echo '${GREEN}✓ Redpanda is ready!${RESET}'; \
	else \
		echo '${YELLOW}⚠ Redpanda may not be ready yet${RESET}'; \
	fi

down: ## Stop PostgreSQL and Redpanda containers
	@echo '${YELLOW}Stopping PostgreSQL and Redpanda...${RESET}'
	@podman-compose down 2>/dev/null || { \
		echo '${YELLOW}⚠ podman-compose not available, trying docker-compose...${RESET}'; \
		export DOCKER_HOST=unix://$$HOME/.colima/default/docker.sock 2>/dev/null && \
		docker-compose down 2>/dev/null || echo '${YELLOW}⚠ Could not stop containers${RESET}'; \
	}

up-colima: ## Start PostgreSQL and Redpanda with Colima (alternative)
	@echo '${GREEN}Checking Colima status...${RESET}'
	@if ! command -v colima >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ Colima is not installed${RESET}'; \
		echo '  Install with: ${GREEN}brew install colima${RESET}'; \
		exit 1; \
	fi
	@if ! colima status >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ Colima is not running. Starting Colima...${RESET}'; \
		colima start || { \
			echo '${YELLOW}⚠ Failed to start Colima${RESET}'; \
			exit 1; \
		}; \
		sleep 3; \
	fi
	@echo '${GREEN}✓ Colima is running${RESET}'
	@echo '${GREEN}Starting PostgreSQL and Redpanda...${RESET}'
	@export DOCKER_HOST=unix://$$HOME/.colima/default/docker.sock && \
	 export DOCKER_CONFIG=$$HOME/.docker && \
	 docker-compose up -d || { \
		echo '${YELLOW}⚠ docker-compose failed${RESET}'; \
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

up-podman: ## Start PostgreSQL and Redpanda with Podman (alias for 'up')
	@echo '${GREEN}Checking Podman status...${RESET}'
	@if ! command -v podman >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ Podman is not installed${RESET}'; \
		echo '  Install with: ${GREEN}brew install podman podman-compose${RESET}'; \
		exit 1; \
	fi
	@if ! command -v podman-compose >/dev/null 2>&1; then \
		echo '${YELLOW}⚠ podman-compose is not installed${RESET}'; \
		echo '  Install with: ${GREEN}brew install podman-compose${RESET}'; \
		exit 1; \
	fi
	@if ! podman machine list 2>/dev/null | grep -q "running"; then \
		echo '${YELLOW}⚠ Podman machine is not running. Starting Podman machine...${RESET}'; \
		$(MAKE) podman-start || { \
			echo '${YELLOW}⚠ Failed to start Podman machine${RESET}'; \
			exit 1; \
		}; \
	fi
	@echo '${GREEN}✓ Podman machine is running${RESET}'
	@echo '${GREEN}Starting PostgreSQL and Redpanda with Podman...${RESET}'
	@podman-compose up -d || { \
		echo '${YELLOW}⚠ podman-compose failed${RESET}'; \
		exit 1; \
	}
	@echo '${GREEN}Waiting for PostgreSQL...${RESET}' && \
	 timeout=30 && \
	 while [ $$timeout -gt 0 ]; do \
		if podman-compose exec -T postgres pg_isready -U event_saga >/dev/null 2>&1; then \
			break; \
		fi; \
		echo -n '.'; \
		sleep 1; \
		timeout=$$((timeout - 1)); \
	done && \
	echo '' && \
	if podman-compose exec -T postgres pg_isready -U event_saga >/dev/null 2>&1; then \
		echo '${GREEN}✓ PostgreSQL is ready!${RESET}'; \
	else \
		echo '${YELLOW}⚠ PostgreSQL may not be ready yet${RESET}'; \
	fi && \
	echo '${GREEN}Waiting for Redpanda...${RESET}' && \
	timeout=20 && \
	while [ $$timeout -gt 0 ]; do \
		if podman-compose exec -T redpanda rpk cluster health >/dev/null 2>&1; then \
			break; \
		fi; \
		echo -n '.'; \
		sleep 1; \
		timeout=$$((timeout - 1)); \
	done && \
	echo '' && \
	if podman-compose exec -T redpanda rpk cluster health >/dev/null 2>&1; then \
		echo '${GREEN}✓ Redpanda is ready!${RESET}'; \
	else \
		echo '${YELLOW}⚠ Redpanda may not be ready yet${RESET}'; \
	fi

down-podman: ## Stop PostgreSQL and Redpanda containers with Podman
	@echo '${YELLOW}Stopping PostgreSQL and Redpanda...${RESET}'
	@podman-compose down

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

# Default test user ID for easy testing
TEST_USER_ID ?= 123e4567-e89b-12d3-a456-426614174000

add-funds: ## Add funds to a wallet account (usage: make add-funds USER_ID=user-123 AMOUNT=1000)
	@if [ -z "$(USER_ID)" ] || [ -z "$(AMOUNT)" ]; then \
		echo '${YELLOW}Usage: make add-funds USER_ID=<user_id> AMOUNT=<amount>${RESET}'; \
		echo ''; \
		echo '${YELLOW}Example:${RESET}'; \
		echo '  make add-funds USER_ID=123e4567-e89b-12d3-a456-426614174000 AMOUNT=1000.0'; \
		exit 1; \
	fi
	@echo '${GREEN}Adding funds to wallet...${RESET}'
	@curl -s -X POST http://localhost:8081/internal/wallet/add-funds \
		-H "Content-Type: application/json" \
		-d "{\"user_id\": \"$(USER_ID)\", \"amount\": $(AMOUNT), \"reason\": \"Manual deposit via Makefile\"}" \
		| python3 -m json.tool 2>/dev/null || cat

test-balance: ## Check wallet balance (usage: make test-balance USER_ID=user-123)
	@USER_ID=$${USER_ID:-$(TEST_USER_ID)}; \
	echo '${GREEN}Checking wallet balance for user: '$$USER_ID'${RESET}'; \
	curl -s http://localhost:8081/internal/wallet/$$USER_ID | python3 -m json.tool 2>/dev/null || cat

test-payment: ## Create a wallet payment (usage: make test-payment USER_ID=user-123 AMOUNT=100)
	@USER_ID=$${USER_ID:-$(TEST_USER_ID)}; \
	AMOUNT=$${AMOUNT:-100.0}; \
	echo '${GREEN}Creating wallet payment...${RESET}'; \
	echo '${YELLOW}User ID: '$$USER_ID'${RESET}'; \
	echo '${YELLOW}Amount: '$$AMOUNT'${RESET}'; \
	RESPONSE=$$(curl -s -X POST http://localhost:8080/api/payments/wallet \
		-H "Content-Type: application/json" \
		-d "{\"user_id\": \"$$USER_ID\", \"service_id\": \"test-service\", \"amount\": $$AMOUNT, \"currency\": \"USD\"}"); \
	echo $$RESPONSE | python3 -m json.tool 2>/dev/null || echo $$RESPONSE; \
	echo ""; \
	PAYMENT_ID=$$(echo $$RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('payment_id', ''))" 2>/dev/null); \
	if [ -n "$$PAYMENT_ID" ]; then \
		echo '${GREEN}Payment created! Payment ID: '$$PAYMENT_ID'${RESET}'; \
		echo '${YELLOW}Save this payment_id to check status later${RESET}'; \
	fi

test-payment-status: ## Check payment status (usage: make test-payment-status PAYMENT_ID=payment-id)
	@if [ -z "$(PAYMENT_ID)" ]; then \
		echo '${YELLOW}Usage: make test-payment-status PAYMENT_ID=<payment_id>${RESET}'; \
		echo ''; \
		echo '${YELLOW}Example:${RESET}'; \
		echo '  make test-payment-status PAYMENT_ID=be203f47-5826-45a0-8cbd-f9d8ae59a654'; \
		exit 1; \
	fi
	@echo '${GREEN}Checking payment status...${RESET}'
	@curl -s http://localhost:8080/api/v1/payments/$(PAYMENT_ID) | python3 -m json.tool 2>/dev/null || cat

test-payment-card: ## Create a credit card payment (usage: make test-payment-card USER_ID=user-123 AMOUNT=100 CARD_TOKEN=token-123)
	@USER_ID=$${USER_ID:-$(TEST_USER_ID)}; \
	AMOUNT=$${AMOUNT:-100.0}; \
	CARD_TOKEN=$${CARD_TOKEN:-test-card-token-123}; \
	echo '${GREEN}Creating credit card payment...${RESET}'; \
	echo '${YELLOW}User ID: '$$USER_ID'${RESET}'; \
	echo '${YELLOW}Amount: '$$AMOUNT'${RESET}'; \
	echo '${YELLOW}Card Token: '$$CARD_TOKEN'${RESET}'; \
	RESPONSE=$$(curl -s -X POST http://localhost:8080/api/payments/creditcard \
		-H "Content-Type: application/json" \
		-d "{\"user_id\": \"$$USER_ID\", \"service_id\": \"test-service\", \"amount\": $$AMOUNT, \"currency\": \"USD\", \"card_token\": \"$$CARD_TOKEN\"}"); \
	echo $$RESPONSE | python3 -m json.tool 2>/dev/null || echo $$RESPONSE; \
	echo ""; \
	PAYMENT_ID=$$(echo $$RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('payment_id', ''))" 2>/dev/null); \
	if [ -n "$$PAYMENT_ID" ]; then \
		echo '${GREEN}Payment created! Payment ID: '$$PAYMENT_ID'${RESET}'; \
		echo '${YELLOW}Save this payment_id to check status later${RESET}'; \
	fi

test-refund: ## Process a refund (usage: make test-refund PAYMENT_ID=payment-id USER_ID=user-123 AMOUNT=50)
	@if [ -z "$(PAYMENT_ID)" ] || [ -z "$(AMOUNT)" ]; then \
		echo '${YELLOW}Usage: make test-refund PAYMENT_ID=<payment_id> AMOUNT=<amount> USER_ID=<user_id>${RESET}'; \
		echo ''; \
		echo '${YELLOW}Example:${RESET}'; \
		echo '  make test-refund PAYMENT_ID=be203f47-5826-45a0-8cbd-f9d8ae59a654 AMOUNT=50.0 USER_ID=123e4567-e89b-12d3-a456-426614174000'; \
		exit 1; \
	fi
	@USER_ID=$${USER_ID:-$(TEST_USER_ID)}; \
	echo '${GREEN}Processing refund...${RESET}'; \
	curl -s -X POST http://localhost:8081/internal/wallet/refund \
		-H "Content-Type: application/json" \
		-d "{\"payment_id\": \"$(PAYMENT_ID)\", \"user_id\": \"$$USER_ID\", \"amount\": $(AMOUNT), \"reason\": \"Test refund\"}" \
		| python3 -m json.tool 2>/dev/null || cat

clean: stop ## Clean build artifacts and stop services
	@echo '${YELLOW}Cleaning up...${RESET}'
	@rm -rf bin/ logs/
	@rm -f coverage.out coverage.html
	@make down 2>/dev/null || true
	@echo '${GREEN}✓ Cleanup complete${RESET}'
