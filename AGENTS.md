# AGENTS.md — little-jack

> This file is for AI coding agents. It describes the project architecture, conventions, and workflows. All comments and documentation in the codebase are written in English, with the exception of `README.md` and the header comment block in `tools.go`, both of which contain brief Chinese notes.

---

## Project Overview

`little-jack` is a minimal Go CLI tool that sends a single user prompt to a Large Language Model (LLM) API and prints the response. It supports the **OpenAI-compatible** API format (e.g., OpenAI, DeepSeek, and other providers that follow the `/chat/completions` schema).

The program reads configuration from environment variables (optionally seeded from a `.env` file), accepts an optional prompt as a command-line argument, and outputs the AI response to stdout. It supports three execution modes:

1. **Normal (non-streaming)** — waits for the full response and prints it.
2. **Streaming** — prints tokens as they arrive via Server-Sent Events (SSE).
3. **Tool calling** — enables multi-turn conversations where the model can invoke registered tools (e.g., `ask_user`). Tool mode forces non-streaming output.

---

## Technology Stack

| Item         | Version / Details                                        |
| ------------ | -------------------------------------------------------- |
| Language     | Go 1.26.2                                                |
| Module       | `github.com/JackPunch/little-jack`                       |
| Dependencies | **None** — the project uses only the Go standard library |
| Build target | Single executable (`package main`)                       |

---

## Project Structure

```
.
├── go.mod           # Go module definition
├── main.go          # Entry point: loads config, reads CLI arg, dispatches agent mode
├── env.go           # Custom `.env` file loader (no external libraries)
├── llm_types.go     # Shared types: LLMConfig, Message, LLMResponse, StreamReader, ToolCallReader, OpenAI structs, tool schemas
├── llm_generate.go  # Non-streaming API client: GenerateText, CreateAgent, CreateStreamingAgent, CreateToolAgent
├── llm_stream.go    # Streaming API client: GenerateTextStream with SSE parsing
├── llm_http.go      # HTTP helpers: doRequest, writeDebugBody, writeDebugSSE, normalizeBaseURL
├── tools.go         # Tool definitions and handlers (e.g., ask_user)
├── tool_registry.go # ToolRegistry: registers tools, dispatches tool execution
├── .env.example     # Example environment variables
├── .env             # Local environment file (gitignored, may contain real secrets)
├── .gitignore       # Ignores .env, build artifacts, coverage, editor files
├── little-jack      # Pre-built binary (gitignored)
└── README.md        # Brief Chinese notes about the test environment
```

All source files belong to `package main`. There are no sub-packages, no internal packages, and no generated code.

---

## Configuration & Environment Variables

The application expects the following environment variables at runtime:

| Variable               | Required | Default | Description                                                                                                                                            |
| ---------------------- | -------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `LLM_BASE_URL`         | **Yes**  | —       | Base URL of the API (e.g., `api.openai.com/v1`). A scheme is optional; `https://` is added automatically if missing.                                   |
| `LLM_MODEL_NAME`       | **Yes**  | —       | Model identifier (e.g., `gpt-4o`, `o3-mini`, `deepseek-v4-flash`)                                                                                      |
| `LLM_API_KEY`          | **Yes**  | —       | API key for authentication                                                                                                                             |
| `LLM_THINKING_TYPE`    | No       | —       | Thinking mode switch. Valid values: `enabled`, `disabled`                                                                                              |
| `LLM_REASONING_EFFORT` | No       | —       | Thinking intensity control. Valid values: `low`, `medium`, `high`, `max`                                                                               |
| `LLM_DEBUG`            | No       | `false` | Enables HTTP request/response logging. When set, `LoadConfigFromEnv` assigns `os.Stderr` to `cfg.DebugOutput`. Valid values: `true`, `1`, `yes`, `on`. |
| `LLM_STREAM`           | No       | `false` | Whether to use streaming output in `main`. Valid values: `true`, `1`, `yes`, `on`. This is read by `main`, not by the LLM framework.                   |
| `LLM_TOOLS`            | No       | `false` | Whether to enable multi-turn tool calling. Valid values: `true`, `1`, `yes`, `on`. Tool mode takes priority over streaming mode.                       |

`.env` File Support  
`env.go` contains a lightweight loader that searches for `.env` in two locations:

1. Directory of the running executable
2. Directory of this source file (useful during `go run`)

**Note:** The doc comment in `env.go` mentions a third search location (current working directory), but the implementation does **not** include it. The actual search order is executable directory first, then source file directory.

It only sets a variable if it is **not already defined** in the process environment (checked via `os.Getenv`), so explicit exports always take precedence. The loader does not support multi-line values or complex escaping; it trims surrounding quotes (`"` and `'`) from values.

---

## Build and Run Commands

```bash
# Build the binary
go build

# Run directly (requires env vars or a .env file)
go run .

# Run with a custom prompt
go run . "Explain Go interfaces in one sentence."

# Run in tool mode (requires LLM_TOOLS=true)
$env:LLM_TOOLS="true"; go run . "What is the weather?"
```

