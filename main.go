package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	// Load .env file first (optional, won't override existing env vars).
	if err := LoadDotEnv(".env"); err != nil {
		log.Fatalf("Failed to load .env: %v", err)
	}

	// Load configuration from environment variables.
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Example 1: Simple single prompt
	prompt := "Hello, introduce yourself in one sentence."
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}

	fmt.Println("User:", prompt)
	resp, err := GenerateTextSimple(cfg, prompt)
	if err != nil {
		log.Fatalf("Failed to generate text: %v", err)
	}
	fmt.Println("AI:", resp)

	// Example 2: Multi-turn context
	fmt.Println("\n--- Multi-turn example ---")
	messages := []Message{
		{Role: "system", Content: "You are a helpful Go programming assistant."},
		{Role: "user", Content: "How do I read an environment variable in Go?"},
	}
	resp2, err := GenerateText(cfg, messages)
	if err != nil {
		log.Fatalf("Failed to generate text: %v", err)
	}
	fmt.Println("AI:", resp2)
}
