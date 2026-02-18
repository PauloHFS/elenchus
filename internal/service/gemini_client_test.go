package service

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestGeminiClientConfig tests the configuration loading
func TestGeminiClientConfig(t *testing.T) {
	// Save original env vars
	originalKey := os.Getenv("GEMINI_API_KEY")
	originalModel := os.Getenv("GEMINI_MODEL_CHAT")
	originalEmbed := os.Getenv("GEMINI_MODEL_EMBEDDING")
	originalTimeout := os.Getenv("GEMINI_TIMEOUT")

	defer func() {
		os.Setenv("GEMINI_API_KEY", originalKey)
		os.Setenv("GEMINI_MODEL_CHAT", originalModel)
		os.Setenv("GEMINI_MODEL_EMBEDDING", originalEmbed)
		os.Setenv("GEMINI_TIMEOUT", originalTimeout)
	}()

	// Test default values
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GEMINI_MODEL_CHAT")
	os.Unsetenv("GEMINI_MODEL_EMBEDDING")
	os.Unsetenv("GEMINI_TIMEOUT")

	config := NewGeminiClientConfig()

	if config.ChatModel != "gemini-2.5-flash" {
		t.Errorf("Expected default chat model 'gemini-2.5-flash', got '%s'", config.ChatModel)
	}

	if config.EmbeddingModel != "gemini-embedding-001" {
		t.Errorf("Expected default embedding model 'gemini-embedding-001', got '%s'", config.EmbeddingModel)
	}

	if config.Timeout != 300*time.Second {
		t.Errorf("Expected default timeout 300s, got %v", config.Timeout)
	}

	// Test custom values
	os.Setenv("GEMINI_API_KEY", "test-key")
	os.Setenv("GEMINI_MODEL_CHAT", "custom-chat-model")
	os.Setenv("GEMINI_MODEL_EMBEDDING", "custom-embed-model")
	os.Setenv("GEMINI_TIMEOUT", "60")

	config = NewGeminiClientConfig()

	if config.APIKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got '%s'", config.APIKey)
	}

	if config.ChatModel != "custom-chat-model" {
		t.Errorf("Expected custom chat model 'custom-chat-model', got '%s'", config.ChatModel)
	}

	if config.EmbeddingModel != "custom-embed-model" {
		t.Errorf("Expected custom embedding model 'custom-embed-model', got '%s'", config.EmbeddingModel)
	}

	if config.Timeout != 60*time.Second {
		t.Errorf("Expected custom timeout 60s, got %v", config.Timeout)
	}
}

// TestGeminiClientWithoutAPIKey tests that client creation fails without API key
func TestGeminiClientWithoutAPIKey(t *testing.T) {
	// Save and clear env var
	originalKey := os.Getenv("GEMINI_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")
	defer os.Setenv("GEMINI_API_KEY", originalKey)

	config := GeminiClientConfig{
		APIKey:         "",
		ChatModel:      "gemini-2.5-flash",
		EmbeddingModel: "gemini-embedding-001",
		Timeout:        300 * time.Second,
	}

	_, err := NewGeminiClient(config)
	if err == nil {
		t.Error("Expected error when creating client without API key, got nil")
	}

	expectedErr := "GEMINI_API_KEY environment variable is required"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

// TestCalculateDivergence tests the divergence calculation function
func TestCalculateDivergence(t *testing.T) {
	tests := []struct {
		name     string
		emb1     []float64
		emb2     []float64
		expected float64
		tolerance float64
	}{
		{
			name:      "identical vectors",
			emb1:      []float64{1, 0, 0},
			emb2:      []float64{1, 0, 0},
			expected:  0.0,
			tolerance: 0.0001,
		},
		{
			name:      "orthogonal vectors",
			emb1:      []float64{1, 0, 0},
			emb2:      []float64{0, 1, 0},
			expected:  1.0,
			tolerance: 0.0001,
		},
		{
			name:      "opposite vectors",
			emb1:      []float64{1, 0, 0},
			emb2:      []float64{-1, 0, 0},
			expected:  1.0, // cosine similarity = -1, divergence = 1 - (-1) = 2, but clamped to 1
			tolerance: 0.0001,
		},
		{
			name:      "empty vectors",
			emb1:      []float64{},
			emb2:      []float64{},
			expected:  1.0,
			tolerance: 0.0001,
		},
		{
			name:      "different lengths",
			emb1:      []float64{1, 0, 0},
			emb2:      []float64{1, 0},
			expected:  1.0,
			tolerance: 0.0001,
		},
		{
			name:      "similar vectors",
			emb1:      []float64{0.9, 0.1, 0.0},
			emb2:      []float64{0.9, 0.1, 0.0},
			expected:  0.0,
			tolerance: 0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateDivergence(tt.emb1, tt.emb2)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("CalculateDivergence(%v, %v) = %v, expected %v (±%v)",
					tt.emb1, tt.emb2, result, tt.expected, tt.tolerance)
			}
		})
	}
}

