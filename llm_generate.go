package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

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

	resp, err := doRequest(req, cfg.DebugOutput)
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
// and returns the model's complete response.
func CreateAgent(cfg LLMConfig, systemPrompt ...string) func(string) (LLMResponse, error) {
	return func(prompt string) (LLMResponse, error) {
		messages := make([]Message, 0, len(systemPrompt)+1)
		for _, sp := range systemPrompt {
			if sp != "" {
				messages = append(messages, Message{Role: "system", Content: sp})
			}
		}
		messages = append(messages, Message{Role: "user", Content: prompt})
		return GenerateText(cfg, messages)
	}
}

// CreateStreamingAgent creates a reusable streaming agent function.
// The returned function accepts a user prompt and returns a StreamReader
// that yields the content stream.
func CreateStreamingAgent(cfg LLMConfig, systemPrompt ...string) func(string) (*StreamReader, error) {
	return func(prompt string) (*StreamReader, error) {
		messages := make([]Message, 0, len(systemPrompt)+1)
		for _, sp := range systemPrompt {
			if sp != "" {
				messages = append(messages, Message{Role: "system", Content: sp})
			}
		}
		messages = append(messages, Message{Role: "user", Content: prompt})
		return GenerateTextStream(cfg, messages)
	}
}
