package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	Debug           bool
	Stream          bool
	Tools           bool
	Thinking        bool
	ReasoningEffort string

	BaseURL   string
	ModelName string
	APIKey    string
}

func getConfig() (*Config, error) {
	var config Config

	file, err := os.Open(".env")
	if err != nil {
		return nil, fmt.Errorf("open .env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line0 := scanner.Text()
		line1 := strings.TrimSpace(line0)
		if line1 == "" {
			continue
		}
		if strings.HasPrefix(line1, "#") {
			continue
		}
		parts := strings.SplitN(line1, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "DEBUG":
			config.Debug = value == "true"
		case "STREAM":
			config.Stream = value == "true"
		case "TOOLS":
			config.Tools = value == "true"
		case "THINKING_TYPE":
			config.Thinking = value == "enabled"
		case "REASONING_EFFORT":
			config.ReasoningEffort = value
		case "BASE_URL":
			config.BaseURL = value
		case "MODEL_NAME":
			config.ModelName = value
		case "API_KEY":
			config.APIKey = value
		}
	}
	return &config, nil
}

type Message struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type Thinking struct {
	Type string `json:"type"`
}

type RequestBody struct {
	Messages        []Message `json:"messages"`
	Model           string    `json:"model"`
	Thinking        *Thinking `json:"thinking,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
	Stream          bool      `json:"stream,omitempty"`
}

type Choice struct {
	FinishReason string  `json:"finish_reason"`
	Index        int64   `json:"index"`
	Message      Message `json:"message"`
}

type Usage struct {
	CompletionTokens      int64 `json:"completion_tokens"`
	PromptTokens          int64 `json:"prompt_tokens"`
	PromptCacheHitTokens  int64 `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int64 `json:"prompt_cache_miss_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

type ResponseBody struct {
	ID                string   `json:"id"`
	Choices           []Choice `json:"choices"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	SystemFingerprint string   `json:"system_fingerprint"`
	Object            string   `json:"object"`
	Usage             Usage    `json:"usage"`
}

type Agent struct {
	Config *Config
	Client *http.Client
}

func (agent *Agent) Chat(messages []Message) (Message, error) {
	if agent == nil {
		return Message{}, fmt.Errorf("agent is nil")
	}
	if agent.Config == nil {
		return Message{}, fmt.Errorf("agent config is nil")
	}
	if agent.Client == nil {
		return Message{}, fmt.Errorf("agent client is nil")
	}

	var thinkingType string
	if agent.Config.Thinking {
		thinkingType = "enabled"
	} else {
		thinkingType = "disabled"
	}

	body := RequestBody{
		Messages:        messages,
		Model:           agent.Config.ModelName,
		Thinking:        &Thinking{Type: thinkingType},
		ReasoningEffort: agent.Config.ReasoningEffort,
		Stream:          agent.Config.Stream,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return Message{}, fmt.Errorf("marshal request body: %w", err)
	}

	url := agent.Config.BaseURL + "/chat/completions"
	req, err := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return Message{}, fmt.Errorf("create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+agent.Config.APIKey)

	resp, err := agent.Client.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("do HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Message{}, fmt.Errorf("API error %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Message{}, fmt.Errorf("read response body: %w", err)
	}

	var result ResponseBody
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return Message{}, fmt.Errorf("unmarshal response body: %w", err)
	}

	if len(result.Choices) == 0 {
		return Message{}, fmt.Errorf("empty choices in response")
	}
	return result.Choices[0].Message, nil
}

func main() {
	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}
	agent := Agent{
		Config: config,
		Client: &http.Client{
			Timeout: 150 * time.Second,
		},
	}

	messages := []Message{
		{Role: "system", Content: "你是一个ai助手"},
		{Role: "user", Content: "你好"},
	}

	newMessage, err := agent.Chat(messages)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(newMessage.Content)
}
