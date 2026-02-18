# Google Gemini API Integration

Este documento descreve a integração com a Google Gemini API para substituir o LM Studio na geração de conteúdo e embeddings.

## Visão Geral

A integração utiliza o **Google Gen AI SDK** (`google.golang.org/genai`) para acessar os modelos:
- **gemini-2.5-flash**: Geração de conteúdo (texto/multimodal)
- **gemini-embedding-001**: Vetorização de texto (embeddings)

## Configuração

### 1. Obtenha sua API Key

1. Acesse: https://aistudio.google.com/apikey
2. Crie uma nova API key
3. **Nunca** insira a chave diretamente no código

### 2. Configure as Variáveis de Ambiente

Copie `.env.example` para `.env` e configure:

```bash
# API Key (obrigatório)
GEMINI_API_KEY=sua-api-key-aqui

# Ou alternativamente:
# GOOGLE_API_KEY=sua-api-key-aqui

# Modelos (opcionais - usam padrões se não especificados)
GEMINI_MODEL_CHAT=gemini-2.5-flash
GEMINI_MODEL_EMBEDDING=gemini-embedding-001

# Timeout em segundos (opcional, padrão: 300s)
GEMINI_TIMEOUT=300
```

## Uso

### Inicialização do Cliente

```go
import (
    "context"
    "github.com/PauloHFS/elenchus/internal/service"
)

// Criar configuração a partir de variáveis de ambiente
config := service.NewGeminiClientConfig()

// Inicializar cliente
client, err := service.NewGeminiClient(config)
if err != nil {
    // Handle error
}

ctx := context.Background()
```

### Geração de Conteúdo (generate_content)

```go
// Texto simples
response, err := client.GenerateContent(ctx, "Explique o que é Go em 2 frases")
if err != nil {
    // Handle error
}
fmt.Println(response)

// Com histórico de conversa
messages := []map[string]string{
    {"role": "user", "content": "Estou aprendendo Go. Por onde começar?"},
    {"role": "assistant", "content": "Comece com variáveis, funções e estruturas de controle."},
    {"role": "user", "content": "Qual o próximo passo?"},
}

response, err = client.GenerateContentWithMessages(ctx, messages)
if err != nil {
    // Handle error
}
fmt.Println(response)
```

### Geração de Embeddings (embed_content)

```go
embedding, err := client.EmbedContent(ctx, "Go é uma linguagem compilada e estaticamente tipada")
if err != nil {
    // Handle error
}

fmt.Printf("Dimensões do embedding: %d\n", len(embedding))
fmt.Printf("Primeiros valores: %v\n", embedding[:10])
```

### Cálculo de Divergência Semântica

```go
text1 := "Go é excelente para backend"
text2 := "Python é ótimo para data science"

emb1, _ := client.EmbedContent(ctx, text1)
emb2, _ := client.EmbedContent(ctx, text2)

divergencia := service.CalculateDivergence(emb1, emb2)
fmt.Printf("Divergência: %.4f (0=idêntico, 1=completamente diferente)\n", divergencia)
```

## Resiliência e Rate Limits

O cliente implementa automaticamente:

### Retry com Exponential Backoff e Jitter

- **Tentativas máximas**: 5
- **Delay base**: 1 segundo
- **Multiplicador**: 2x (exponencial)
- **Delay máximo**: 60 segundos
- **Jitter**: 10% para prevenir "thundering herd"

### Detecção de Rate Limit

Detecta automaticamente erros de rate limit pelos seguintes indicadores:
- HTTP 429
- "Too Many Requests"
- "quota exceeded"
- "RESOURCE_EXHAUSTED"
- "RPM limit exceeded"
- "TPM limit exceeded"
- "RPD limit exceeded"

### Limites do Tier Gratuito

Esteja ciente dos limites da API gratuita:
- **RPM** (Requests per minute)
- **TPM** (Tokens per minute)
- **RPD** (Requests per day)

O cliente lida automaticamente com esses limites através do mecanismo de retry.

## Tratamento de Erros

