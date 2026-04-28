<!-- From: /Users/jackpunch/WorkSpace/little-jack/AGENTS.md -->
# AGENTS.md — little-jack

> This file is for AI coding agents. It describes the project architecture, conventions, and workflows. All comments and documentation in the codebase are written in English, with the exception of `README.md` which contains brief Chinese notes about the test environment.

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
├── llm.go          # LLM API client: config, OpenAI request/response, streaming
├── .env.example    # Example environment variables
├── .env            # Local environment file (gitignored, may contain real secrets)
├── .gitignore      # Ignores .env, build artifacts, coverage, editor files
├── little-jack     # Pre-built binary (gitignored)
└── README.md       # Brief Chinese notes about the test environment
```

All source files belong to `package main`. There are no sub-packages, no internal packages, and no generated code.

---

## Configuration & Environment Variables

The application expects the following environment variables at runtime:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LLM_BASE_URL` | **Yes** | — | Base URL of the API (e.g., `api.openai.com/v1`). A scheme is optional; `https://` is added automatically if missing. |
| `LLM_MODEL_NAME` | **Yes** | — | Model identifier (e.g., `gpt-4o`, `o3-mini`) |
| `LLM_API_KEY` | **Yes** | — | API key for authentication |
| `LLM_THINKING_TYPE` | No | — | Thinking mode switch. Valid values: `enabled`, `disabled` |
| `LLM_REASONING_EFFORT` | No | — | Thinking intensity control. Valid values: `low`, `medium`, `high`, `max` |
| `LLM_DEBUG` | No | `false` | Enables HTTP request/response logging. When set, `LoadConfigFromEnv` assigns `os.Stderr` to `cfg.DebugOutput`. Valid values: `true`, `1`, `yes`, `on`. |
| `LLM_STREAM` | No | `false` | Whether to use streaming output in the demo `main`. Valid values: `true`, `1`, `yes`, `on`. **Always disabled when `LLM_DEBUG` is enabled.** This is read by `main`, not by the framework. |

`.env` File Support  
`env.go` contains a lightweight loader that searches for `.env` in two locations:
1. Directory of the running executable
2. Directory of the source file (useful during `go run`)

**Note:** The doc comment in `env.go` mentions a third search location (current working directory), but the implementation does **not** include it. The actual search order is executable directory first, then source file directory.

It only sets a variable if it is **not already defined** in the process environment (checked via `os.Getenv`), so explicit exports always take precedence. The loader does not support multi-line values or complex escaping; it trims surrounding quotes (`"` and `'`) from values.

---

## Build and Run Commands

```bash
# Build the binary
go build -o little-jack

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

1. `main()` calls `LoadDotEnv(".env")` to optionally load local environment variables, then calls `LoadConfigFromEnv()` to read all env vars and construct an `LLMConfig`. The LLM layer (`llm.go`) does not read environment variables.
2. The prompt is taken from `os.Args[1]` or falls back to a hardcoded greeting (`"Hello, introduce yourself in one sentence."`).
3. `main` decides which factory to use based on its own logic (e.g. checking `LLM_STREAM`). `CreateAgent(cfg, systemPrompt...)` returns a non-streaming function that builds a message slice and delegates to `GenerateText(cfg, messages)`. `CreateStreamingAgent(cfg, systemPrompt...)` returns a streaming variant that delegates to `GenerateTextStream(cfg, messages, onChunk)`. Thinking controls are passed through `cfg.ThinkingType` and `cfg.ReasoningEffort`.
4. `GenerateText` calls the OpenAI-compatible API endpoint (`/chat/completions`) in non-streaming mode. `GenerateTextStream` opens an SSE connection and invokes the caller-supplied `onChunk` callback for every token; it does **not** perform any IO itself. Both functions respect `cfg.DebugOutput` for request/response logging.
5. The HTTP helper (`doRequest`) uses a 120-second timeout and returns non-200 status codes as errors. When a non-nil `debugOut` is provided, it writes the full request and response bodies to that writer.

---

## Deployment Notes

- The output is a single statically-linked binary with zero external runtime dependencies.
- For deployment, set the required environment variables in the host environment rather than relying on a `.env` file.
- Ensure the target platform can reach the configured `LLM_BASE_URL` over HTTPS.
