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
	result, err := generateRaw(cfg, messages, nil)
	if err != nil {
		return LLMResponse{}, err
	}
	msg := result.Choices[0].Message
	return LLMResponse{
		Content:          msg.Content,
		ReasoningContent: msg.ReasoningContent,
	}, nil
}

// generateRaw sends a non-streaming request and returns the raw API response.
func generateRaw(cfg LLMConfig, messages []Message, tools []Tool) (openAIResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", normalizeBaseURL(cfg.BaseURL))

	reqBody := openAIRequest{
		Model:    cfg.ModelName,
		Messages: messages,
		Tools:    tools,
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
		return openAIResponse{}, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return openAIResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := doRequest(req, cfg.DebugOutput)
	if err != nil {
		return openAIResponse{}, err
	}

	var result openAIResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return openAIResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}
	if result.Error != nil {
		return openAIResponse{}, fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return openAIResponse{}, fmt.Errorf("no choices in response")
	}

	return result, nil
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

// CreateToolAgent creates a non-streaming agent that supports tool calling.
// It runs a loop: sends the prompt (with available tools), checks if the model
// wants to call tools, executes them, and sends the results back until the
// model produces a final answer.
//
// Reasoning content is preserved and re-sent on every turn, which is required
// by DeepSeek's API.
func CreateToolAgent(cfg LLMConfig, registry *ToolRegistry, systemPrompt ...string) func(string) (LLMResponse, error) {
	return func(prompt string) (LLMResponse, error) {
		messages := make([]Message, 0, len(systemPrompt)+1)
		for _, sp := range systemPrompt {
			if sp != "" {
				messages = append(messages, Message{Role: "system", Content: sp})
			}
		}
		messages = append(messages, Message{Role: "user", Content: prompt})

		for {
			result, err := generateRaw(cfg, messages, registry.Tools())
			if err != nil {
				return LLMResponse{}, err
			}

			choice := result.Choices[0]
			assistantMsg := choice.Message

			// Preserve the full assistant message (including reasoning_content
			// and tool_calls) for the next API turn. DeepSeek requires
			// reasoning_content to be replayed verbatim.
			messages = append(messages, assistantMsg)

			if choice.FinishReason == nil || *choice.FinishReason != "tool_calls" {
				return LLMResponse{
					Content:          assistantMsg.Content,
					ReasoningContent: assistantMsg.ReasoningContent,
				}, nil
			}

			// Print reasoning and content before calling tools.
			if cfg.DebugOutput == nil {
				if assistantMsg.ReasoningContent != "" {
					fmt.Println("\n---")
					fmt.Println("Reasoning:", assistantMsg.ReasoningContent)
					fmt.Println("---")
				}
				if assistantMsg.Content != "" {
					fmt.Println("AI:", assistantMsg.Content)
				}
			}

			// Execute every tool call requested by the model.
			for _, tc := range assistantMsg.ToolCalls {
				output, err := registry.Execute(tc)
				if err != nil {
					output = fmt.Sprintf("Error: %v", err)
				}
				messages = append(messages, Message{
					Role:       "tool",
					Content:    output,
					ToolCallID: tc.ID,
				})
			}
		}
	}
}
