.PHONY: up down logs bootstrap-user seed-dev test

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

test:
	cd backend && go test ./config ./middleware ./handlers ./repository
	cd frontend && npm run lint && npm run build
	cd sync_script && python3 -m unittest -v test_import_promo.py test_dedupe_promo.py
