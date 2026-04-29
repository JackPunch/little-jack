package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LLMConfig holds the configuration for the LLM API client.
// This client uses the OpenAI-compatible API format.
type LLMConfig struct {
	BaseURL         string    // e.g., "api.openai.com/v1"
	ModelName       string    // e.g., "gpt-4o"
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

// ReadChunk returns the next chunk from the stream.
// When the stream is exhausted, it returns io.EOF.
func (r *StreamReader) ReadChunk() (StreamChunk, error) {
	chunk, ok := <-r.chunkCh
	if !ok {
		r.once.Do(func() {
			r.err = <-r.errCh
		})
		if r.err != nil {
			return StreamChunk{}, r.err
		}
		return StreamChunk{}, io.EOF
	}
	r.result.Content += chunk.Content
	r.result.ReasoningContent += chunk.ReasoningContent
	return chunk, nil
}

// Result returns the final aggregated response after the stream is exhausted.
func (r *StreamReader) Result() (LLMResponse, error) {
	for {
		_, err := r.ReadChunk()
		if err == io.EOF {
			break
		}
		if err != nil {
			return r.result, err
		}
	}
	return r.result, nil
}

// Close discards any unread chunks.
func (r *StreamReader) Close() error {
	for range r.chunkCh {
	}
	return nil
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

func doRequest(req *http.Request, debugOut io.Writer) ([]byte, error) {
	if debugOut != nil {
		fmt.Fprintf(debugOut, "=== API Request ===\n")
		fmt.Fprintf(debugOut, "%s %s\n", req.Method, req.URL)
		for k, v := range req.Header {
			fmt.Fprintf(debugOut, "Header: %s: %s\n", k, strings.Join(v, ", "))
		}
		if req.Body != nil {
			bodyBytes, _ := io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			fmt.Fprintf(debugOut, "Body:\n%s\n", string(bodyBytes))
		}
		fmt.Fprintf(debugOut, "===================\n\n")
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

	if debugOut != nil {
		fmt.Fprintf(debugOut, "=== API Response ===\n")
		fmt.Fprintf(debugOut, "Status: %s\n", resp.Status)
		for k, v := range resp.Header {
			fmt.Fprintf(debugOut, "Header: %s: %s\n", k, strings.Join(v, ", "))
		}
		fmt.Fprintf(debugOut, "Body:\n%s\n", string(body))
		fmt.Fprintf(debugOut, "====================\n")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GenerateTextStream calls the LLM API in streaming mode and returns a StreamReader.
// The reader yields chunks as they arrive from the API via ReadChunk.
// Errors during the initial request are returned immediately; errors that occur
// while reading the stream are surfaced through ReadChunk.
func GenerateTextStream(cfg LLMConfig, messages []Message) (*StreamReader, error) {
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
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	if cfg.DebugOutput != nil {
		fmt.Fprintf(cfg.DebugOutput, "=== API Request ===\n")
		fmt.Fprintf(cfg.DebugOutput, "%s %s\n", req.Method, req.URL)
		for k, v := range req.Header {
			fmt.Fprintf(cfg.DebugOutput, "Header: %s: %s\n", k, strings.Join(v, ", "))
		}
		fmt.Fprintf(cfg.DebugOutput, "Body:\n%s\n", string(body))
		fmt.Fprintf(cfg.DebugOutput, "===================\n\n")
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if cfg.DebugOutput != nil {
			fmt.Fprintf(cfg.DebugOutput, "=== API Response ===\nStatus: %s\nBody:\n%s\n====================\n", resp.Status, string(bodyBytes))
		}
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	chunkCh := make(chan StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		defer resp.Body.Close()
		defer close(chunkCh)

		if cfg.DebugOutput != nil {
			fmt.Fprintf(cfg.DebugOutput, "=== API Response ===\n")
			fmt.Fprintf(cfg.DebugOutput, "Status: %s\n", resp.Status)
			for k, v := range resp.Header {
				fmt.Fprintf(cfg.DebugOutput, "Header: %s: %s\n", k, strings.Join(v, ", "))
			}
			fmt.Fprintf(cfg.DebugOutput, "Body:\n")
		}

		reader := bufio.NewReader(resp.Body)
		var streamErr error
		defer func() {
			errCh <- streamErr
			close(errCh)
		}()

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				streamErr = err
				return
			}

			if cfg.DebugOutput != nil {
				fmt.Fprintf(cfg.DebugOutput, "%s", line)
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
			if delta.Content != "" || delta.ReasoningContent != "" {
				chunkCh <- StreamChunk{
					Content:          delta.Content,
					ReasoningContent: delta.ReasoningContent,
				}
			}
		}

		if cfg.DebugOutput != nil {
			fmt.Fprintf(cfg.DebugOutput, "====================\n")
		}
	}()

	return &StreamReader{
		chunkCh: chunkCh,
		errCh:   errCh,
	}, nil
}

