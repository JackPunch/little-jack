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

	// ─── Mode Dispatch ───
	// Tool mode takes priority. If tools are disabled, choose between
	// streaming and non-streaming output.
	var runErr error
	switch {
	case cfg.ToolsEnabled:
		if stream && !debugMode {
			fmt.Println("[Note: tool calling forces non-streaming mode]")
		}
		runErr = runToolAgent(cfg, prompt, systemPrompt, debugMode)
	case stream:
		runErr = runStreamAgent(cfg, prompt, systemPrompt, debugMode)
	default:
		runErr = runNormalAgent(cfg, prompt, systemPrompt, debugMode)
	}

	if runErr != nil {
		log.Fatalf("Agent failed: %v", runErr)
	}
}

// runToolAgent runs the multi-turn tool-calling agent and prints each turn.
func runToolAgent(cfg LLMConfig, prompt, systemPrompt string, debugMode bool) error {
	registry := NewToolRegistry()
	registry.Register(AskUserTool(), AskUserHandler)

	agent := CreateToolAgent(cfg, registry, systemPrompt)
	reader, err := agent(prompt)
	if err != nil {
		return fmt.Errorf("create tool agent: %w", err)
	}
	defer reader.Close()

	for {
		turn, err := reader.ReadTurn()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read turn: %w", err)
		}

		if debugMode {
			continue
		}

		switch turn.Role {
		case "assistant":
			if turn.ReasoningContent != "" {
				fmt.Println("\n---")
				fmt.Println("Reasoning:", turn.ReasoningContent)
				fmt.Println("---")
			}
			if turn.Content != "" {
				fmt.Println("AI:", turn.Content)
			}
			for _, tc := range turn.ToolCalls {
				fmt.Printf("[Calling tool: %s]\n", tc.Function.Name)
			}
			// Only signal the goroutine if there are more turns ahead.
			if !turn.IsFinal {
				reader.ackCh <- struct{}{}
			}
		case "tool":
			fmt.Printf("[Tool result: %s] %s\n", turn.ToolName, turn.Content)
		}
	}

	_, err = reader.Result()
	return err
}

// runStreamAgent runs the streaming agent and prints tokens as they arrive.
func runStreamAgent(cfg LLMConfig, prompt, systemPrompt string, debugMode bool) error {
	streamAgent := CreateStreamingAgent(cfg, systemPrompt)
	reader, err := streamAgent(prompt)
	if err != nil {
		return fmt.Errorf("create stream agent: %w", err)
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
			return fmt.Errorf("read chunk: %w", err)
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

	_, err = reader.Result()
	return err
}

// runNormalAgent runs the simple non-streaming agent and prints the response.
func runNormalAgent(cfg LLMConfig, prompt, systemPrompt string, debugMode bool) error {
	agent := CreateAgent(cfg, systemPrompt)
	resp, err := agent(prompt)
	if err != nil {
		return fmt.Errorf("generate text: %w", err)
	}

	if debugMode {
		return nil
	}

	if resp.ReasoningContent != "" {
		fmt.Println("\n---")
		fmt.Println("Reasoning:", resp.ReasoningContent)
		fmt.Println("---")
	}
	fmt.Println("AI:", resp.Content)
	return nil
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
