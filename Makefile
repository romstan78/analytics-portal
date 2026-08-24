.PHONY: up down logs bootstrap-user seed-dev test test-e2e config-prod \
	types types-check

# Полный стек, включая SQL Server на постоянном томе mssql_data_volume.
up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

bootstrap-user:
	docker compose --profile tools run --rm bootstrap-user

seed-dev:
	docker compose --profile tools run --rm seed-dev

# Типы фронтенда генерируются из Go-структур: контракт API описан один раз.
types:
	cd backend && go run ./cmd/tsgen

# Падает, если api.generated.ts разошёлся с Go-структурами.
types-check:
	@tmp=$$(mktemp -t api.generated.XXXXXX.ts); \
	trap 'rm -f "$$tmp"' EXIT; \
	(cd backend && go run ./cmd/tsgen "$$tmp") >/dev/null; \
	diff -u frontend/src/types/api.generated.ts "$$tmp" \
		|| { echo "api.generated.ts устарел — выполните: make types"; exit 1; }
	@echo "Типы фронтенда совпадают с Go-структурами"

test: types-check
	cd backend && go vet ./... && go test ./config ./middleware ./handlers ./repository ./services
	cd frontend && npm run lint && npm run test:unit && npm run build
	cd sync_script && python3 -m unittest -v test_import_promo.py test_dedupe_promo.py test_import_network_facts.py

test-e2e:
	cd frontend && npm run test:e2e

config-prod:
	docker compose -f docker-compose.yml -f docker-compose.production.yml config --quiet
