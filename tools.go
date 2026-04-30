// tools.go - 工具定义与实现
//
// 每个工具需要以下三个函数（以 ask_user 为例）：
//   1. AskUserTool() Tool
//      - 返回 Tool 结构体，描述工具名称、用途和参数 Schema
//      - 供 ToolRegistry 注册时传入
//   2. AskUserHandler(args json.RawMessage) (string, error)
//      - 签名固定：func(json.RawMessage) (string, error)
//      - 解析 AI 传入的 JSON 参数，调用实际逻辑，返回结果字符串
//   3. AskUser(question string) (string, error)
//      - 实际执行业务逻辑的函数
//      - 输入/输出类型不限，由 Handler 负责与 AI 侧的 JSON 格式转换
//
// 新增工具时，参照 ask_user 的格式，在文件末尾添加新的区块即可。

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// ─── ask_user ─────────────────────────────────────────────

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

// ───────────────────────────────────────────────────────────