No Makefile, build scripts, or CI/CD pipelines exist in the repository.

---

## Code Style Guidelines

- Follow standard Go formatting (`gofmt`).
- Keep everything in `package main`; do not introduce sub-packages unless the project grows significantly.
- Use exported names (`PascalCase`) only for entities that need to be accessed across files. Currently all cross-file functions and types are exported (e.g., `LoadDotEnv`, `LoadConfigFromEnv`, `GenerateText`, `CreateAgent`, `CreateStreamingAgent`, `CreateToolAgent`).
- Group related code with comment separators (see `llm_types.go` for the `// ─── OpenAI format ───` style).
- Prefer the standard library over third-party dependencies.

### Adding a New Tool

To add a new tool, create three functions in `tools.go` following the `ask_user` pattern:

1. `XxxTool() Tool` — returns the `Tool` struct describing the function name, description, and JSON Schema parameters.
2. `XxxHandler(args json.RawMessage) (string, error)` — unmarshals the JSON arguments and calls the business logic.
3. `Xxx(...) (string, error)` — the actual business logic function with native Go types.

Then register the tool and its handler in `main.go` inside `runToolAgent` via `registry.Register(XxxTool(), XxxHandler)`.

---

## Runtime Architecture

1. `main()` calls `LoadDotEnv(".env")` to optionally load local environment variables, then calls `LoadConfigFromEnv()` (defined in `main.go`) to read all env vars and construct an `LLMConfig`. The LLM layer does not read environment variables directly.
2. The prompt is taken from `os.Args[1]` or falls back to a hardcoded greeting (`"Hello, introduce yourself in one sentence."`).
3. `main` dispatches to one of three runners based on config:
   - **Tool mode** (`cfg.ToolsEnabled`) — calls `runToolAgent`, which uses `CreateToolAgent`.
   - **Streaming mode** (`LLM_STREAM=true`, tools disabled) — calls `runStreamAgent`, which uses `CreateStreamingAgent`.
   - **Normal mode** (default) — calls `runNormalAgent`, which uses `CreateAgent`.
4. `CreateAgent(cfg, systemPrompt...)` returns a non-streaming function that builds a message slice and delegates to `GenerateText(cfg, messages)`.
5. `CreateStreamingAgent(cfg, systemPrompt...)` returns a streaming variant that delegates to `GenerateTextStream(cfg, messages)`, returning a `*StreamReader`. The reader yields individual `StreamChunk` values via `ReadChunk`, and the aggregated response is available through `Result()` after the stream is consumed.
6. `CreateToolAgent(cfg, registry, systemPrompt...)` returns a function that performs multi-turn tool calling. It returns a `*ToolCallReader` that yields `Turn` values via `ReadTurn`. Each assistant turn may contain `Content`, `ReasoningContent`, and `ToolCalls`. The caller must acknowledge non-final turns by sending on `reader.ackCh` so the agent can execute tools and continue. Reasoning content is preserved and replayed verbatim on every turn (required by DeepSeek's API).
7. `GenerateText` (in `llm_generate.go`) calls the OpenAI-compatible API endpoint (`/chat/completions`) in non-streaming mode. `GenerateTextStream` (in `llm_stream.go`) opens an SSE connection and returns a `*StreamReader` that parses SSE lines in a background goroutine. Both functions respect `cfg.DebugOutput` for request/response logging.
8. The HTTP helper (`doRequest` in `llm_http.go`) uses a 120-second timeout and returns non-200 status codes as errors. When a non-nil `debugOut` is provided, it writes the full request and response bodies to that writer.
9. `llm_types.go` defines all shared data structures: `LLMConfig`, `Message`, `LLMResponse`, `StreamChunk`, `StreamReader`, `ToolCallReader`, `Turn`, `Tool`, `Function`, `Parameters`, `Property`, `ToolCall`, and the internal OpenAI request/response types.

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

Because the LLM client makes real HTTP calls, consider adding interfaces around `GenerateText` and the HTTP transport so that tests can use mocked responses. The tool registry can be tested independently by creating a `ToolRegistry`, registering mock tools, and calling `Execute` with synthetic `ToolCall` values.

---

## Security Considerations

- **API keys are secrets.** The `.env` file is listed in `.gitignore`, but verify that secrets are never committed.
- The existing `.env` file in the working directory may contain a real key. Treat it as sensitive.
- The custom `.env` loader does not support multi-line values or complex escaping; it trims quotes (`"` and `'`) from values.
- API keys are sent over HTTPS in the `Authorization` header.
- There is no retry logic, rate-limit handling, or request logging — be mindful when running in production-like environments.
- The `ask_user` tool writes prompts to `stderr` and reads answers from `stdin`. In piped or non-interactive environments, this may block or receive EOF.

---

## Deployment Notes

- The output is a single statically-linked binary with zero external runtime dependencies.
- For deployment, set the required environment variables in the host environment rather than relying on a `.env` file.
- Ensure the target platform can reach the configured `LLM_BASE_URL` over HTTPS.
- If tool calling is enabled, ensure `stdin` is available if the `ask_user` tool (or similar interactive tools) is registered.
