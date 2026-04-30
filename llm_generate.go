package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
				<-ackCh

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
