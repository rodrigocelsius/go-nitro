.PHONY: test test-pkg test-cli test-blinq test-http test-data test-sql-integration openapi-smoke test-sql-tx

# Nested modules: `go test ./...` from the repo root does not work.
# Default `test` matches CI (pkg, CLI, blinq, fasthttp).

test: test-pkg test-cli test-blinq test-http

test-pkg:
	cd pkg && go test ./...

test-cli:
	cd cmd/nitrogen && go test ./...

# Static OpenAPI smoke (Resource + AuthRoutes examples)
openapi-smoke:
	cd cmd/nitrogen && go run . openapi --dir ../../examples/v1/db-blinq -o /tmp/nitro-openapi-db-blinq.json
	cd cmd/nitrogen && go run . openapi --dir ../../examples/v1/auth --bearer -o /tmp/nitro-openapi-auth.json

test-sql-tx:
	cd examples/v1/sql-tx && go test ./...

test-blinq:
	cd blinq && go test ./...

test-http:
	cd adapters/fasthttp && go test ./...

test-data:
	cd data/adapter/pgx && go test ./...
	cd data/adapter/sql && go test ./...

# Live CRUD against Postgres / MySQL / MariaDB / SQL Server / SQLite.
# Requires Docker. Starts compose, waits for health, runs //go:build integration.
test-sql-integration:
	docker compose -f data/adapter/sql/testdata/docker-compose.yml up -d
	@echo "waiting for databases…"
	@i=0; \
	until docker compose -f data/adapter/sql/testdata/docker-compose.yml exec -T postgres pg_isready -U postgres -d nitro >/dev/null 2>&1; do \
	  i=$$((i+1)); [ $$i -gt 60 ] && exit 1; sleep 2; \
	done
	@i=0; \
	until docker compose -f data/adapter/sql/testdata/docker-compose.yml exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot -pnitro --silent >/dev/null 2>&1; do \
	  i=$$((i+1)); [ $$i -gt 90 ] && exit 1; sleep 2; \
	done
	@i=0; \
	until docker compose -f data/adapter/sql/testdata/docker-compose.yml exec -T mariadb healthcheck.sh --connect --innodb_initialized >/dev/null 2>&1; do \
	  i=$$((i+1)); [ $$i -gt 90 ] && exit 1; sleep 2; \
	done
	@# SQL Server: create database nitro if missing, then wait for TCP accept
	@i=0; \
	until docker compose -f data/adapter/sql/testdata/docker-compose.yml exec -T sqlserver \
	  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'Nitro_Str0ng!' -C -Q "IF DB_ID('nitro') IS NULL CREATE DATABASE nitro;" >/dev/null 2>&1 \
	  || docker compose -f data/adapter/sql/testdata/docker-compose.yml exec -T sqlserver \
	  /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P 'Nitro_Str0ng!' -Q "IF DB_ID('nitro') IS NULL CREATE DATABASE nitro;" >/dev/null 2>&1; do \
	  i=$$((i+1)); [ $$i -gt 90 ] && exit 1; sleep 3; \
	done
	cd data/adapter/sql && NITRO_SQL_INTEGRATION=1 NITRO_DB_QUIET=1 go test -tags=integration -count=1 -timeout 5m ./...
