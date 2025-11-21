# QWEN.md – Project Context for Chatty

---

## 📚 Project Overview

**Chatty** is a minimal terminal‑based chat client for any OpenAI‑compatible API, written in **Go**.  It focuses on:
- Near‑instant startup (compiled binary, no runtime bloat)
- Streaming responses so you see the output as it arrives
- Markdown rendering with syntax highlighting
- Simple YAML configuration with environment‑variable overrides
- Session persistence (SQLite) and basic shell‑like line editing

The codebase is deliberately small (~1.6k lines of Go) and organized into a clean, modular layout that is easy to extend.

---

## 🏗️ Building & Running

The project ships a **Makefile** that drives the most common tasks.

| Task | Command | Description |
|------|---------|-------------|
| **Build** | `make build` | Compiles `cmd/chatty/main.go` into a binary named `chatty` (development build). |
| **Run** | `make run` | Builds then executes `./chatty`. |
| **Test** | `make test` | Runs Go tests with the race detector and generates coverage. |
| **Install** | `make install` | Installs the binary to `$GOPATH/bin` (or `$HOME/go/bin`). |
| **Cross‑platform builds** | `make build-all` | Generates binaries for Linux (amd64/arm64), macOS (arm64) and Windows (amd64/arm64) in the `dist/` directory. |
| **Release build** | `make build-release` | Stripped, size‑optimised binaries for distribution. |
| **Compress releases** | `make compress-release` (invoked by `make release`) | Optionally compresses binaries with `upx` if available. |
| **Tag & release** | `make release` | Runs tests, builds, compresses, then guides you through creating a Git tag. |

> **Note**: The Makefile derives version information from Git tags. If no tag exists, it defaults to `0.1.5`.

### Quick start (local build)
```bash
# Clone & enter repository
git clone https://github.com/ZaguanLabs/chatty.git
cd chatty

# Build and run
make run
```

---

## ⚙️ Configuration

A sample configuration file is provided at `config.example.yaml`.  Copy it to `config.yaml` (or specify `--config <path>` when launching) and fill in your API details.

```yaml
api:
  url: "https://api.openai.com/v1"
  key: "${OPENAI_API_KEY}"
model:
  name: "gpt-4o-mini"
  temperature: 0.7
  stream: true
ui:
  show_timestamps: true
logging:
  level: "info"
```

Environment variables override the YAML values (e.g., `CHATTY_API_KEY`, `CHATTY_API_URL`).

---

## 🧭 Development Conventions

- **Formatting** – `go fmt ./...` (automatically run by `make fmt` if added). The project currently uses the standard `gofmt` style.
- **Testing** – Tests live alongside source files; run with `make test`. The `-race` flag is enabled for extra safety.
- **Versioning** – Tags follow `v<MAJOR>.<MINOR>.<PATCH>` (e.g., `v0.3.0`). The Makefile extracts the latest tag for `VERSION`.
- **Dependency management** – Managed via Go modules (`go.mod`, `go.sum`). Add new dependencies with `go get` and commit the updated files.
- **Cross‑compilation** – Controlled via `GOOS`/`GOARCH` in the Makefile; no additional tooling required.
- **Release workflow** – `make release` runs tests, builds, compresses, and prompts for a new tag. Pushing the tag triggers GitHub Actions to publish binaries.

---

## 📂 Repository Layout

```
chatty/
├─ cmd/chatty/          # CLI entry point (main.go)
├─ internal/
│  ├─ chat.go          # Chat loop, command handling, streaming logic
│  ├─ client.go        # HTTP client for OpenAI‑compatible APIs
│  ├─ storage/
│  │   └─ storage.go   # SQLite persistence layer for sessions
│  └─ config/
│       └─ config.go   # Config parsing & validation
├─ config.example.yaml   # Sample configuration file
├─ go.mod / go.sum       # Go module definition
├─ Makefile              # Build, test, release automation
└─ README.md             # User‑facing documentation
```

---

## 📦 Key Dependencies

| Module | Purpose |
|--------|---------|
| `github.com/charmbracelet/bubbles` & `bubbletea` | TUI framework for interactive terminal UI |
| `github.com/charmbracelet/glamour` | Markdown rendering with syntax highlighting |
| `github.com/charmbracelet/lipgloss` | Styling utilities for the TUI |
| `modernc.org/sqlite` | Embedded SQLite driver for session storage |
| `gopkg.in/yaml.v3` | YAML configuration parsing |
| `github.com/peterh/liner` | Line‑editing (history, arrow keys) |

---

## 📖 How to Contribute

1. Fork the repository.
2. Create a feature/fix branch.
3. Ensure `make test` passes.
4. Follow the existing code style (standard `gofmt`).
5. Open a Pull Request.

---

## 🛡️ License

Chatty is released under the **MIT License** (see `LICENSE`).

---

*This QWEN.md file is generated automatically to give future Qwen‑Code sessions a concise, up‑to‑date snapshot of the project.*