// TestContainsRateLimitError tests rate limit error detection
func TestContainsRateLimitError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{"HTTP 429", "googleapi: Error 429: Too Many Requests", true},
		{"quota exceeded", "quota exceeded for resource", true},
		{"rate limit", "rate limit exceeded", true},
		{"RESOURCE_EXHAUSTED", "RESOURCE_EXHAUSTED: Quota exceeded", true},
		{"RPM limit", "RPM limit exceeded", true},
		{"TPM limit", "TPM limit exceeded", true},
		{"RPD limit", "RPD limit exceeded", true},
		{"normal error", "connection timeout", false},
		{"invalid request", "invalid request format", false},
		{"empty message", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsRateLimitError(tt.errMsg)
			if result != tt.expected {
				t.Errorf("containsRateLimitError(%q) = %v, expected %v", tt.errMsg, result, tt.expected)
			}
		})
	}
}

// TestContainsIgnoreCase tests case-insensitive substring matching
func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"exact match", "hello world", "world", true},
		{"case mismatch", "HELLO WORLD", "world", true},
		{"partial match", "Hello World", "hello", true},
		{"no match", "hello world", "foo", false},
		{"empty substring", "hello", "", true},
		{"empty string", "", "foo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsIgnoreCase(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

// Integration test - only runs when GEMINI_API_KEY is set
func TestGeminiClientIntegration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" || apiKey == "your-api-key-here" {
		t.Skip("Skipping integration test: GEMINI_API_KEY not set")
	}

	config := GeminiClientConfig{
		APIKey:         apiKey,
		ChatModel:      "gemini-2.5-flash",
		EmbeddingModel: "gemini-embedding-001",
		Timeout:        60 * time.Second,
	}

	client, err := NewGeminiClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Test GenerateContent
	t.Run("GenerateContent", func(t *testing.T) {
		response, err := client.GenerateContent(ctx, "Say hello in one word")
		if err != nil {
			t.Errorf("GenerateContent failed: %v", err)
			return
		}
		if len(response) == 0 {
			t.Error("GenerateContent returned empty response")
		}
		t.Logf("Response: %s", response)
	})

	// Test GenerateContentWithMessages
	t.Run("GenerateContentWithMessages", func(t *testing.T) {
		messages := []map[string]string{
			{"role": "user", "content": "What is 2+2? Answer with just the number."},
		}
		response, err := client.GenerateContentWithMessages(ctx, messages)
		if err != nil {
			t.Errorf("GenerateContentWithMessages failed: %v", err)
			return
		}
		if len(response) == 0 {
			t.Error("GenerateContentWithMessages returned empty response")
		}
		t.Logf("Response: %s", response)
	})

	// Test EmbedContent
	t.Run("EmbedContent", func(t *testing.T) {
		embedding, err := client.EmbedContent(ctx, "Hello, world!")
		if err != nil {
			t.Errorf("EmbedContent failed: %v", err)
			return
		}
		if len(embedding) == 0 {
			t.Error("EmbedContent returned empty embedding")
		}
		t.Logf("Embedding dimensions: %d", len(embedding))
	})

	// Test HealthCheck
	t.Run("HealthCheck", func(t *testing.T) {
		err := client.HealthCheck(ctx)
		if err != nil {
			t.Errorf("HealthCheck failed: %v", err)
		}
	})
}
