package main

import (
	"fmt"
	"io"
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

	if !debugMode {
		fmt.Println("User:", prompt)
	}

	systemPrompt := "You are a helpful assistant."
	if cfg.ToolsEnabled {
		systemPrompt = "You are a helpful assistant with access to tools. When you need more information, clarification, or confirmation from the user to complete a task, use the ask_user tool instead of guessing or making assumptions. After receiving the user's answer, proceed to complete the task."
	}

	if cfg.ToolsEnabled {
		if stream && !debugMode {
			fmt.Println("[Note: tool calling forces non-streaming mode]")
		}

		registry := NewToolRegistry()
		registry.Register(AskUserTool(), AskUserHandler)

		agent := CreateToolAgent(cfg, registry, systemPrompt)
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
	} else if stream {
		streamAgent := CreateStreamingAgent(cfg, systemPrompt)
		reader, err := streamAgent(prompt)
		if err != nil {
			log.Fatalf("Failed to generate text: %v", err)
		}
		defer reader.Close()

		var contentBuilder, reasoningBuilder strings.Builder
		var printedAI, printedReasoning bool

		for {
			chunk, err := reader.ReadChunk()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Stream read failed: %v", err)
			}

			if chunk.ReasoningContent != "" {
				reasoningBuilder.WriteString(chunk.ReasoningContent)
				if !debugMode && !printedReasoning {
					fmt.Print("Reasoning: ")
					printedReasoning = true
				}
				if !debugMode {
					fmt.Print(chunk.ReasoningContent)
				}
			}

			if chunk.Content != "" {
				contentBuilder.WriteString(chunk.Content)
				if !debugMode && !printedAI {
					if printedReasoning {
						fmt.Println()
					}
					fmt.Print("AI: ")
					printedAI = true
				}
				if !debugMode {
					fmt.Print(chunk.Content)
				}
			}
		}

		if !debugMode {
			fmt.Println()
		}

		resp, err := reader.Result()
		if err != nil {
			log.Fatalf("Stream error: %v", err)
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
		ToolsEnabled:    parseBool(os.Getenv("LLM_TOOLS")),
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
