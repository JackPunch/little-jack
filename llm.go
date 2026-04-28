package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LLMConfig holds the configuration for the LLM API client.
type LLMConfig struct {
	Format    string // e.g., "openai" or "anthropic"
	BaseURL   string // e.g., "api.openai.com/v1" or "api.anthropic.com/v1"
	ModelName string // e.g., "gpt-4o" or "claude-3-sonnet-20240229"
	APIKey    string
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LoadConfigFromEnv loads LLM configuration from environment variables.
// Expected env vars: LLM_FORMAT, LLM_BASE_URL, LLM_MODEL_NAME, LLM_API_KEY
func LoadConfigFromEnv() (LLMConfig, error) {
	cfg := LLMConfig{
		Format:    os.Getenv("LLM_FORMAT"),
		BaseURL:   os.Getenv("LLM_BASE_URL"),
		ModelName: os.Getenv("LLM_MODEL_NAME"),
		APIKey:    os.Getenv("LLM_API_KEY"),
	}

	if cfg.Format == "" {
		cfg.Format = "openai" // default
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

// GenerateText calls the LLM API with the given context messages.
// It automatically selects the correct API format based on cfg.Format.
func GenerateText(cfg LLMConfig, messages []Message) (string, error) {
	switch cfg.Format {
	case "openai":
		return generateTextOpenAI(cfg, messages)
	case "anthropic":
		return generateTextAnthropic(cfg, messages)
	default:
		return "", fmt.Errorf("unsupported LLM format: %s", cfg.Format)
	}
}

// GenerateTextSimple is a convenience wrapper that takes a single user prompt.
func GenerateTextSimple(cfg LLMConfig, prompt string) (string, error) {
	messages := []Message{
		{Role: "user", Content: prompt},
	}
	return GenerateText(cfg, messages)
}

// ─── OpenAI format ───

type openAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type openAIResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func generateTextOpenAI(cfg LLMConfig, messages []Message) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", normalizeBaseURL(cfg.BaseURL))

	body, err := json.Marshal(openAIRequest{
		Model:    cfg.ModelName,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := doRequest(req)
	if err != nil {
		return "", err
	}

	var result openAIResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

// ─── Anthropic format ───

type anthropicRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func generateTextAnthropic(cfg LLMConfig, messages []Message) (string, error) {
	url := fmt.Sprintf("%s/messages", normalizeBaseURL(cfg.BaseURL))

	body, err := json.Marshal(anthropicRequest{
		Model:     cfg.ModelName,
		Messages:  messages,
		MaxTokens: 4096,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := doRequest(req)
	if err != nil {
		return "", err
	}

	var result anthropicResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("API error (%s): %s", result.Error.Type, result.Error.Message)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return result.Content[0].Text, nil
}

// normalizeBaseURL ensures the base URL has a scheme (https by default).
func normalizeBaseURL(base string) string {
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		return base
	}
	return "https://" + base
}

// ─── HTTP helper ───

func doRequest(req *http.Request) ([]byte, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
