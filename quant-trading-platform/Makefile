.PHONY: up down build logs migrate seed test-backend test-ml psql clean

up:
	docker-compose up -d

down:
	docker-compose down

build:
	docker-compose build

logs:
	docker-compose logs -f

migrate:
	docker-compose run --rm api-gateway go run cmd/migrate/main.go

seed:
	docker-compose run --rm api-gateway go run cmd/seed/main.go

test-backend:
	docker-compose run --rm api-gateway go test ./...

test-ml:
	docker-compose run --rm ml-engine pytest

psql:
	docker-compose exec postgres psql -U quant -d quant_trading

clean:
	docker-compose down -v --remove-orphans