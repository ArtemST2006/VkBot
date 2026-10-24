GO ?= go
BINARY ?= bin/vk-bot
GOOSE ?= goose
GOOSE_DIR ?= migrations

-include .env
export VK_GROUP_TOKEN VK_GROUP_ID VK_ADMIN_ID DATABASE_URL

.PHONY: build run migrate migrate-down migrate-status migrate-create format check test

build:
	$(GO) build -trimpath -o $(BINARY) ./cmd/vk-bot

run: migrate
	$(GO) run ./cmd/vk-bot

migrate:
	@test -n "$(DATABASE_URL)" || { echo "Укажите DATABASE_URL"; exit 1; }
	@$(GOOSE) -dir "$(GOOSE_DIR)" postgres "$(DATABASE_URL)" up

migrate-down:
	@test -n "$(DATABASE_URL)" || { echo "Укажите DATABASE_URL"; exit 1; }
	@$(GOOSE) -dir "$(GOOSE_DIR)" postgres "$(DATABASE_URL)" down

migrate-status:
	@test -n "$(DATABASE_URL)" || { echo "Укажите DATABASE_URL"; exit 1; }
	@$(GOOSE) -dir "$(GOOSE_DIR)" postgres "$(DATABASE_URL)" status

migrate-create:
	@test -n "$(NAME)" || { echo "Укажите имя: make migrate-create NAME=create_tasks"; exit 1; }
	@$(GOOSE) -dir "$(GOOSE_DIR)" create "$(NAME)" sql

format:
	$(GO) fmt ./...

check:
	$(GO) vet ./...
	$(GO) test ./...

test:
	$(GO) test ./...
