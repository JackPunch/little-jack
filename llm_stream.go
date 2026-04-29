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
