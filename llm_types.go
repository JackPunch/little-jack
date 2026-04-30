package main

import (
	"io"
	"sync"
)

// ─── Configuration ───

// LLMConfig holds the configuration for the LLM API client.
// This client uses the OpenAI-compatible API format.
type LLMConfig struct {
	BaseURL         string // e.g., "api.openai.com/v1"
	ModelName       string // e.g., "gpt-4o"
	APIKey          string
	ThinkingType    string    // enabled/disabled
	ReasoningEffort string    // low/medium/high/max
	ToolsEnabled    bool      // enable tool calling
	DebugOutput     io.Writer // non-nil enables HTTP request/response logging
}

// ─── Core Types ───

// Message represents a chat message.
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

// LLMResponse holds both the final answer and the reasoning chain.
type LLMResponse struct {
	Content          string
	ReasoningContent string
}

// ─── Streaming ───

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

// ─── OpenAI Format ───

// openAIRequest is the request body sent to the OpenAI-compatible API.
type openAIRequest struct {
	Model           string          `json:"model"`
	Messages        []Message       `json:"messages"`
	Tools           []Tool          `json:"tools,omitempty"`
	Thinking        *thinkingConfig `json:"thinking,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	Stream          bool            `json:"stream"`
}

// thinkingConfig controls whether the model shows its reasoning.
type thinkingConfig struct {
	Type string `json:"type"`
}

// openAIStreamResponse is the SSE response format for streaming requests.
type openAIStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Index        int     `json:"index"`
		Delta        Message `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// openAIResponse is the non-streaming response format.
type openAIResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ─── Tool Calling ───

// Turn represents one message in a multi-turn tool-calling conversation.
// It can be from the assistant or from a tool execution result.
type Turn struct {
	Role             string     // "assistant" or "tool"
	Content          string     // assistant content or tool output
	ReasoningContent string     // only for assistant
	ToolCalls        []ToolCall // only for assistant
	ToolCallID       string     // only for tool
	ToolName         string     // only for tool
	IsFinal          bool       // only for assistant: true if this is the last turn
}

// ToolCallReader reads multi-turn tool-calling progress turn by turn.
// Use ReadTurn to iterate over each turn (assistant or tool).
// After all turns are consumed, call Result() to get the aggregated final response.
type ToolCallReader struct {
	turnCh <-chan Turn
	ackCh  chan<- struct{}
	errCh  <-chan error
	once   sync.Once
	err    error
	result LLMResponse
}

// Tool describes a function that the model may call.
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function describes the signature of a callable tool.
type Function struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

// Parameters describes the JSON Schema for a function's arguments.
type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property describes a single parameter field.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolCall represents a single tool invocation requested by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
