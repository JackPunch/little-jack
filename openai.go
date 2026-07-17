package main

type Message struct {
	Role             Role       `json:"role"`
	Content          *string    `json:"content,omitempty"`
	ReasoningContent *string    `json:"reasoning_content,omitempty"`
	ToolCallId       *string    `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	Name             *string    `json:"name,omitempty"`
}

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

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

type ResponseBody struct {
	ID                string   `json:"id"`
	Choices           []Choice `json:"choices"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	SystemFingerprint string   `json:"system_fingerprint"`
	Object            string   `json:"object"`
	Usage             Usage    `json:"usage"`
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
