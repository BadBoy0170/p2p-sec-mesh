.PHONY: up down build logs attack clean help

# ── Defaults ──────────────────────────────────────────────────────────────────
COMPOSE  := docker compose
NODES    := a b c d e

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

# ── Cluster lifecycle ──────────────────────────────────────────────────────────
up: ## Build images and start all 5 nodes + coordinator + ollama
	$(COMPOSE) up --build -d
	@echo ""
	@echo "  Dashboard: http://localhost:8090"
	@echo "  Logs:      make logs"
	@echo "  Attack:    make attack NODE=a"
	@echo ""

down: ## Stop and remove all containers
	$(COMPOSE) down

restart: down up ## Rebuild and restart everything

build: ## Build images without starting
	$(COMPOSE) build

# ── Logs ──────────────────────────────────────────────────────────────────────
logs: ## Stream logs from all containers
	$(COMPOSE) logs -f

logs-node: ## Stream logs for a specific node (make logs-node NODE=a)
	$(COMPOSE) logs -f node-$(NODE)-go node-$(NODE)-ai

logs-coordinator: ## Stream coordinator logs
	$(COMPOSE) logs -f coordinator

# ── Attack simulation ──────────────────────────────────────────────────────────
NODE ?= a
PORT_a := 2222
PORT_b := 2223
PORT_c := 2224
PORT_d := 2225
PORT_e := 2226

attack: ## Trigger a honeypot attack on a node (make attack NODE=a)
	@echo "Attacking node-$(NODE) on port $(PORT_$(NODE))..."
	@echo SCAN | nc -w1 localhost $(PORT_$(NODE)) || true
	@echo "Attack sent. Watch: make logs-node NODE=$(NODE)"

attack-all: ## Attack all 5 nodes simultaneously (tests consensus voting)
	@echo "Attacking all nodes..."
	@for port in 2222 2223 2224 2225 2226; do \
		echo SCAN | nc -w1 localhost $$port & \
	done; wait
	@echo "All attacks sent. Watch: make logs"

# ── Status ────────────────────────────────────────────────────────────────────
status: ## Show running containers and their status
	$(COMPOSE) ps

topology: ## Fetch current mesh topology as JSON
	@curl -s http://localhost:8090/api/topology | python3 -m json.tool

report: ## Fetch the latest AI threat event report
	@curl -s http://localhost:8090/api/report | python3 -m json.tool

# ── Cleanup ───────────────────────────────────────────────────────────────────
clean: ## Remove containers, images, and volumes
	$(COMPOSE) down --volumes --rmi local
