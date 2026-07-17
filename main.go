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

	"gopkg.in/yaml.v3"
)

type Config struct {
	BaseURL         string `yaml:"base_url"`
	ModelName       string `yaml:"model_name"`
	APIKey          string `yaml:"api_key"`
	Thinking        bool   `yaml:"thinking"`
	ReasoningEffort string `yaml:"reasoning_effort"`
	Tools           bool   `yaml:"tools"`
	Stream          bool   `yaml:"stream"` // 暂不支持流式调用
}

const configFile = "config.yaml"

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
	if c.Stream {
		return fmt.Errorf("stream output is not supported")
	}
	return nil
}

func getConfig() (*Config, error) {
	var config Config

	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Please input base url:")
		config.BaseURL, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read base url: %w", err)
		}
		config.BaseURL = strings.TrimSpace(config.BaseURL)
		fmt.Println("Please input model name:")
		config.ModelName, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read model name: %w", err)
		}
		config.ModelName = strings.TrimSpace(config.ModelName)
		fmt.Println("Please input api key:")
		config.APIKey, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read api key: %w", err)
		}
		config.APIKey = strings.TrimSpace(config.APIKey)
		config.Thinking = true

		data, err := yaml.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshal config: %w", err)
		}

		err = os.WriteFile(configFile, data, 0644)
		if err != nil {
			return nil, fmt.Errorf("write %s file: %w", configFile, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("stat %s file: %w", configFile, err)
	} else {
		file, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("read %s file: %w", configFile, err)
		}

		err = yaml.Unmarshal(file, &config)
		if err != nil {
			return nil, fmt.Errorf("unmarshal %s file: %w", configFile, err)
		}
	}

	err = config.Validate()
	if err != nil {
		return nil, err
	}

	return &config, nil
}

type Message struct {
	Role             Role       `json:"role"`
	Content          *string    `json:"content,omitempty"`
	ReasoningContent *string    `json:"reasoning_content,omitempty"`
	ToolCallId       *string    `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	Name             *string    `json:"name,omitempty"`
}

func (m *Message) GetContent() string {
	if m.Content == nil {
		return ""
	}
	return *m.Content
}

func (m *Message) GetReasoningContent() string {
	if m.ReasoningContent == nil {
		return ""
	}
	return *m.ReasoningContent
}

func (m *Message) GetToolCallId() string {
	if m.ToolCallId == nil {
		return ""
	}
	return *m.ToolCallId
}

func NewSystemMessage(content string) Message {
	return Message{
		Role:    RoleSystem,
		Content: &content,
	}
}

func NewUserMessage(content string) Message {
	return Message{
		Role:    RoleUser,
		Content: &content,
	}
}

func NewToolMessage(content string, toolCallId string) Message {
	return Message{
		Role:       RoleTool,
		Content:    &content,
		ToolCallId: &toolCallId,
	}
}

// ToolCall 代表模型生成的具体工具调用指令
type ToolCall struct {
	ID       string       `json:"id"`       // tool 调用的唯一 ID
	Type     string       `json:"type"`     // 目前仅支持 "function"
	Function FunctionCall `json:"function"` // 模型生成的具体函数名与参数
}

// FunctionCall 代表模型具体要调用的函数和生成的参数
type FunctionCall struct {
	Name      string `json:"name"`      // 模型调用的函数名
	Arguments string `json:"arguments"` // 关键：模型生成的参数，格式为 JSON 字符串 (如 "{\"location\": \"Hangzhou\"}")
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
	Tools           []Tool    `json:"tools,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`     // 目前仅支持 "function"
	Function FunctionDesc `json:"function"` // 函数的具体描述
}

// FunctionDesc 描述了具体函数的名称、作用以及参数规范
type FunctionDesc struct {
	Name        string         `json:"name"`                  // 函数名称，必须由 a-zA-Z0-9、下划线和连字符组成，最大长度 64
	Description string         `json:"description,omitempty"` // 函数的功能描述，供模型理解何时以及如何调用
	Parameters  map[string]any `json:"parameters,omitempty"`  // 输入参数，以 JSON Schema 对象描述。如果为空，可省略或传空对象
	Strict      bool           `json:"strict,omitempty"`      // 是否开启 strict 模式以强符合 JSON Schema (Beta 功能)
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

	if agent.config.Tools {
		body.Tools = []Tool{MockTool}
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

var MockTool = Tool{
	Type: "function",
	Function: FunctionDesc{
		Name:        "findNews",
		Description: "Get the latest news from the web",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{
					"type":        "string",
					"description": "The city and state, e.g. San Francisco, CA",
				},
			},
			"required": []string{"location"},
		},
		Strict: false,
	},
}

func findNews(location string) string {
	return "It is very hot in " + location + "!"
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
		NewSystemMessage("You are a helpful assistant."),
	}

	reader := bufio.NewReader(os.Stdin)

	isToolCalling := false

	for {
		if !isToolCalling {
			fmt.Println("User: ")
			line, err := reader.ReadString('\n')
			isEOF := err == io.EOF

			if err != nil && !isEOF {
				log.Printf("read input: %v", err)
				continue
			}

			line = strings.TrimSpace(line)

			if line == "" {
				if isEOF {
					break
				}
				continue
			}

			messages = append(messages, NewUserMessage(line))
		}

		newMessage, err := agent.Chat(messages)
		if err != nil {
			log.Printf("API call failed: %v", err)
			messages = messages[:len(messages)-1]
			continue
		}
		messages = append(messages, newMessage)
		fmt.Println()
		if config.Thinking && newMessage.GetReasoningContent() != "" {
			fmt.Println("Thinking: ")
			fmt.Println(newMessage.GetReasoningContent())
		}
		if newMessage.GetContent() != "" {
			fmt.Println("Assistant: ")
			fmt.Println(newMessage.GetContent())
		}
		if len(newMessage.ToolCalls) > 0 {
			isToolCalling = true
			for i, tc := range newMessage.ToolCalls {
				fmt.Printf("Tool Call [%d]:\n", i)
				fmt.Printf("  ID: %s\n", tc.ID)
				fmt.Printf("  Name: %s\n", tc.Function.Name)
				fmt.Printf("  Arguments: %s\n", tc.Function.Arguments)

				var arg struct {
					Location string `json:"location"`
				}
				json.Unmarshal([]byte(tc.Function.Arguments), &arg)

				result := findNews(arg.Location)

				messages = append(messages, NewToolMessage(result, tc.ID))
			}
		} else {
			isToolCalling = false
		}
		fmt.Println()
	}

}
