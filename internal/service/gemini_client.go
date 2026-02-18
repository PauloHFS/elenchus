package service

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"google.golang.org/genai"
)

const (
	// Default Gemini models
	defaultGeminiChatModel      = "gemini-2.5-flash"
	defaultGeminiEmbeddingModel = "gemini-embedding-001"

	// Retry configuration
	maxRetries        = 5
	baseRetryDelay    = 1 * time.Second
	maxRetryDelay     = 60 * time.Second
	retryMultiplier   = 2.0
	retryJitterFactor = 0.1
)

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		fmt.Sscanf(value, "%d", &result)
		if result > 0 {
			return result
		}
	}
	return defaultValue
}

// GeminiClientConfig holds configuration for the Gemini client
type GeminiClientConfig struct {
	APIKey         string
	ChatModel      string
	EmbeddingModel string
	Timeout        time.Duration
}

// GeminiClient manages communication with Google Gemini API
type GeminiClient struct {
	client         *genai.Client
	config         GeminiClientConfig
	chatModel      string
	embeddingModel string
}

// GeminiError represents an error from the Gemini API with rate limit information
type GeminiError struct {
	Err         error
	StatusCode  int
	RetryAfter  time.Duration
	IsRateLimit bool
}

func (e *GeminiError) Error() string {
	if e.IsRateLimit {
		return fmt.Sprintf("rate limit exceeded: %v", e.Err)
	}
	return fmt.Sprintf("gemini API error: %v", e.Err)
}

func (e *GeminiError) Unwrap() error {
	return e.Err
}

// NewGeminiClientConfig creates a configuration from environment variables
func NewGeminiClientConfig() GeminiClientConfig {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}

	timeout := time.Duration(getEnvInt("GEMINI_TIMEOUT", 300)) * time.Second

	return GeminiClientConfig{
		APIKey:         apiKey,
		ChatModel:      getEnv("GEMINI_MODEL_CHAT", defaultGeminiChatModel),
		EmbeddingModel: getEnv("GEMINI_MODEL_EMBEDDING", defaultGeminiEmbeddingModel),
		Timeout:        timeout,
	}
}

// NewGeminiClient creates a new Gemini client with the given configuration
func NewGeminiClient(config GeminiClientConfig) (*GeminiClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  config.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiClient{
		client:         client,
		config:         config,
		chatModel:      config.ChatModel,
		embeddingModel: config.EmbeddingModel,
	}, nil
}

// GenerateContent generates text content using the Gemini chat model
func (c *GeminiClient) GenerateContent(ctx context.Context, prompt string) (string, error) {
	var result string
	err := c.withRetry(ctx, func(ctx context.Context) error {
		resp, err := c.client.Models.GenerateContent(ctx, c.chatModel, genai.Text(prompt), &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(float32(0.0)),
			MaxOutputTokens: 8192,
		})
		if err != nil {
			return err
		}

		result = resp.Text()
		if result == "" {
			return fmt.Errorf("no content generated")
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	return result, nil
}

// GenerateContentWithMessages generates content using a conversation history
func (c *GeminiClient) GenerateContentWithMessages(ctx context.Context, messages []map[string]string) (string, error) {
	var result string
	err := c.withRetry(ctx, func(ctx context.Context) error {
		// Convert messages to Gemini format
		var contents []*genai.Content
		for _, msg := range messages {
			role := msg["role"]
			content := msg["content"]

			geminiRole := genai.RoleUser
			if role == "assistant" {
				geminiRole = genai.RoleModel
			}

			contents = append(contents, &genai.Content{
				Role: geminiRole,
				Parts: []*genai.Part{
					{Text: content},
				},
			})
		}

		resp, err := c.client.Models.GenerateContent(ctx, c.chatModel, contents, &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(float32(0.0)),
			MaxOutputTokens: 8192,
		})
		if err != nil {
			return err
		}

		result = resp.Text()
		if result == "" {
			return fmt.Errorf("no content generated")
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	return result, nil
}

// EmbedContent generates embeddings for the given text
func (c *GeminiClient) EmbedContent(ctx context.Context, text string) ([]float64, error) {
	var embedding []float64

	err := c.withRetry(ctx, func(ctx context.Context) error {
		resp, err := c.client.Models.EmbedContent(ctx, c.embeddingModel, genai.Text(text), nil)
		if err != nil {
			return err
		}

		// The new SDK returns Embeddings (plural) array
		if len(resp.Embeddings) == 0 || resp.Embeddings[0] == nil || len(resp.Embeddings[0].Values) == 0 {
			return fmt.Errorf("no embedding generated")
		}

		// Convert float32 to float64
		embedding = make([]float64, len(resp.Embeddings[0].Values))
		for i, v := range resp.Embeddings[0].Values {
			embedding[i] = float64(v)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return embedding, nil
}

// withRetry executes a function with exponential backoff and jitter for rate limits
func (c *GeminiClient) withRetry(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	delay := baseRetryDelay

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if it's a rate limit error
		isRateLimit := false

		// Check for HTTP 429 or quota exceeded errors
		if err.Error() != "" {
			isRateLimit = containsRateLimitError(err.Error())
		}

		if !isRateLimit {
			// For non-rate-limit errors, return immediately
			return err
		}

		// Apply exponential backoff with jitter
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * retryMultiplier)
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}
			// Add jitter
			jitter := time.Duration(float64(delay) * retryJitterFactor * (rand.Float64() - 0.5) * 2)
			delay += jitter
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// containsRateLimitError checks if an error message indicates a rate limit issue
func containsRateLimitError(errMsg string) bool {
	rateLimitIndicators := []string{
		"429",
		"Too Many Requests",
		"quota exceeded",
		"rate limit",
		"RESOURCE_EXHAUSTED",
		"RPM",
		"TPM",
		"RPD",
	}

	for _, indicator := range rateLimitIndicators {
		if containsIgnoreCase(errMsg, indicator) {
			return true
		}
	}
	return false
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && contains(s, substr)
}

// contains is a helper function for substring search
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sChar := s[i+j]
			subChar := substr[j]
			// Case-insensitive comparison for ASCII
			if sChar >= 'A' && sChar <= 'Z' {
				sChar += 32
			}
			if subChar >= 'A' && subChar <= 'Z' {
				subChar += 32
			}
			if sChar != subChar {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// CalculateDivergence calculates the cosine divergence between two embeddings
func CalculateDivergence(emb1, emb2 []float64) float64 {
	if len(emb1) == 0 || len(emb2) == 0 || len(emb1) != len(emb2) {
		return 1.0
	}

	var dotProduct, mag1, mag2 float64
	for i := range emb1 {
		dotProduct += emb1[i] * emb2[i]
		mag1 += emb1[i] * emb1[i]
		mag2 += emb2[i] * emb2[i]
	}

	if mag1 == 0 || mag2 == 0 {
		return 0.0
	}

	similarity := dotProduct / (math.Sqrt(mag1) * math.Sqrt(mag2))
	divergence := 1.0 - similarity

	// Clamp to [0, 1] range
	if divergence < 0 {
		divergence = 0
	}
	if divergence > 1 {
		divergence = 1
	}

	return divergence
}

// HealthCheck verifies the Gemini API connection
func (c *GeminiClient) HealthCheck(ctx context.Context) error {
	_, err := c.GenerateContent(ctx, "Hello")
	return err
}
