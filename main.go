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

func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if c.ModelName == "" {
		return fmt.Errorf("model_name is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	return nil
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
			config.Thinking = value != "disabled" //根据 DeepSeek API 文档，thinking.type 为空时默认开启思考模式
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

	err = config.Validate()
	if err != nil {
		return nil, err
	}

	return &config, nil
}

type Message struct {
	Role             Role   `json:"role"`
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

type agent struct {
	config *Config
	client *http.Client
}

func NewAgent(config *Config) (*agent, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	err := config.Validate()
	if err != nil {
		return nil, err
	}

	return &agent{
		config: config,
		client: &http.Client{
			Timeout: 150 * time.Second,
		},
	}, nil
}

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

func (agent *agent) Chat(messages []Message) (Message, error) {
	var thinkingType string
	if agent.config.Thinking {
		thinkingType = "enabled"
	} else {
		thinkingType = "disabled"
	}

	body := RequestBody{
		Messages:        messages,
		Model:           agent.config.ModelName,
		Thinking:        &Thinking{Type: thinkingType},
		ReasoningEffort: agent.config.ReasoningEffort,
		Stream:          agent.config.Stream,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return Message{}, fmt.Errorf("marshal request body: %w", err)
	}

	url := agent.config.BaseURL + "/chat/completions"
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
	req.Header.Set("Authorization", "Bearer "+agent.config.APIKey)

	resp, err := agent.client.Do(req)
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
		log.Fatalf("load config: %v", err)
	}
	agent, err := NewAgent(config)
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}

	messages := []Message{
		{Role: RoleSystem, Content: "你是一个ai助手"},
		{Role: RoleUser, Content: "你好"},
	}

	newMessage, err := agent.Chat(messages)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(newMessage.Content)
}
