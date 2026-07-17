package main

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
