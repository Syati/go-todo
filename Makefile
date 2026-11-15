.PHONY: dev run test lint check build up down migrate rollback db-status seed generate generate-sql generate-views format-views tools bootstrap
dev:
	mise exec -- air
run: generate-views
	mise exec -- go run ./cmd/go-todo serve
test: generate-views
	mise exec -- go test -race ./...
lint: generate-views
	mise exec -- golangci-lint run --fix
check:
	mise exec -- go tool templ generate -check
	mise exec -- go vet ./...
build: generate-views
	mise exec -- go build -o bin/go-todo ./cmd/go-todo
up:
	docker compose up -d --wait db
down:
	docker compose down
migrate:
	mise exec -- go run ./cmd/go-todo db up
rollback:
	mise exec -- go run ./cmd/go-todo db down
db-status:
	mise exec -- go run ./cmd/go-todo db status
seed:
	mise exec -- go run ./cmd/go-todo db seed
generate: generate-sql generate-views
generate-sql:
	mise exec -- sqlc generate
generate-views:
	mise exec -- go tool templ generate
format-views:
	mise exec -- go tool templ fmt internal/views
	mise exec -- go tool templ generate
tools:
	mise exec -- go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
bootstrap:
	mise exec -- go run ./cmd/go-todo admin bootstrap
