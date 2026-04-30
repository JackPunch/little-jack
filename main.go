package main

import (
	"bufio"
	"os"
	"strings"
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

		BaseUrl   string
		ModelName string
		ApiKey    string
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
			config.BaseUrl = value
		case "MODEL_NAME":
			config.ModelName = value
		case "API_KEY":
			config.ApiKey = value
		}
	}

	// ==============================================
}
