package main

import (
	"bufio"
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
// This client uses the OpenAI-compatible API format.
type LLMConfig struct {
	BaseURL         string // e.g., "api.openai.com/v1"
	ModelName       string // e.g., "gpt-4o"
	APIKey          string
	ThinkingType    string // enabled/disabled
	ReasoningEffort string // low/medium/high/max
	DebugMode       bool   // true to print full request/response instead of chat messages
	Stream          bool   // true to enable streaming output; forced false when DebugMode is true
}

// Message represents a chat message.
type Message struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// LLMResponse holds both the final answer and the reasoning chain.
type LLMResponse struct {
	Content          string
	ReasoningContent string
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
		DebugMode:       parseBool(os.Getenv("LLM_DEBUG")),
		Stream:          parseBool(os.Getenv("LLM_STREAM")),
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
// It uses the OpenAI-compatible API format (non-streaming).
func GenerateText(cfg LLMConfig, messages []Message) (LLMResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", normalizeBaseURL(cfg.BaseURL))

	reqBody := openAIRequest{
		Model:    cfg.ModelName,
		Messages: messages,
		Stream:   false,
	}
	if cfg.ThinkingType != "" {
		reqBody.Thinking = &thinkingConfig{Type: cfg.ThinkingType}
	}
	if cfg.ReasoningEffort != "" {
		reqBody.ReasoningEffort = cfg.ReasoningEffort
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return LLMResponse{}, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return LLMResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := doRequest(req, cfg.DebugMode)
	if err != nil {
		return LLMResponse{}, err
	}

	var result openAIResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return LLMResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}
	if result.Error != nil {
		return LLMResponse{}, fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return LLMResponse{}, fmt.Errorf("no choices in response")
	}

	msg := result.Choices[0].Message
	return LLMResponse{
		Content:          msg.Content,
		ReasoningContent: msg.ReasoningContent,
	}, nil
}

// CreateAgent creates a reusable agent function bound to the given config
// and an optional system prompt. The returned function accepts a user prompt
// and returns the model's response including any reasoning content.
// It routes to GenerateTextStream when cfg.Stream is true (and debug mode is off),
// otherwise it uses the non-streaming GenerateText.
func CreateAgent(cfg LLMConfig, systemPrompt ...string) func(string) (LLMResponse, error) {
	return func(prompt string) (LLMResponse, error) {
		messages := make([]Message, 0, len(systemPrompt)+1)
		for _, sp := range systemPrompt {
			if sp != "" {
				messages = append(messages, Message{Role: "system", Content: sp})
			}
		}
		messages = append(messages, Message{Role: "user", Content: prompt})
		if cfg.Stream && !cfg.DebugMode {
			return GenerateTextStream(cfg, messages)
		}
		return GenerateText(cfg, messages)
	}
}

// ─── OpenAI format ───

type openAIRequest struct {
	Model           string          `json:"model"`
	Messages        []Message       `json:"messages"`
	Thinking        *thinkingConfig `json:"thinking,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	Stream          bool            `json:"stream"`
}

type thinkingConfig struct {
	Type string `json:"type"`
}

type openAIStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Index        int     `json:"index"`
		Delta        Message `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type openAIResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// normalizeBaseURL ensures the base URL has a scheme (https by default).
func normalizeBaseURL(base string) string {
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		return base
	}
	return "https://" + base
}

// ─── HTTP helper ───

func doRequest(req *http.Request, debug bool) ([]byte, error) {
	if debug {
		fmt.Fprintf(os.Stderr, "=== API Request ===\n")
		fmt.Fprintf(os.Stderr, "%s %s\n", req.Method, req.URL)
		for k, v := range req.Header {
			fmt.Fprintf(os.Stderr, "Header: %s: %s\n", k, strings.Join(v, ", "))
		}
		if req.Body != nil {
			bodyBytes, _ := io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			fmt.Fprintf(os.Stderr, "Body:\n%s\n", string(bodyBytes))
		}
		fmt.Fprintf(os.Stderr, "===================\n\n")
	}

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

	if debug {
		fmt.Fprintf(os.Stderr, "=== API Response ===\n")
		fmt.Fprintf(os.Stderr, "Status: %s\n", resp.Status)
		for k, v := range resp.Header {
			fmt.Fprintf(os.Stderr, "Header: %s: %s\n", k, strings.Join(v, ", "))
		}
		fmt.Fprintf(os.Stderr, "Body:\n%s\n", string(body))
		fmt.Fprintf(os.Stderr, "====================\n")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GenerateTextStream calls the LLM API in streaming mode.
// It prints tokens to stdout in real time and returns the complete response.
func GenerateTextStream(cfg LLMConfig, messages []Message) (LLMResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", normalizeBaseURL(cfg.BaseURL))

	reqBody := openAIRequest{
		Model:    cfg.ModelName,
		Messages: messages,
		Stream:   true,
	}
	if cfg.ThinkingType != "" {
		reqBody.Thinking = &thinkingConfig{Type: cfg.ThinkingType}
	}
	if cfg.ReasoningEffort != "" {
		reqBody.ReasoningEffort = cfg.ReasoningEffort
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return LLMResponse{}, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return LLMResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return LLMResponse{}, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	reader := bufio.NewReader(resp.Body)
	var result LLMResponse
	var reasoningStarted, contentStarted bool

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return LLMResponse{}, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openAIStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		if delta.ReasoningContent != "" {
			if !reasoningStarted {
				fmt.Println("\n---")
				fmt.Print("Reasoning: ")
				reasoningStarted = true
			}
			fmt.Print(delta.ReasoningContent)
			result.ReasoningContent += delta.ReasoningContent
		}

		if delta.Content != "" {
			if !contentStarted {
				if reasoningStarted {
					fmt.Println("\n---")
				}
				fmt.Print("AI: ")
				contentStarted = true
			}
			fmt.Print(delta.Content)
			result.Content += delta.Content
		}
	}

	if reasoningStarted || contentStarted {
		fmt.Println()
	}

	return result, nil
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "on"
}
