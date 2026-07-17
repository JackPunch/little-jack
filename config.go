package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const configFile = "config.yaml"

type Config struct {
	BaseURL         string `yaml:"base_url"`
	ModelName       string `yaml:"model_name"`
	APIKey          string `yaml:"api_key"`
	Thinking        bool   `yaml:"thinking"`
	ReasoningEffort string `yaml:"reasoning_effort"`
	Tools           bool   `yaml:"tools"`
	Stream          bool   `yaml:"stream"` // 暂不支持流式调用
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
