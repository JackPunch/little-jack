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
