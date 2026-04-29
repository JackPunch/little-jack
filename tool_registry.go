package main

import (
	"encoding/json"
	"fmt"
)

// ToolRegistry manages a collection of tools and their handlers.
type ToolRegistry struct {
	tools    []Tool
	handlers map[string]func(json.RawMessage) (string, error)
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		handlers: make(map[string]func(json.RawMessage) (string, error)),
	}
}

// Register adds a tool and its handler to the registry.
func (r *ToolRegistry) Register(tool Tool, handler func(json.RawMessage) (string, error)) {
	r.tools = append(r.tools, tool)
	r.handlers[tool.Function.Name] = handler
}

// Tools returns the list of tool definitions for API requests.
func (r *ToolRegistry) Tools() []Tool {
	return r.tools
}

// Execute runs the handler for the given tool call.
func (r *ToolRegistry) Execute(call ToolCall) (string, error) {
	h, ok := r.handlers[call.Function.Name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", call.Function.Name)
	}
	return h(json.RawMessage(call.Function.Arguments))
}
