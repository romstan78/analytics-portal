.PHONY: up down logs bootstrap-user seed-dev test test-e2e config-prod \
	types types-check demo-db-init demo-db-load demo-db-reset demo-up demo-down \
	demo-bootstrap-user demo-ecom-load demo-ecom-reset demo-registry-load \
	demo-registry-reset demo-kam-users demo-kam-users-preview demo-approval-scope

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

# Изолированный демонстрационный контур: отдельные контейнеры, порты и тома.
# Исходный my_project_mssql_data_volume эти команды не используют.
demo-db-init:
	docker compose -f docker-compose.demo.yml up -d --build mssql_db backend

demo-db-load:
	python3 sync_script/create_demo_promo_db.py

demo-db-reset:
	python3 sync_script/create_demo_promo_db.py --replace --confirm RESET_DEMO_PROMO_DB

# Интернет-продажи собираются из демо-справочников самой demo-БД, поэтому
# исходный SQL Server для этих команд не нужен вовсе.
demo-ecom-load:
	python3 sync_script/create_demo_ecom_sales.py

demo-ecom-reset:
	python3 sync_script/create_demo_ecom_sales.py --replace --confirm RESET_DEMO_ECOM_DB

# Реестр сетей: карточки из демо-справочника, факт отгрузок — из интернет-продаж.
demo-registry-load:
	python3 sync_script/create_demo_network_registry.py

demo-registry-reset:
	python3 sync_script/create_demo_network_registry.py --replace --confirm RESET_DEMO_NETWORK_REGISTRY

# Учётные записи КАМов демо-контура. Пароли генерируются на машине запускающего
# и печатаются один раз — их нужно сохранить сразу.
demo-kam-users-preview:
	python3 sync_script/create_demo_kam_users.py --dry-run

demo-kam-users:
	python3 sync_script/create_demo_kam_users.py

# Область согласования: старший КАМ видит промо только своих подчинённых.
demo-approval-scope:
	python3 sync_script/set_demo_approval_scope.py --preset demo

demo-up:
	docker compose -f docker-compose.demo.yml up -d --build

demo-down:
	docker compose -f docker-compose.demo.yml down

demo-bootstrap-user:
	docker compose -f docker-compose.demo.yml --profile tools run --rm bootstrap-user

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
	cd backend && go vet ./... && go test ./config ./middleware ./handlers ./repository ./services ./cmd/bootstrap_user
	cd frontend && npm run lint && npm run test:unit && npm run build
	cd sync_script && python3 -m unittest -v test_import_promo.py test_dedupe_promo.py test_import_network_facts.py test_create_demo_promo_db.py test_create_demo_ecom_sales.py test_create_demo_network_registry.py

test-e2e:
	cd frontend && npm run test:e2e

config-prod:
	docker compose -f docker-compose.yml -f docker-compose.production.yml config --quiet
