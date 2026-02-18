.PHONY: setup generate css dev dev-reset build test test-cover bench check-lmstudio

# =============================================================================
# Setup e Instalação
# =============================================================================

setup:
	@echo "🚀 Configurando ambiente Elenchus..."
	@echo ""
	@if [ ! -f .env ]; then \
		echo "📝 Criando arquivo .env..."; \
		cp .env.example .env; \
		echo "✅ .env criado"; \
	else \
		echo "✅ .env já existe"; \
	fi
	@echo ""
	@echo "📦 Instalando dependências Node.js..."
	@npm install
	@echo "✅ Node dependencies instaladas"
	@echo ""
	@echo "🎉 Setup completo!"
	@echo ""
	@echo "Próximos passos:"
	@echo "  1. Edite o arquivo .env com suas configurações"
	@echo "  2. Inicie o LM Studio e carregue os modelos"
	@echo "  3. Execute: make check-lmstudio"
	@echo "  4. Execute: make dev"

# =============================================================================
# Verificações
# =============================================================================

check-lmstudio:
	@echo "🔍 Verificando LM Studio..."
	@bash -c 'source .env 2>/dev/null || true; \
	URL=$${LMSTUDIO_URL:-http://localhost:1234}; \
	MODEL=$${LMSTUDIO_MODEL_CHAT:-deepseek/deepseek-r1-0528-qwen3-8b}; \
	EMBED_MODEL=$${LMSTUDIO_MODEL_EMBEDDING:-text-embedding-qwen3-embedding-0.6b}; \
	echo "URL: $$URL"; \
	echo "Modelo Chat: $$MODEL"; \
	echo "Modelo Embedding: $$EMBED_MODEL"; \
	echo ""; \
	if curl -s "$$URL/v1/models" > /dev/null 2>&1; then \
		echo "✅ LM Studio está respondendo em $$URL"; \
	else \
		echo "❌ LM Studio não está respondendo em $$URL"; \
		echo "   Certifique-se de que o LM Studio está rodando e o servidor está ativado."; \
		exit 1; \
	fi'

check-env:
	@if [ ! -f .env ]; then \
		echo "❌ Arquivo .env não encontrado!"; \
		echo "   Execute: cp .env.example .env"; \
		exit 1; \
	fi
	@echo "✅ Arquivo .env encontrado"

# =============================================================================
# Geração de Código e Assets
# =============================================================================

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

# =============================================================================
# Desenvolvimento
# =============================================================================

dev: check-env generate css
	@rm -f elenchus.db elenchus.db-wal elenchus.db-shm
	@go run -tags fts5 ./cmd/api seed
	@go tool air

dev-reset: check-env generate css
	@rm -f elenchus.db elenchus.db-wal elenchus.db-shm
	@go run -tags fts5 ./cmd/api seed

# =============================================================================
# Build
# =============================================================================

build: check-env generate css
	@go build -tags fts5 -ldflags="-s -w" -o bin/elenchus ./cmd/api

# =============================================================================
# Testes
# =============================================================================

test:
	@go test -tags fts5 -v -race ./internal/... ./test/...

test-cover:
	@go test -tags fts5 -coverprofile=coverage.out ./internal/...
	@go tool cover -html=coverage.out

bench:
	@go test -tags fts5 -bench=. -benchmem ./test/benchmarks/...

# =============================================================================
# Limpeza
# =============================================================================

clean:
	@rm -f elenchus.db elenchus.db-wal elenchus.db-shm
	@rm -rf tmp/
	@rm -f coverage.out
	@echo "🧹 Limpeza completa"
