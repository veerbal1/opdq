.PHONY: up down logs psql run

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