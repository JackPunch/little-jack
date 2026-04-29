package main

import (
	"io"
	"sync"
)

// LLMConfig holds the configuration for the LLM API client.
// This client uses the OpenAI-compatible API format.
type LLMConfig struct {
	BaseURL         string // e.g., "api.openai.com/v1"
	ModelName       string // e.g., "gpt-4o"
	APIKey          string
	ThinkingType    string    // enabled/disabled
	ReasoningEffort string    // low/medium/high/max
	DebugOutput     io.Writer // non-nil enables HTTP request/response logging
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

// StreamChunk represents a piece of the streaming response.
type StreamChunk struct {
	Content          string
	ReasoningContent string
}

// StreamReader reads streaming chunks from the LLM.
// Use ReadChunk to iterate over individual chunks.
// After the stream is fully consumed, call Result() to get the aggregated response.
type StreamReader struct {
	chunkCh <-chan StreamChunk
	errCh   <-chan error
	once    sync.Once
	err     error
	result  LLMResponse
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

