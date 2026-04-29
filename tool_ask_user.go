package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// AskUserTool returns the tool definition for asking the user a question.
func AskUserTool() Tool {
	return Tool{
		Type: "function",
		Function: Function{
			Name:        "ask_user",
			Description: "Ask the user a question when you need more information, clarification, or confirmation to proceed. Use this tool instead of guessing or making assumptions.",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"question": {
						Type:        "string",
						Description: "The question to ask the user. Be clear and concise.",
					},
				},
				Required: []string{"question"},
			},
		},
	}
}

// AskUserHandler is the tool handler that prompts the user and reads their answer.
// It reads an entire line (including spaces) from stdin.
func AskUserHandler(args json.RawMessage) (string, error) {
	var params struct {
		Question string `json:"question"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("failed to parse ask_user arguments: %w", err)
	}
	if strings.TrimSpace(params.Question) == "" {
		return "", fmt.Errorf("ask_user received an empty question")
	}
	answer, err := AskUser(params.Question)
	if err != nil {
		return "", fmt.Errorf("failed to read user input: %w", err)
	}
	return answer, nil
}

// AskUser prints a question to stderr and reads the user's answer from stdin.
// It reads a full line (spaces are preserved) and strips the trailing newline.
// Writing to stderr ensures the prompt is visible even when stdout is piped.
func AskUser(question string) (string, error) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "[Tool: ask_user]", question)
	fmt.Fprint(os.Stderr, "> ")
	// Flush stderr so the prompt appears immediately.
	_ = os.Stderr.Sync()

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return "", fmt.Errorf("received EOF while waiting for input (stdin may be closed or piped)")
		}
		return "", err
	}

	answer = strings.TrimSuffix(answer, "\n")
	answer = strings.TrimSuffix(answer, "\r")
	answer = strings.TrimSpace(answer)
	fmt.Fprintf(os.Stderr, "[Received input: %q]\n", answer)
	return answer, nil
}
