# AGENTS.md — little-jack

> This file is for AI coding agents. It describes the project architecture, conventions, and workflows. All comments and documentation in the codebase are written in English.

---

## Project Overview

`little-jack` is a minimal Go CLI tool that sends a single user prompt to a Large Language Model (LLM) API and prints the response. It supports the **OpenAI-compatible** API format (e.g., OpenAI, DeepSeek, and other providers that follow the `/chat/completions` schema).

The program reads configuration from environment variables (optionally seeded from a `.env` file), accepts an optional prompt as a command-line argument, and outputs the AI response to stdout.

---

## Technology Stack

| Item | Version / Details |
|------|-------------------|
| Language | Go 1.26.2 |
| Module | `github.com/JackPunch/little-jack` |
| Dependencies | **None** — the project uses only the Go standard library |
| Build target | Single executable (`package main`) |

---

## Project Structure

```
.
├── go.mod          # Go module definition
├── main.go         # Entry point: loads config, reads CLI arg, calls LLM
├── env.go          # Custom `.env` file loader (no external libraries)
├── llm.go          # LLM API client: OpenAI request/response handling
├── .env.example    # Example environment variables
├── .env            # Local environment file (gitignored, may contain real secrets)
├── .gitignore      # Ignores .env, build artifacts, coverage, editor files
└── README.md       # Currently empty
```

All source files belong to `package main`. There are no sub-packages, no internal packages, and no generated code.

---

## Configuration & Environment Variables

The application expects the following environment variables at runtime:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LLM_BASE_URL` | **Yes** | — | Base URL of the API (e.g., `api.openai.com/v1`). A scheme is optional; `https://` is added automatically if missing. |
| `LLM_MODEL_NAME` | **Yes** | — | Model identifier (e.g., `gpt-4o`) |
| `LLM_API_KEY` | **Yes** | — | API key for authentication |
| `LLM_THINKING_TYPE` | No | — | Thinking mode switch. Valid values: `enabled`, `disabled` |
| `LLM_REASONING_EFFORT` | No | — | Thinking intensity control. Valid values: `low`, `medium`, `high`, `max` |

`.env` File Support  
`env.go` contains a lightweight loader that searches for `.env` in two locations:
1. Directory of the running executable
2. Directory of the source file (useful during `go run`)

It only sets a variable if it is **not already defined** in the process environment, so explicit exports always take precedence.

---

## Build and Run Commands

```bash
# Build the binary
go build -o little-jack.exe

# Run directly (requires env vars or a .env file)
go run .

# Run with a custom prompt
go run . "Explain Go interfaces in one sentence."
```

No Makefile, build scripts, or CI/CD pipelines exist in the repository.

---

## Code Style Guidelines

- Follow standard Go formatting (`gofmt`).
- Keep everything in `package main`; do not introduce sub-packages unless the project grows significantly.
- Use exported names (`PascalCase`) only for entities that need to be accessed across files. Currently all cross-file functions and types are exported (e.g., `LoadDotEnv`, `LoadConfigFromEnv`, `GenerateText`, `CreateAgent`).
- Group related code with comment separators (see `llm.go` for the `// ─── OpenAI format ───` style).
- Prefer the standard library over third-party dependencies.

---

## Testing Instructions

**There are currently no tests in the repository.**  
If you add tests:

- Name test files `*_test.go`.
- Place them in the same directory as the code under test.
- Run tests with:
  ```bash
  go test ./...
  ```
- Generate coverage:
  ```bash
  go test -coverprofile=coverage.out ./...
  go tool cover -html=coverage.out
  ```

Because the LLM client makes real HTTP calls, consider adding interfaces around `GenerateText` and the HTTP transport so that tests can use mocked responses.

---

## Security Considerations

- **API keys are secrets.** The `.env` file is listed in `.gitignore`, but verify that secrets are never committed.
- The existing `.env` file in the working directory may contain a real key. Treat it as sensitive.
- The custom `.env` loader does not support multi-line values or complex escaping; it trims quotes (`"` and `'`) from values.
- API keys are sent over HTTPS in the `Authorization` header.
- There is no retry logic, rate-limit handling, or request logging — be mindful when running in production-like environments.

---

## Runtime Architecture

1. `main()` calls `LoadDotEnv(".env")` to optionally load local environment variables.
2. `LoadConfigFromEnv()` validates required variables and returns an `LLMConfig`.
3. The prompt is taken from `os.Args[1]` or falls back to a hardcoded greeting.
4. `CreateAgent(cfg, systemPrompt...)` returns a function that builds a message slice (including an optional system message) and delegates to `GenerateText(cfg, messages)`. Thinking controls are read from `cfg.ThinkingType` and `cfg.ReasoningEffort`.
5. `GenerateText` calls the OpenAI-compatible API endpoint.
6. The HTTP helper (`doRequest`) uses a 120-second timeout and returns non-200 status codes as errors.

---

## Deployment Notes

- The output is a single statically-linked binary with zero external runtime dependencies.
- For deployment, set the required environment variables in the host environment rather than relying on a `.env` file.
- Ensure the target platform can reach the configured `LLM_BASE_URL` over HTTPS.
