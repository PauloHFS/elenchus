//go:build example
// +build example

package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Example usage of the Gemini Client
// Run with: go run -tags example internal/service/gemini_example.go
func main() {
	// Load environment variables from .env file (optional, for local development)
	_ = godotenv.Load()

	// Check if API key is configured
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}

	if apiKey == "" || apiKey == "your-api-key-here" {
		fmt.Println("❌ Error: GEMINI_API_KEY environment variable is not set")
		fmt.Println("   Get your API key from: https://aistudio.google.com/apikey")
		fmt.Println("   Then set it: export GEMINI_API_KEY=your-api-key-here")
		os.Exit(1)
	}

	// Create client configuration
	config := NewGeminiClientConfig()

	// Initialize the Gemini client
	client, err := NewGeminiClient(config)
	if err != nil {
		fmt.Printf("❌ Failed to create Gemini client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	fmt.Println("=== Google Gemini API Integration Example ===\n")

	// Example 1: Text Generation (generate_content)
	fmt.Println("1. Text Generation with gemini-2.5-flash")
	fmt.Println("   Prompt: 'Explain what is Go programming language in 2 sentences'")
	fmt.Println()

	startTime := time.Now()
	response, err := client.GenerateContent(ctx, "Explain what is Go programming language in 2 sentences")
	elapsed := time.Since(startTime)

	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ⏱️  Response time: %v\n", elapsed)
		fmt.Printf("   ✅ Response:\n")
		fmt.Printf("      %s\n\n", response)
	}

	// Example 2: Conversation with Message History
	fmt.Println("2. Conversation with Message History")
	fmt.Println()

	messages := []map[string]string{
		{"role": "user", "content": "I'm learning Go. What should I start with?"},
		{"role": "assistant", "content": "Start with the basics: variables, functions, and control structures. Go has a simple syntax that makes it easy to learn."},
		{"role": "user", "content": "What's the next step after learning the basics?"},
	}

	response, err = client.GenerateContentWithMessages(ctx, messages)
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ✅ Response:\n")
		fmt.Printf("      %s\n\n", response)
	}

	// Example 3: Text Embedding (embed_content)
	fmt.Println("3. Text Embedding with gemini-embedding-001")
	fmt.Println("   Text: 'Go is a statically typed, compiled programming language'")
	fmt.Println()

	startTime = time.Now()
	embedding, err := client.EmbedContent(ctx, "Go is a statically typed, compiled programming language")
	elapsed = time.Since(startTime)

	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ⏱️  Response time: %v\n", elapsed)
		fmt.Printf("   ✅ Embedding dimensions: %d\n", len(embedding))
		fmt.Printf("   ✅ First 10 values: %v\n\n", embedding[:10])
	}

	// Example 4: Calculate Divergence between Two Texts
	fmt.Println("4. Semantic Divergence Calculation")
	fmt.Println()

	text1 := "Go is great for backend development"
	text2 := "Python is excellent for data science"

	emb1, err := client.EmbedContent(ctx, text1)
	if err != nil {
		fmt.Printf("   ❌ Error embedding text1: %v\n", err)
	} else {
		emb2, err := client.EmbedContent(ctx, text2)
		if err != nil {
			fmt.Printf("   ❌ Error embedding text2: %v\n", err)
		} else {
			divergence := CalculateDivergence(emb1, emb2)
			fmt.Printf("   Text 1: \"%s\"\n", text1)
			fmt.Printf("   Text 2: \"%s\"\n", text2)
			fmt.Printf("   📊 Divergence: %.4f (0 = identical, 1 = completely different)\n\n", divergence)
		}
	}

	// Example 5: Rate Limit Handling Demonstration
	fmt.Println("5. Rate Limit Handling")
	fmt.Println("   The client automatically handles HTTP 429 errors with:")
	fmt.Println("   - Exponential backoff (base: 1s, multiplier: 2x, max: 60s)")
	fmt.Println("   - Jitter to prevent thundering herd")
	fmt.Println("   - Maximum 5 retry attempts")
	fmt.Println("   - Detects: 429, 'Too Many Requests', 'quota exceeded', 'RESOURCE_EXHAUSTED'")
	fmt.Println()

	// Example 6: Health Check
	fmt.Println("6. Health Check")
	err = client.HealthCheck(ctx)
	if err != nil {
		fmt.Printf("   ❌ Health check failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Gemini API is accessible\n")
	}

	fmt.Println("\n=== Example Complete ===")
}
