# Chatty

A dead-simple, blazing-fast terminal chat client for OpenAI-compatible APIs written in Go.

## Features

- **Fast startup**: Near-instant launch with minimal overhead
- **Streaming responses**: Real-time token-by-token output for immediate feedback
- **Reasoning model support**: Automatically detects and styles thinking tags (`<think>`, `<thinking>`) with dimmed magenta color
- **Markdown rendering**: Beautiful formatted output with syntax highlighting for code blocks
- **Simple & clean**: Intuitive command-line interface with ANSI colors
- **Config-driven**: Loads settings from `config.yaml` with environment overrides
- **Interactive chat**: Real-time conversation with in-memory history
- **Lean architecture**: Easy to understand and modify

## Getting Started

### Prerequisites

- Go 1.23 or later
- Access to an OpenAI-compatible API endpoint

### Installation

```bash
go install github.com/PromptShieldLabs/chatty/cmd/chatty@latest
```

Alternatively, clone the repository and build locally:

```bash
git clone https://github.com/PromptShieldLabs/chatty.git
cd chatty
go build ./cmd/chatty
```

### Configuration

Chatty works with any OpenAI-compatible API provider. Create `config.yaml` in the project root or pass `--config` to specify a path.

#### Using PromptShield

[PromptShield](https://promptshield.io) provides access to multiple AI models through a unified API:

```yaml
api:
  url: "https://api.promptshield.io/v1"
  key: "${CHATTY_API_KEY}"
model:
  name: "openai/gpt-4o-mini"
  temperature: 0.7
  stream: true
```

#### Using OpenAI Directly

```yaml
api:
  url: "https://api.openai.com/v1"
  key: "${CHATTY_API_KEY}"
model:
  name: "gpt-4o-mini"
  temperature: 0.7
  stream: true
```

#### Using Other Compatible Providers

Chatty works with any OpenAI-compatible API. Examples:

**xAI (Grok):**
```yaml
api:
  url: "https://api.x.ai/v1"
  key: "${CHATTY_API_KEY}"
model:
  name: "grok-beta"
  temperature: 0.7
  stream: true
```

**Anthropic (Direct API):**
```yaml
api:
  url: "https://api.anthropic.com/v1"
  key: "${CHATTY_API_KEY}"
model:
  name: "claude-3-5-sonnet-20241022"
  temperature: 0.7
  stream: true
```

**Local models (Ollama, LM Studio, etc.):**
```yaml
api:
  url: "http://localhost:11434/v1"
  key: "not-needed"
model:
  name: "llama3.2"
  temperature: 0.7
  stream: true
```

#### Environment Variables

Environment variables override config file values:

- `CHATTY_API_URL` - Override the API endpoint
- `CHATTY_API_KEY` - Override the API key

### Running

```bash
# Build with version info
make build

# Run
./chatty

# Or build and run directly
make run
```

### Available Commands

Once running, you can use these commands:
- `/help` - Show available commands
- `/exit` or `/quit` - Exit the chat
- `/reset` or `/clear` - Clear conversation history
- `/history` - Show conversation history
- `/markdown` - Toggle markdown rendering on/off

## Architecture

Chatty follows a lean, simple architecture:

```
chatty/
├── cmd/chatty/
│   └── main.go           # Entry point (~55 lines)
├── internal/
│   ├── config/
│   │   └── config.go     # Config loading (~134 lines)
│   ├── client.go         # OpenAI HTTP client (~228 lines)
│   └── chat.go           # Chat loop + colors (~453 lines)
├── config.yaml
└── go.mod
```

**Total: ~870 lines of production code**

## Development

- Run tests: `make test`
- Build: `make build`
- Install: `make install`
- Format: `go fmt ./...`

## License

TBD
