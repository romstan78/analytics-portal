.PHONY: up down logs bootstrap-user seed-dev test test-e2e config-prod \
	up-existing-db down-existing-db logs-existing-db

# Полный стек, включая собственный контейнер SQL Server на томе mssql_data.
up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

# Режим внешней базы: SQL Server уже запущен отдельно (контейнер my_local_mssql
# на томе my_project_mssql_data_volume, алиас mssql_db в сети my_project_default).
# Поднимаются только backend и frontend, база не пересоздаётся.
EXISTING_DB_COMPOSE := -f docker-compose.yml -f docker-compose.existing-db.yml

up-existing-db:
	docker compose $(EXISTING_DB_COMPOSE) up -d --build

down-existing-db:
	docker compose $(EXISTING_DB_COMPOSE) down

logs-existing-db:
	docker compose $(EXISTING_DB_COMPOSE) logs -f --tail=200

bootstrap-user:
	docker compose --profile tools run --rm bootstrap-user

seed-dev:
	docker compose --profile tools run --rm seed-dev

test:
	cd backend && go vet ./... && go test ./config ./middleware ./handlers ./repository ./services
	cd frontend && npm run lint && npm run test:unit && npm run build
	cd sync_script && python3 -m unittest -v test_import_promo.py test_dedupe_promo.py test_import_network_facts.py

test-e2e:
	cd frontend && npm run test:e2e

config-prod:
	docker compose -f docker-compose.yml -f docker-compose.production.yml config --quiet
