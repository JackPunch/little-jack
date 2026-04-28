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

	// Example: Create an agent with an optional system prompt.
	agent := CreateAgent(cfg, "You are a helpful assistant.")

	prompt := "Hello, introduce yourself in one sentence."
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}

	if !cfg.DebugMode {
		fmt.Println("User:", prompt)
	}
	resp, err := agent(prompt)
	if err != nil {
		log.Fatalf("Failed to generate text: %v", err)
	}
	if !cfg.DebugMode && !cfg.Stream {
		if resp.ReasoningContent != "" {
			fmt.Println("\n---")
			fmt.Println("Reasoning:", resp.ReasoningContent)
			fmt.Println("---")
		}
		fmt.Println("AI:", resp.Content)
	}
}
