# GOTH Stack Boilerplate

Arquitetura monolítica de alta performance baseada em Go, SQLite, Templ e HTMX. Este projeto é focado em escalabilidade vertical, deploy simplificado através de binário único e eliminação de dependências de infraestrutura externa.

## O GOTH Way

A filosofia GOTH orienta o desenvolvimento deste sistema através de cinco pilares fundamentais:

1. Go como Fonte da Verdade: Toda a lógica de negócio, roteamento e validação residem exclusivamente no backend. O frontend atua como uma projeção do estado do servidor.
2. SSR Interativo (Templ + HTMX): Utilização de Templ para componentes type-safe e HTMX para interatividade, minimizando a necessidade de JavaScript no lado do cliente.
3. SQLite Hardened: O banco de dados é um arquivo local otimizado com modo WAL (Write-Ahead Logging), Busy Timeout e Sincronização Normal para garantir performance de nível de produção.
4. Single Binary: Assets, migrações de banco de dados, documentação API e o executável são consolidados em um único arquivo binário via go:embed.
5. Resiliência Operacional: Sistema de background jobs com rastreio de idempotência e recuperação automática de processos interrompidos (zombie recovery).

## Stack Tecnológico

| Componente | Tecnologia | Detalhe |
| --- | --- | --- |
| Linguagem | Go 1.24+ | Biblioteca padrão e arquitetura modular |
| Banco de Dados | SQLite | WAL Mode, FTS5, Foreign Keys |
| Interface | Templ | Server-Side Rendering type-safe |
| Interatividade | HTMX | Comunicação assíncrona servidor-cliente |
| Estilos | Tailwind CSS v4 | Compilação JIT nativa |
| Tempo Real | SSE Broker | Server-Sent Events direcionados por usuário |
| Fila de Jobs | Custom Engine | Persistência transacional em SQLite |
| Documentação | Swagger | Especificação OpenAPI 3.0 |

## Estrutura do Projeto

```
├── cmd/api/                # Ponto de entrada da aplicação
├── docs/                   # Especificação OpenAPI e Documentação
├── internal/
│   ├── cmd/                # Implementação dos comandos CLI
│   ├── db/                 # Camada de persistência (SQLC e Paging)
│   ├── middleware/         # Cadeia de interceptores HTTP (Auth, Logging, etc)
│   ├── web/                # Handlers HTTP e SSE Broker
│   └── worker/             # Processamento assíncrono e resiliência
├── migrations/             # Esquema SQL idempotente
├── test/                   # Benchmarks de performance e testes de integração
├── web/
│   └── static/             # Assets estáticos (CSS, JS, Imagens)
└── storage/                # Diretório de persistência de arquivos (local)
```

## Desenvolvimento

### Pré-requisitos

Para contribuir com este projeto, é necessário instalar as seguintes dependências em seu ambiente:

1. **Linguagens e Ambientes:**
   - [Go 1.25+](https://go.dev/doc/install)
   - [Node.js 20+](https://nodejs.org/) (incluindo NPM)

2. **Gerenciamento de Ferramentas:**
   Este projeto utiliza o sistema de `tool` do Go 1.24+. As ferramentas necessárias (`templ`, `sqlc`, `swag`, `govulncheck`, `air`, `lefthook`) são gerenciadas automaticamente através do arquivo `go.mod`.

   Para executar qualquer ferramenta manualmente, utilize:
   ```bash
   go tool <ferramenta> [argumentos]
   ```

### Procedimento Inicial

1. Instalação de dependências do Go e NPM:
```bash
go mod download
npm install
```

2. Ativação dos Git Hooks:
```bash
go tool lefthook install
```

3. Inicialização do ambiente de desenvolvimento:
```bash
make dev
```
Este comando executa a geração de código (`templ`, `sqlc`, `swag`), compilação de assets Tailwind, reset do banco de dados local com sementes (`seed`) e inicia o servidor com hot-reload.

## CLI Console

O binário gerado atua como uma ferramenta de linha de comando para operações administrativas:

- `server`: Inicia o servidor web (operação padrão)
- `migrate`: Executa migrações pendentes no banco de dados
- `seed`: Popula o banco com dados de teste
- `create-user`: Registra manualmente um usuário (args: `<email> <password>`)
- `help`: Exibe a lista de comandos disponíveis

## Infraestrutura e Observabilidade

- **Health Check:** `GET /health` - Monitoramento de conectividade de banco, latência de jobs e espaço em disco.
- **Métricas:** `GET /metrics` - Exposição de coletores nativos para Prometheus.
- **API Docs:** `GET /swagger/index.html` - Documentação interativa das rotas do sistema.

## Configuração

O sistema utiliza variáveis de ambiente para configuração. Em ambiente de produção (`APP_ENV=prod`), os seguintes parâmetros são obrigatórios:

- `SESSION_SECRET`: Chave para assinatura de cookies de sessão.
- `SMTP_USER` / `SMTP_PASS`: Credenciais de autenticação para o serviço de e-mail.
- `DATABASE_URL`: Caminho para o arquivo de banco de dados SQLite.

## Qualidade e Automação

### Guia de Testes

O projeto segue uma estrutura de testes rigorosa para garantir estabilidade e performance:

1. **Testes Unitários:** Localizados junto ao código fonte (`*_test.go`). Focam em lógica isolada de pacotes.
   ```bash
   make test
   ```
2. **Testes de Integração:** Testam a comunicação entre múltiplos componentes (ex: Web + DB).
3. **Benchmarks de Performance:** Localizados em `test/benchmarks/`. Medem latência, alocação de memória e concorrência.
   ```bash
   make bench
   ```
4. **Cobertura de Código:**
   ```bash
   make test-cover
   ```

O fluxo de trabalho é protegido por:

- **CI (GitHub Actions):** Execução automatizada de testes unitários, benchmarks de performance, análise de vulnerabilidades, linting e verificação de integridade de código gerado (Drift Check).
- **Git Hooks (Lefthook):** Executa formatação, geração de código e testes locais antes de permitir a subida de código ao repositório remoto.
