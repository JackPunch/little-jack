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

func getConfig() (Config, error) {
	var config Config

	file, err := os.Open(".env")
	if err != nil {
		return Config{}, err
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
	return config, nil
}

type Message struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type RequestBody struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
	Thinking struct {
		Type string `json:"type"`
	} `json:"thinking,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Stream          bool   `json:"stream,omitempty"`
}

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

type Agent struct {
	Config Config
}

func (agent *Agent) Chat(messages []Message) (Message, error) {
	client := &http.Client{
		Timeout: 150 * time.Second,
	}
	var thinkingType string
	if agent.Config.Thinking {
		thinkingType = "enabled"
	} else {
		thinkingType = "disabled"
	}

	body := RequestBody{
		Messages: messages,
		Model:    agent.Config.ModelName,
		Thinking: struct {
			Type string `json:"type"`
		}{Type: thinkingType},
		ReasoningEffort: agent.Config.ReasoningEffort,
		Stream:          agent.Config.Stream,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return Message{}, err
	}

	url := agent.Config.BaseURL + "/chat/completions"
	req, err := http.NewRequest(
		"POST",
		url,
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return Message{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+agent.Config.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Message{}, err
	}

	var result ResponseBody
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return Message{}, err
	}

	return result.Choices[0].Message, err
}

func main() {
	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}
	agent := Agent{
		Config: config,
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
