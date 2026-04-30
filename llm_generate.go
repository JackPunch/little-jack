package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ─── HTTP Helpers ───

// normalizeBaseURL ensures the base URL has a scheme (https by default).
func normalizeBaseURL(base string) string {
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		return base
	}
	return "https://" + base
}

// doRequest performs an HTTP request with a 120-second timeout and returns
// the response body. When debugOut is non-nil, it logs the full request and
// response. Non-200 status codes are returned as errors.
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
			fmt.Fprintln(debugOut, "Body:")
			writeDebugBody(debugOut, bodyBytes)
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
		fmt.Fprintln(debugOut, "Body:")
		writeDebugBody(debugOut, body)
		fmt.Fprintf(debugOut, "====================\n")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// writeDebugBody writes a payload to debug output. If it is valid JSON,
// it is indented for readability; otherwise it is written as-is.
func writeDebugBody(w io.Writer, body []byte) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err == nil {
		fmt.Fprintf(w, "%s\n", buf.String())
	} else {
		fmt.Fprintf(w, "%s\n", string(body))
	}
}

// writeDebugSSE writes an SSE data line to debug output. If the payload
// after "data: " is valid JSON, it prints "data:" on its own line followed
// by the indented JSON; otherwise it writes the line as-is.
func writeDebugSSE(w io.Writer, line string) {
	data := strings.TrimPrefix(line, "data: ")
	if data == line { // not a data: line
		fmt.Fprintf(w, "%s", line)
		return
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(data), "", "  "); err == nil {
		fmt.Fprintf(w, "data:\n%s\n", buf.String())
	} else {
		fmt.Fprintf(w, "%s", line)
	}
}

// ─── Non-Streaming API ───

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

// ─── Streaming API ───

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
		fmt.Fprintln(cfg.DebugOutput, "Body:")
		writeDebugBody(cfg.DebugOutput, body)
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
			fmt.Fprintf(cfg.DebugOutput, "=== API Response ===\nStatus: %s\n", resp.Status)
			fmt.Fprintln(cfg.DebugOutput, "Body:")
			writeDebugBody(cfg.DebugOutput, bodyBytes)
			fmt.Fprintln(cfg.DebugOutput, "====================")
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
				writeDebugSSE(cfg.DebugOutput, line)
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

// ─── Agent Constructors ───

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

// CreateToolAgent creates a reusable agent that supports multi-turn tool calling.
// The returned function accepts a user prompt and returns a ToolCallReader
// that yields one assistant turn at a time.
//
// The caller is responsible for consuming all turns (via ReadTurn) and for
// all console output. The agent layer performs no direct I/O.
//
// Reasoning content is preserved and re-sent on every turn, which is required
// by DeepSeek's API.
func CreateToolAgent(cfg LLMConfig, registry *ToolRegistry, systemPrompt ...string) func(string) (*ToolCallReader, error) {
	return func(prompt string) (*ToolCallReader, error) {
		messages := make([]Message, 0, len(systemPrompt)+1)
		for _, sp := range systemPrompt {
			if sp != "" {
				messages = append(messages, Message{Role: "system", Content: sp})
			}
		}
		messages = append(messages, Message{Role: "user", Content: prompt})

		turnCh := make(chan Turn)
		ackCh := make(chan struct{})
		errCh := make(chan error, 1)

		reader := &ToolCallReader{
			turnCh: turnCh,
			ackCh:  ackCh,
			errCh:  errCh,
		}

		go func() {
			defer close(turnCh)
			var runErr error
			defer func() {
				errCh <- runErr
				close(errCh)
			}()

			for turnIndex := 0; ; turnIndex++ {
				result, err := generateRaw(cfg, messages, registry.Tools())
				if err != nil {
					runErr = err
					return
				}

				choice := result.Choices[0]
				assistantMsg := choice.Message

				// Preserve the full assistant message (including reasoning_content
				// and tool_calls) for the next API turn. DeepSeek requires
				// reasoning_content to be replayed verbatim.
				messages = append(messages, assistantMsg)

				isFinal := choice.FinishReason == nil || *choice.FinishReason != "tool_calls"

				turnCh <- Turn{
					Role:             "assistant",
					ReasoningContent: assistantMsg.ReasoningContent,
					Content:          assistantMsg.Content,
					ToolCalls:        assistantMsg.ToolCalls,
					IsFinal:          isFinal,
				}

				if isFinal {
					reader.result = LLMResponse{
						Content:          assistantMsg.Content,
						ReasoningContent: assistantMsg.ReasoningContent,
					}
					return
				}

				// Wait for the caller to finish printing before executing tools.
				if cfg.DebugOutput == nil {
					<-ackCh
				}

				// Execute every tool call requested by the model.
				for _, tc := range assistantMsg.ToolCalls {
					output, err := registry.Execute(tc)
					if err != nil {
						output = fmt.Sprintf("Error: %v", err)
					}
					turnCh <- Turn{
						Role:       "tool",
						Content:    output,
						ToolCallID: tc.ID,
						ToolName:   tc.Function.Name,
					}
					messages = append(messages, Message{
						Role:       "tool",
						Content:    output,
						ToolCallID: tc.ID,
					})
				}
			}
		}()

		return reader, nil
	}
}

// ─── ToolCallReader Methods ───

// ReadTurn returns the next turn from the tool-calling conversation.
// When all turns are consumed, it returns io.EOF.
func (r *ToolCallReader) ReadTurn() (Turn, error) {
	turn, ok := <-r.turnCh
	if !ok {
		r.once.Do(func() {
			r.err = <-r.errCh
		})
		if r.err != nil {
			return Turn{}, r.err
		}
		return Turn{}, io.EOF
	}
	return turn, nil
}

// Result returns the final aggregated response after all turns are consumed.
func (r *ToolCallReader) Result() (LLMResponse, error) {
	for {
		_, err := r.ReadTurn()
		if err == io.EOF {
			break
		}
		if err != nil {
			return r.result, err
		}
	}
	return r.result, nil
}

// Close discards any unread turns.
func (r *ToolCallReader) Close() error {
	for range r.turnCh {
	}
	return nil
}
