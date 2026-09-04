.PHONY: up down logs psql run migrate-up migrate-down migrate-status test

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