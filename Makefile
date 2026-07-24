.PHONY: up down build backend-dev frontend-dev ml-dev db-migrate db-reset logs clean

# ============================================
# Docker 服务管理
# ============================================

up:
	docker-compose up -d

down:
	docker-compose down

build:
	docker-compose build

logs:
	docker-compose logs -f

clean:
	docker-compose down -v

# ============================================
# 本地开发
# ============================================

backend-dev:
	cd backend && go run ./cmd/gateway/main.go

frontend-dev:
	cd frontend && npm run dev

ml-dev:
	cd ml-service && uvicorn app.main:app --host 0.0.0.0 --port 8000 --reload

# ============================================
# 数据库
# ============================================

db-migrate:
	cd backend && go run ./cmd/migrate/main.go up

db-reset:
	cd backend && go run ./cmd/migrate/main.go reset

# ============================================
# 工具
# ============================================

fmt:
	cd backend && go fmt ./...

lint:
	cd backend && golangci-lint run ./...
	cd frontend && npm run lint

test:
	cd backend && go test ./...
	cd frontend && npm test
	cd ml-service && python -m pytest

# ============================================
# 基础设施
# ============================================

infra-up:
	docker-compose up -d postgres timescaledb redis kafka zookeeper minio

infra-down:
	docker-compose down postgres timescaledb redis kafka zookeeper minio