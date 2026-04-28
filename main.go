package main

import (
	"fmt"
	"log"
	"os"
	"strings"
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

	prompt := "Hello, introduce yourself in one sentence."
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}

	stream := parseBool(os.Getenv("LLM_STREAM"))
	debugMode := cfg.DebugOutput != nil
	if debugMode {
		stream = false
	}

	if !debugMode {
		fmt.Println("User:", prompt)
	}

	systemPrompt := "You are a helpful assistant."

	if stream {
		streamAgent := CreateStreamingAgent(cfg, systemPrompt)
		var reasoningStarted, contentStarted bool
		resp, err := streamAgent(prompt, func(chunk StreamChunk) {
			if chunk.ReasoningContent != "" {
				if !reasoningStarted {
					fmt.Println("\n---")
					fmt.Print("Reasoning: ")
					reasoningStarted = true
				}
				fmt.Print(chunk.ReasoningContent)
			}
			if chunk.Content != "" {
				if !contentStarted {
					if reasoningStarted {
						fmt.Println("\n---")
					}
					fmt.Print("AI: ")
					contentStarted = true
				}
				fmt.Print(chunk.Content)
			}
		})
		if err != nil {
			log.Fatalf("Failed to generate text: %v", err)
		}
		if reasoningStarted || contentStarted {
			fmt.Println()
		}
		_ = resp
	} else {
		agent := CreateAgent(cfg, systemPrompt)
		resp, err := agent(prompt)
		if err != nil {
			log.Fatalf("Failed to generate text: %v", err)
		}
		if !debugMode {
			if resp.ReasoningContent != "" {
				fmt.Println("\n---")
				fmt.Println("Reasoning:", resp.ReasoningContent)
				fmt.Println("---")
			}
			fmt.Println("AI:", resp.Content)
		}
	}
}

// LoadConfigFromEnv loads LLM configuration from environment variables.
// Expected env vars: LLM_BASE_URL, LLM_MODEL_NAME, LLM_API_KEY
func LoadConfigFromEnv() (LLMConfig, error) {
	cfg := LLMConfig{
		BaseURL:         os.Getenv("LLM_BASE_URL"),
		ModelName:       os.Getenv("LLM_MODEL_NAME"),
		APIKey:          os.Getenv("LLM_API_KEY"),
		ThinkingType:    os.Getenv("LLM_THINKING_TYPE"),
		ReasoningEffort: os.Getenv("LLM_REASONING_EFFORT"),
	}

	if parseBool(os.Getenv("LLM_DEBUG")) {
		cfg.DebugOutput = os.Stderr
	}

	if cfg.BaseURL == "" {
		return LLMConfig{}, fmt.Errorf("missing required env var: LLM_BASE_URL")
	}
	if cfg.ModelName == "" {
		return LLMConfig{}, fmt.Errorf("missing required env var: LLM_MODEL_NAME")
	}
	if cfg.APIKey == "" {
		return LLMConfig{}, fmt.Errorf("missing required env var: LLM_API_KEY")
	}

	return cfg, nil
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "on"
}
