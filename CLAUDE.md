# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Chatty is a minimal terminal chat client for OpenAI-compatible APIs written in Go. It's designed for instant startup, real-time streaming responses, and minimal dependencies. The codebase is intentionally small (~1,650 lines) and focused.

## Essential Commands

### Building and Running
```bash
make build              # Build with version info from git tags
make run                # Build and run immediately
./chatty                # Run the built binary
./chatty --config path  # Run with custom config file
```

### Testing
```bash
make test               # Run all tests
go test ./...           # Alternative test command
```

### Development
```bash
go fmt ./...            # Format code (run before commits)
make clean              # Remove built binaries and dist/
```

### Cross-Platform Builds
```bash
make build-all          # Build for all platforms (Linux, macOS, Windows)
make build-release      # Build stripped binaries for distribution (~28% smaller)
```

### Releases
```bash
make release            # Interactive release: runs tests, prompts for version, creates tag
make tag                # Quick tag creation (skips tests/checklist)
git push origin v0.x.x  # Push tag to trigger automated GitHub release
```

## Architecture

Chatty follows a lean three-layer architecture:

### 1. Entry Point (`cmd/chatty/main.go`)
- Parses flags and loads configuration
- Creates API client and storage layer
- Bootstraps the chat session
- Handles version info injection via ldflags

### 2. Core Chat Session (`internal/chat.go`)
The heart of the application managing:
- Interactive REPL loop with line editing support (via `peterh/liner`)
- Command handling (`/help`, `/exit`, `/reset`, `/history`, `/markdown`, `/list`, `/load`)
- Streaming response processing with special handling for reasoning model thinking tags (`<think>`, `<thinking>`)
- Markdown rendering via Glamour when enabled
- Real-time color output for different message types
- Session persistence coordination

**Key behavior:** The streaming logic in `streamResponse()` detects thinking tags in real-time, dims them in magenta, then streams and re-renders the final response with markdown formatting.

### 3. HTTP Client (`internal/client.go`)
OpenAI-compatible API client with:
- Non-streaming (`Chat()`) and streaming (`ChatStream()`) modes
- SSE (Server-Sent Events) parsing for streaming responses
- Special handling for o3 models (excludes temperature parameter)
- Separate timeout configurations: 30s for regular, 120s for streaming

### 4. Persistence Layer (`internal/storage/storage.go`)
SQLite-based conversation storage:
- Session management (create, list, load)
- Message persistence with timestamps
- Uses WAL mode for better concurrency
- Stores to `~/.local/share/chatty/chatty.db` by default
- Can be disabled by setting `storage.path: "disable"` in config

### 5. Configuration (`internal/config/config.go`)
YAML-based config with environment variable overrides:
- Loads from `config.yaml` by default or `--config` path
- Supports `${VAR}` expansion in YAML values
- Environment variables override file values:
  - `CHATTY_API_URL` → API endpoint
  - `CHATTY_API_KEY` or provider-specific keys → API key

## Code Patterns and Conventions

### Error Handling
- Always wrap errors with context using `fmt.Errorf("context: %w", err)`
- Return early on errors
- Validate inputs at function boundaries (nil checks, empty strings)

### Streaming Architecture
The streaming implementation is split across two files:
- `client.go`: Low-level SSE parsing and delta extraction
- `chat.go`: High-level thinking tag detection, color management, and markdown rendering

When modifying streaming behavior, consider both layers.

### Testing
- Tests are colocated with source files (`*_test.go`)
- Use table-driven tests for multiple scenarios
- Mock I/O for chat session testing via `SetIO()`

### Version Management
Version info is injected at build time via ldflags in the Makefile:
```go
-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
```
The version is then displayed with 'v' prefix stripped in the welcome banner.

## Key Dependencies

- **charmbracelet/glamour**: Terminal markdown rendering with syntax highlighting
- **peterh/liner**: Line editing with history (arrow keys, Ctrl-C handling)
- **modernc.org/sqlite**: Pure Go SQLite implementation (no CGo)
- **golang.org/x/term**: Terminal detection for enabling line editing

## Configuration Details

Chatty works with any OpenAI-compatible API. Example providers:
- Zaguán (https://zaguanai.com)
- OpenAI
- xAI (Grok)
- Anthropic (via compatible endpoint)
- Local models (Ollama, LM Studio)

Required config structure:
```yaml
api:
  url: "https://api.example.com/v1"
  key: "${API_KEY}"  # Supports env var expansion
model:
  name: "model-name"
  temperature: 0.7
  stream: true
storage:
  path: ""  # Empty = default location, "disable" = no persistence
```

## Common Development Tasks

### Adding a New Command
1. Add case to `handleCommand()` switch in `internal/chat.go`
2. Implement handler function following pattern: `func (s *Session) handleCommandName(ctx context.Context, args ...)`
3. Add command to help text in `printHelp()`

### Modifying API Behavior
- Non-streaming changes: Edit `Client.Chat()` in `internal/client.go`
- Streaming changes: Edit `Client.ChatStream()` and `processStream()` in `internal/client.go`
- Response formatting: Edit `streamResponse()` in `internal/chat.go`

### Adding Configuration Options
1. Add field to config structs in `internal/config/config.go`
2. Update validation in `Load()` function
3. Document in README.md and config.yaml example

### Single Test Execution
```bash
go test -v ./internal -run TestSpecificFunction
go test -v ./internal/config -run TestConfigLoad
```