```go
response, err := client.GenerateContent(ctx, prompt)
if err != nil {
    if strings.Contains(err.Error(), "429") {
        // Rate limit - mesmo com retry, foi excedido
    } else if strings.Contains(err.Error(), "quota") {
        // Cota diária excedida
    } else {
        // Outro erro
    }
}
```

## Health Check

```go
err := client.HealthCheck(ctx)
if err != nil {
    // API indisponível ou problema de autenticação
}
```

## Exemplo Completo

Veja `internal/service/gemini_example.go` para um exemplo completo demonstrando:
1. Geração de texto
2. Conversa com histórico
3. Geração de embeddings
4. Cálculo de divergência
5. Health check

Para executar o exemplo:

```bash
# Configure sua API key primeiro
export GEMINI_API_KEY=sua-api-key

go run -tags example internal/service/gemini_example.go
```

## Testes

Execute os testes unitários:

```bash
go test -v ./internal/service/gemini_client_test.go
```

Para testes de integração (requer API key válida):

```bash
export GEMINI_API_KEY=sua-api-key
go test -v -run TestGeminiClientIntegration ./internal/service/...
```

## Comparação: LM Studio vs Gemini API

| Característica | LM Studio | Gemini API |
|---------------|-----------|------------|
| Execução | Local | Cloud |
| Latência | Baixa (local) | Média (rede) |
| Custo | Hardware | Pay-as-you-go (ou free tier) |
| Rate Limits | Nenhum | RPM/TPM/RPD |
| Modelos | Gerenciado pelo usuário | Gerenciado pelo Google |
| VRAM | Limitada pelo hardware | Ilimitada (cloud) |

## Troubleshooting

### Erro: "GEMINI_API_KEY environment variable is required"

Certifique-se de que a variável de ambiente está configurada:
```bash
export GEMINI_API_KEY=sua-api-key
```

### Erro: "429 Too Many Requests"

O retry automático já está em execução. Se persistir, aguarde alguns minutos antes de novas requisições.

### Erro: "quota exceeded"

Você excedeu a cota diária do free tier. Considere:
- Aguardar até o próximo dia
- Upgrade para um plano pago
- Reduzir o número de requisições

### Embeddings com dimensões inesperadas

Verifique se está usando o modelo correto (`gemini-embedding-001`).

## Referências

- [Google Gemini API Documentation](https://ai.google.dev/gemini-api/docs)
- [Google Gen AI Go SDK](https://pkg.go.dev/google.golang.org/genai)
- [Obter API Key](https://aistudio.google.com/apikey)
- [Limites e Cotas](https://ai.google.dev/gemini-api/docs/rate-limits)

## Políticas de Acesso (Policies)

O projeto inclui políticas de acesso baseadas em atributos (ABAC) para controlar o uso do serviço de avaliação.

### Localização

`internal/policies/evaluation_policy.go`

### Funções Principais

- **`CanAccessEvaluation`**: Verifica se usuário pode acessar uma avaliação
- **`CanCreateEvaluation`**: Verifica se usuário pode criar nova avaliação
- **`CanDeleteEvaluation`**: Verifica se usuário pode deletar avaliação
- **`CanViewAudit`**: Verifica se usuário pode visualizar auditorias
- **`CanViewIteration`**: Verifica se usuário pode visualizar iterações
- **`CheckEvaluationAccess`**: Função genérica com verificação completa
- **`CheckTenantAccess`**: Verifica acesso ao tenant

### Exemplo de Uso

```go
import "github.com/PauloHFS/elenchus/internal/policies"

// Verificar acesso antes de executar avaliação
err := policies.CheckEvaluationAccess(ctx, user, evaluation, policies.ActionView)
if err != nil {
    return http.StatusForbidden, err
}

// Verificar acesso ao tenant
err = policies.CheckTenantAccess(ctx, user, tenantID)
if err != nil {
    return http.StatusForbidden, err
}
```

### Regras de Negócio

1. **Admins** têm acesso total a todos os recursos
2. **Usuários** só podem acessar recursos do seu próprio tenant
3. **Avaliações completadas** não podem ser editadas
4. **Apenas o criador** ou admins podem deletar avaliações
5. **Apenas admins** podem realizar ações de auditoria
