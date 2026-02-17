.PHONY: generate css dev build test test-cover

generate: update-js
	@go tool templ generate
	@go tool sqlc generate
	@go tool swag init -g internal/cmd/server.go

update-js:
	@mkdir -p web/static/assets/js
	@cp node_modules/htmx.org/dist/htmx.min.js web/static/assets/js/
	@cp node_modules/alpinejs/dist/cdn.min.js web/static/assets/js/alpine.min.js

css:
	@npx @tailwindcss/cli -i ./web/static/assets/css/input.css -o ./web/static/assets/styles.css --minify

dev: generate css
	@rm -f goth.db goth.db-wal goth.db-shm
	@go run -tags fts5 ./cmd/api seed
	@go tool air

dev-reset: generate css
	@rm -f goth.db goth.db-wal goth.db-shm
	@go run -tags fts5 ./cmd/api seed

build: generate css
	@go build -tags fts5 -ldflags="-s -w" -o bin/goth ./cmd/api

test:
	@go test -tags fts5 -v -race ./internal/... ./test/...

test-cover:
	@go test -tags fts5 -coverprofile=coverage.out ./internal/...
	@go tool cover -html=coverage.out

bench:
	@go test -tags fts5 -bench=. -benchmem ./test/benchmarks/...
