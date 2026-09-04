.PHONY: up down logs psql psql-app run seed test migrate-up migrate-down migrate-status ui build dev-ui start

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f postgres

psql:
	docker compose exec postgres psql -U opd -d opd_db

run:
	set -a; . ./.env; set +a; go run ./cmd/server

migrate-up:
	set -a; . ./.env; set +a; goose -dir migrations postgres "$$MIGRATE_DATABASE_URL" up

migrate-down:
	set -a; . ./.env; set +a; goose -dir migrations postgres "$$MIGRATE_DATABASE_URL" down

migrate-status:
	set -a; . ./.env; set +a; goose -dir migrations postgres "$$MIGRATE_DATABASE_URL" status

psql-app:
	set -a; . ./.env; set +a; psql "$$DATABASE_URL"

test:
	go test -race ./...

seed:
	set -a; . ./.env; set +a; go run ./cmd/seed

ui:
	cd web && bun run build

dev-ui:
	cd web && bun run dev


build: ui
	go build -o bin/server ./cmd/server

start: build
	set -a; . ./.env; set +a; ./bin/server