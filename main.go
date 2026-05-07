package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	// ==============================================
	// 先读取.env环境变量
	// ==============================================
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
	var config Config

	file, err := os.Open(".env")
	if err != nil {
		panic(err)
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

	// ==============================================
	// 创建提示词
	// ==============================================
	systemPrompt := "You are a helpful assistant."
	userPrompt := "What is the capital of France?"
	if len(os.Args) > 1 && os.Args[1] != "" {
		userPrompt = os.Args[1]
	}
	type Message struct {
		Role             string `json:"role"`
		Content          string `json:"content"`
		ReasoningContent string `json:"reasoning_content,omitempty"`
	}
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// ==============================================
	// 创建请求
	// ==============================================
	client := &http.Client{
		Timeout: 150 * time.Second,
	}

	type RequestBody struct {
		Messages []Message `json:"messages"`
		Model    string    `json:"model"`
		Thinking struct {
			Type string `json:"type"`
		} `json:"thinking,omitempty"`
		ReasoningEffort string `json:"reasoning_effort,omitempty"`
		Stream          bool   `json:"stream,omitempty"`
		Tools           string `json:"tools,omitempty"` // 暂不支持，占位
	}
	var thinkingType string
	if config.Thinking {
		thinkingType = "enabled"
	} else {
		thinkingType = "disabled"
	}

	body := RequestBody{
		Messages: messages,
		Model:    config.ModelName,
		Thinking: struct {
			Type string `json:"type"`
		}{Type: thinkingType},
		ReasoningEffort: config.ReasoningEffort,
		Stream:          config.Stream,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}

	url := config.BaseURL + "/chat/completions"
	req, err := http.NewRequest(
		"POST",
		url,
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	// ==============================================
	// 发送请求
	// ==============================================
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// ==============================================
	// 解析响应
	// ==============================================
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	// ==============================================
	// 先打印看看
	// ==============================================
	// var prettyJSON bytes.Buffer
	// if err := json.Indent(&prettyJSON, respBody, "", "  "); err == nil {
	// 	fmt.Println(prettyJSON.String())
	// }

	type ResponseBody struct {
		ID      string `json:"id"`
		Choices []struct {
			FinishReason string  `json:"finish_reason"`
			Index        int64   `json:"index"`
			Message      Message `json:"message"`
		} `json:"choices"`
		Created           int64  `json:"created"`
		Model             string `json:"model"`
		SystemFingerprint string `json:"system_fingerprint"`
		Object            string `json:"object"`
		Usage             struct {
			CompletionTokens      int64 `json:"completion_tokens"`
			PromptTokens          int64 `json:"prompt_tokens"`
			PromptCacheHitTokens  int64 `json:"prompt_cache_hit_tokens"`
			PromptCacheMissTokens int64 `json:"prompt_cache_miss_tokens"`
			TotalTokens           int64 `json:"total_tokens"`
		} `json:"usage"`
	}

	var result ResponseBody
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		panic(err)
	}
	// fmt.Println(result)
	fmt.Println(result.Choices[0].Message.Content)

	// ==============================================
	// 构造第二次请求
	// ==============================================
	messages = append(messages, result.Choices[0].Message)

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')

	newMessage := Message{
		Role:    "user",
		Content: input,
	}

	messages = append(messages, newMessage)

	body = RequestBody{
		Messages: messages,
		Model:    config.ModelName,
		Thinking: struct {
			Type string `json:"type"`
		}{Type: thinkingType},
		ReasoningEffort: config.ReasoningEffort,
		Stream:          config.Stream,
	}

	jsonBody, err = json.Marshal(body)
	if err != nil {
		panic(err)
	}

	req, err = http.NewRequest(
		"POST",
		url,
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	// ==============================================
	// 发送请求
	// ==============================================
	resp, err = client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// ==============================================
	// 解析响应
	// ==============================================
	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(respBody, &result)
	if err != nil {
		panic(err)
	}
	// fmt.Println(result)
	fmt.Println(result.Choices[0].Message.Content)

}
