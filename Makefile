-include .env
export

# ─── Tools ────────────────────────────────────────────────────────────────────
MIGRATE        := migrate
MIGRATE_DSN    := mysql://$(DB_USER):$(DB_PASSWORD)@tcp($(DB_HOST):$(DB_PORT))/$(DB_NAME)
MOCKGEN        := mockgen
GOLANGCI_LINT  := golangci-lint

# ─── Run locally ──────────────────────────────────────────────────────────────
.PHONY: run-bot
run-bot:
	go run ./cmd/bot

.PHONY: run-notifier
run-notifier:
	go run ./cmd/notifier

.PHONY: run-scheduler
run-scheduler:
	go run ./cmd/scheduler

# ─── Docker ───────────────────────────────────────────────────────────────────
.PHONY: docker-up
docker-up:
	docker compose up -d

.PHONY: docker-down
docker-down:
	docker compose down

.PHONY: docker-build
docker-build:
	docker compose build

.PHONY: docker-migrate
docker-migrate:
	docker compose run --rm migrate

.PHONY: docker-start
docker-start:
	docker compose build
	docker compose up -d


.PHONY: docker-clean
docker-clean:
	docker compose down -v --remove-orphans
	docker system prune -f
	@if [ "$(TEST_MODE)" = "true" ] && [ -n "$(DB_PATH)" ]; then \
		echo "TEST_MODE=true: очищаем данные MySQL ($(DB_PATH))..."; \
		rm -rf "$(DB_PATH)"; \
		echo "Директория $(DB_PATH) очищена."; \
	fi

.PHONY: docker-restart
docker-restart:
	docker compose down -v --remove-orphans
	docker system prune -f
	@if [ "$(TEST_MODE)" = "true" ] && [ -n "$(DB_PATH)" ]; then \
		echo "TEST_MODE=true: очищаем данные MySQL ($(DB_PATH))..."; \
		rm -rf "$(DB_PATH)"; \
		mkdir -p "$(DB_PATH)"; \
		echo "Директория $(DB_PATH) очищена. БД будет пересоздана заново."; \
	fi
	docker compose build
	docker compose up -d

# ─── Migrations ───────────────────────────────────────────────────────────────
.PHONY: migrate-up
migrate-up:
	$(MIGRATE) -path migrations -database "$(MIGRATE_DSN)" up

.PHONY: migrate-down
migrate-down:
	$(MIGRATE) -path migrations -database "$(MIGRATE_DSN)" down 1

.PHONY: migrate-status
migrate-status:
	$(MIGRATE) -path migrations -database "$(MIGRATE_DSN)" version

# ─── Tests ────────────────────────────────────────────────────────────────────
.PHONY: test
test:
	go test ./... -count=1 -race

.PHONY: test-integration
test-integration:
	docker compose -f docker-compose.test.yml up -d
	go test ./_test/integration/... -count=1 -tags=integration
	docker compose -f docker-compose.test.yml down

.PHONY: test-coverage
test-coverage:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ─── Code generation ──────────────────────────────────────────────────────────
.PHONY: mock-gen
mock-gen:
	go generate ./internal/domain/repository/...

# ─── Lint ─────────────────────────────────────────────────────────────────────
.PHONY: lint
lint:
	$(GOLANGCI_LINT) run ./...

# ─── Misc ─────────────────────────────────────────────────────────────────────
.PHONY: tidy
tidy:
	go mod tidy
