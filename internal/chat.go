package internal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ZaguanLabs/chatty/internal/config"
	"github.com/ZaguanLabs/chatty/internal/storage"
	"github.com/ZaguanLabs/chatty/internal/ui"
	"github.com/ZaguanLabs/chatty/internal/validation"
	"github.com/charmbracelet/glamour"
	"github.com/peterh/liner"
	"golang.org/x/term"
)

// Global markdown renderer singleton to avoid repeated initialization overhead
var (
	mdRenderer     *glamour.TermRenderer
	mdRendererInit sync.Once
	mdRendererErr  error

	// Cached regex patterns for thinking tags
	thinkTagPattern     *regexp.Regexp
	thinkClosePattern   *regexp.Regexp
	patternInit         sync.Once
)

// initThinkingPatterns initializes regex patterns for detecting thinking tags
func initThinkingPatterns() {
	thinkTagPattern = regexp.MustCompile(`(<thinking>)|(\\u0060\\u0060\\u0060)`)
	thinkClosePattern = regexp.MustCompile(`(</thinking>)|(\\u0060\\u0060\\u0060)`)
}

// initMarkdownRenderer initializes the global markdown renderer once.
func initMarkdownRenderer() {
	// Use fixed dark style instead of WithAutoStyle to avoid terminal background detection
	mdRenderer, mdRendererErr = glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(100),
	)
}

// enhanceCodeBlocks processes markdown-rendered text to add enhanced styling to code blocks
func (s *Session) enhanceCodeBlocks(renderedText string) string {
	// Simple approach: wrap code blocks with enhanced borders when detected
	lines := strings.Split(renderedText, "\n")
	var enhanced strings.Builder

	inCodeBlock := false
	codeBlockLang := ""
	codeLineCount := 0

	for _, line := range lines {
		// Check for code block start (language specification)
		if strings.HasPrefix(line, "```") && !inCodeBlock {
			inCodeBlock = true
			codeLineCount = 0

			// Extract language if specified
			codeBlockLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))

			// Create enhanced code block header using UI functions
			emoji := ui.GetLanguageEmoji(codeBlockLang)
			if codeBlockLang != "" {
				enhanced.WriteString(ui.BorderGray + "┌─ " + emoji + " " + codeBlockLang + " " + strings.Repeat("─", s.getContentWidth()-len(codeBlockLang)-len(emoji)-5) + "┐" + ui.Reset + "\n")
			} else {
				enhanced.WriteString(ui.BorderGray + "┌─ Code Block " + strings.Repeat("─", s.getContentWidth()-15) + "┐" + ui.Reset + "\n")
			}
			enhanced.WriteString(ui.BGGray + ui.BorderGray + "│" + ui.Reset + "\n")
			continue
		}

		// Check for code block end
		if strings.HasPrefix(line, "```") && inCodeBlock {
			inCodeBlock = false
			codeLineCount++

			// Close the code block
			enhanced.WriteString(ui.BGGray + ui.BorderGray + "│" + ui.Reset + "\n")
			if codeBlockLang != "" {
				enhanced.WriteString(ui.BorderGray + "└" + strings.Repeat("─", s.getContentWidth()-2) + "┘" + ui.Reset + "\n")
			} else {
				enhanced.WriteString(ui.BorderGray + "└" + strings.Repeat("─", s.getContentWidth()-2) + "┘" + ui.Reset + "\n")
			}
			enhanced.WriteString("\n")
			codeBlockLang = ""
			continue
		}

		// Process code block lines
		if inCodeBlock {
			codeLineCount++
			if strings.TrimSpace(line) != "" {
				// Add the code line with enhanced styling
				enhanced.WriteString(ui.BGGray + ui.Cyan + " " + line)
				if len(line) < s.getContentWidth()-2 {
					enhanced.WriteString(strings.Repeat(" ", s.getContentWidth()-2-len(line)))
				}
				enhanced.WriteString(" " + ui.Reset + "\n")
			} else {
				enhanced.WriteString(ui.BGGray + " " + strings.Repeat(" ", s.getContentWidth()-2) + " " + ui.Reset + "\n")
			}
		} else {
			// Regular content
			enhanced.WriteString(line + "\n")
		}
	}

	return enhanced.String()
}

// getMarkdownRenderer returns the global markdown renderer, initializing it if needed.
func getMarkdownRenderer() (*glamour.TermRenderer, error) {
	mdRendererInit.Do(initMarkdownRenderer)
	return mdRenderer, mdRendererErr
}

// CommandHandler defines the interface for command handlers.
// CommandHandler interface for processing chat commands
type CommandHandler interface {
	Process(ctx context.Context, parts []string) (exit bool, err error)
	Name() string
	Aliases() []string
	HelpText() string
	Usage() string
	MinArgs() int
}

// CommandRegistry maps command names to their handlers.
type CommandRegistry struct {
	handler CommandHandler
}

// Command definitions for easy extensibility.
var commandRegistry = map[string]CommandRegistry{
	"exit":     {handler: &ExitCommandHandler{session: nil}},
	"reset":    {handler: &ResetCommandHandler{session: nil}},
	"help":     {handler: &HelpCommandHandler{session: nil}},
	"history":  {handler: &HistoryCommandHandler{session: nil}},
	"markdown": {handler: &MarkdownCommandHandler{session: nil}},
	"list":     {handler: &ListCommandHandler{session: nil}},
	"load":     {handler: &LoadCommandHandler{session: nil}},
}

// initializeCommandHandlers sets up the command handlers.
func (s *Session) initializeCommandHandlers() map[string]CommandHandler {
	handlers := make(map[string]CommandHandler)
	for cmd, reg := range commandRegistry {
		// Set session reference for handlers that need it
		if h, ok := reg.handler.(interface{ setSession(*Session) }); ok {
			h.setSession(s)
		}
		handlers[cmd] = reg.handler
	}
	return handlers
}

// findCommand finds a command by its alias.
func findCommand(alias string) (string, *CommandRegistry) {
	for cmd, reg := range commandRegistry {
		for _, cmdAlias := range reg.handler.Aliases() {
			if alias == cmdAlias {
				return cmd, &reg
			}
		}
	}
	return "", nil
}

// Command Handler Implementations

// ExitCommandHandler handles the exit command
type ExitCommandHandler struct {
	session *Session
}

func (h *ExitCommandHandler) setSession(s *Session) { h.session = s }

func (h *ExitCommandHandler) Process(ctx context.Context, parts []string) (exit bool, err error) {
	// Create a nice goodbye header
	goodbyeText := "👋 Goodbye! Thanks for using Chatty!"
	width := len(goodbyeText) + 4
	if width < 40 {
		width = 40
	}

	// Top border
	fmt.Fprint(h.session.output, ui.BorderBlue+"┌"+strings.Repeat("─", width-2)+"┐"+ui.Reset+"\n")

	// Goodbye text
	fmt.Fprint(h.session.output, ui.BGBlue+ui.BrightWhite+" │ "+goodbyeText)
	if len(goodbyeText) < width-3 {
		fmt.Fprint(h.session.output, strings.Repeat(" ", width-3-len(goodbyeText)))
	}
	fmt.Fprint(h.session.output, " │"+ui.Reset+"\n")

	// Final border
	fmt.Fprint(h.session.output, ui.BorderBlue+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")
	return true, nil
}

func (h *ExitCommandHandler) Name() string { return "exit" }
func (h *ExitCommandHandler) Aliases() []string { return []string{"/exit", "/quit"} }
func (h *ExitCommandHandler) HelpText() string { return "Exit the chat" }
func (h *ExitCommandHandler) Usage() string { return "" }
func (h *ExitCommandHandler) MinArgs() int { return 0 }

// ResetCommandHandler handles the reset command
type ResetCommandHandler struct {
	session *Session
}

func (h *ResetCommandHandler) setSession(s *Session) { h.session = s }

func (h *ResetCommandHandler) Process(ctx context.Context, parts []string) (exit bool, err error) {
	h.session.history = h.session.history[:0]
	h.session.sessionID = 0

	// Create a nice reset header
	resetText := "🗑️ History cleared. Starting fresh!"
	width := len(resetText) + 4
	if width < 40 {
		width = 40
	}

	// Top border
	fmt.Fprint(h.session.output, ui.BorderGray+"┌"+strings.Repeat("─", width-2)+"┐"+ui.Reset+"\n")

	// Reset text
	fmt.Fprint(h.session.output, ui.BGGray+ui.BrightWhite+" │ "+resetText)
	if len(resetText) < width-3 {
		fmt.Fprint(h.session.output, strings.Repeat(" ", width-3-len(resetText)))
	}
	fmt.Fprint(h.session.output, " │"+ui.Reset+"\n")

	// Final border
	fmt.Fprint(h.session.output, ui.BorderGray+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")
	return false, nil
}

func (h *ResetCommandHandler) Name() string { return "reset" }
func (h *ResetCommandHandler) Aliases() []string { return []string{"/reset", "/clear"} }
func (h *ResetCommandHandler) HelpText() string { return "Clear conversation history" }
func (h *ResetCommandHandler) Usage() string { return "" }
func (h *ResetCommandHandler) MinArgs() int { return 0 }

// HelpCommandHandler handles the help command
type HelpCommandHandler struct {
	session *Session
}

func (h *HelpCommandHandler) setSession(s *Session) { h.session = s }

func (h *HelpCommandHandler) Process(ctx context.Context, parts []string) (exit bool, err error) {
	h.session.printHelp()
	return false, nil
}

func (h *HelpCommandHandler) Name() string { return "help" }
func (h *HelpCommandHandler) Aliases() []string { return []string{"/help"} }
func (h *HelpCommandHandler) HelpText() string { return "Show available commands" }
func (h *HelpCommandHandler) Usage() string { return "" }
func (h *HelpCommandHandler) MinArgs() int { return 0 }

// HistoryCommandHandler handles the history command
type HistoryCommandHandler struct {
	session *Session
}

func (h *HistoryCommandHandler) setSession(s *Session) { h.session = s }

func (h *HistoryCommandHandler) Process(ctx context.Context, parts []string) (exit bool, err error) {
	h.session.printHistory()
	return false, nil
}

func (h *HistoryCommandHandler) Name() string { return "history" }
func (h *HistoryCommandHandler) Aliases() []string { return []string{"/history"} }
func (h *HistoryCommandHandler) HelpText() string { return "Show conversation history" }
func (h *HistoryCommandHandler) Usage() string { return "" }
func (h *HistoryCommandHandler) MinArgs() int { return 0 }

// MarkdownCommandHandler handles the markdown command
type MarkdownCommandHandler struct {
	session *Session
}

func (h *MarkdownCommandHandler) setSession(s *Session) { h.session = s }

func (h *MarkdownCommandHandler) Process(ctx context.Context, parts []string) (exit bool, err error) {
	h.session.renderMarkdown = !h.session.renderMarkdown
	status := "enabled"
	if !h.session.renderMarkdown {
		status = "disabled"
	}

	// Create a nice markdown header
	markdownText := fmt.Sprintf("✨ Markdown rendering %s!", status)
	width := len(markdownText) + 4
	if width < 40 {
		width = 40
	}

	// Top border
	fmt.Fprint(h.session.output, ui.BorderGray+"┌"+strings.Repeat("─", width-2)+"┐"+ui.Reset+"\n")

	// Markdown text
	fmt.Fprint(h.session.output, ui.BGGray+ui.BrightWhite+" │ "+markdownText)
	if len(markdownText) < width-3 {
		fmt.Fprint(h.session.output, strings.Repeat(" ", width-3-len(markdownText)))
	}
	fmt.Fprint(h.session.output, " │"+ui.Reset+"\n")

	// Final border
	fmt.Fprint(h.session.output, ui.BorderGray+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")
	return false, nil
}

func (h *MarkdownCommandHandler) Name() string { return "markdown" }
func (h *MarkdownCommandHandler) Aliases() []string { return []string{"/markdown"} }
func (h *MarkdownCommandHandler) HelpText() string { return "Toggle markdown rendering" }
func (h *MarkdownCommandHandler) Usage() string { return "" }
func (h *MarkdownCommandHandler) MinArgs() int { return 0 }

// ListCommandHandler handles the list command
type ListCommandHandler struct {
	session *Session
}

func (h *ListCommandHandler) setSession(s *Session) { h.session = s }

func (h *ListCommandHandler) Process(ctx context.Context, parts []string) (exit bool, err error) {
	return false, h.session.handleListSessions(ctx)
}

func (h *ListCommandHandler) Name() string { return "list" }
func (h *ListCommandHandler) Aliases() []string { return []string{"/list", "/sessions"} }
func (h *ListCommandHandler) HelpText() string { return "Show saved conversations" }
func (h *ListCommandHandler) Usage() string { return "" }
func (h *ListCommandHandler) MinArgs() int { return 0 }

// LoadCommandHandler handles the load command
type LoadCommandHandler struct {
	session *Session
}

func (h *LoadCommandHandler) setSession(s *Session) { h.session = s }

func (h *LoadCommandHandler) Process(ctx context.Context, parts []string) (exit bool, err error) {
	if len(parts) < 2 {
		return false, errors.New("usage: /load <session-id>")
	}

	id, convErr := strconv.ParseInt(parts[1], 10, 64)
	if convErr != nil {
		return false, fmt.Errorf("invalid session id %q", parts[1])
	}

	if err := h.session.handleLoadSession(ctx, id); err != nil {
		return false, err
	}
	return false, nil
}

func (h *LoadCommandHandler) Name() string { return "load" }
func (h *LoadCommandHandler) Aliases() []string { return []string{"/load"} }
func (h *LoadCommandHandler) HelpText() string { return "Load a saved conversation" }
func (h *LoadCommandHandler) Usage() string { return "/load <session-id>" }
func (h *LoadCommandHandler) MinArgs() int { return 1 }

// ANSI color codes and styles for terminal output
const (
	colorReset   = "\033[0m"
	colorCyan    = "\033[36m"
	colorGreen   = "\033[32m"
	colorRed     = "\033[31m"
	colorYellow  = "\033[33m"
	colorMagenta = "\033[35m"
	colorBlue    = "\033[34m"
	colorWhite   = "\033[37m"
	colorGray    = "\033[90m"
	styleDim     = "\033[2m"
	styleItalic  = "\033[3m"
	styleBold    = "\033[1m"
)

// Unicode box drawing characters for visual separators
const (
	boxHorizontal   = "─"
	boxVertical     = "│"
	boxTopLeft      = "┌"
	boxTopRight     = "┐"
	boxBottomLeft   = "└"
	boxBottomRight  = "┘"
	boxCross        = "┼"
	boxTeeDown      = "┬"
	boxTeeUp        = "┴"
	boxTeeRight     = "├"
	boxTeeLeft      = "┤"
	separatorThin   = "────────────────────────────────────────"
	separatorThick   = "════════════════════════════════════════"
)

// Session manages a chat conversation with history.
type Session struct {
	client         *Client
	config         *config.Config
	store          *storage.Store
	sessionID      int64
	history        []Message
	input          io.Reader
	output         io.Writer
	useColors      bool
	version        string
	renderMarkdown bool
	lineReader     *liner.State
	terminalWidth  int
}

// NewSession creates a new chat session.
func NewSession(client *Client, cfg *config.Config, store *storage.Store, version string) (*Session, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	s := &Session{
		client:         client,
		config:         cfg,
		store:          store,
		history:        make([]Message, 0, 16),
		input:          os.Stdin,
		output:         os.Stdout,
		useColors:      true,
		version:        version,
		renderMarkdown: true,
	}

	// Detect terminal width for responsive design
	s.detectTerminalWidth()

	return s, nil
}

// detectTerminalWidth determines the actual terminal width for responsive UI
func (s *Session) detectTerminalWidth() {
	width := 80 // Default fallback width

	// Try to get terminal size from the system
	if fd := s.input.(*os.File); fd != nil && fd.Name() == "/dev/stdin" {
		if w, _, err := term.GetSize(int(fd.Fd())); err == nil && w > 0 {
			width = w
		}
	}

	// Apply reasonable limits for terminal UI
	if width > 120 {
		width = 120 // Cap maximum width for better readability
	} else if width < 40 {
		width = 40 // Minimum width for UI elements
	}

	s.terminalWidth = width
}

// getContentWidth returns the usable width for content (excluding margins/padding)
func (s *Session) getContentWidth() int {
	// Reserve space for borders, avatar, and padding
	// Format: [avatar] content (with borders)
	// Roughly 8 chars for borders/avatars, rest for content
	return s.terminalWidth - 8
}

// Run starts the interactive chat loop.
func (s *Session) Run(ctx context.Context) error {
	if s == nil {
		return errors.New("session is nil")
	}
	if ctx == nil {
		return errors.New("context is nil")
	}

	s.printWelcome()

	var scanner *bufio.Scanner
	if s.shouldUseLineEditor() {
		if s.lineReader == nil {
			s.lineReader = liner.NewLiner()
			s.lineReader.SetCtrlCAborts(true)
		}
		defer s.closeLineReader()
	} else {
		scanner = bufio.NewScanner(s.input)
	}

	for {
		var raw string
		var err error

		if s.lineReader != nil {
			raw, err = s.lineReader.Prompt(s.plainPromptString())
			if err != nil {
				if errors.Is(err, io.EOF) {
					fmt.Fprintln(s.output)
					return nil
				}
				if errors.Is(err, liner.ErrPromptAborted) {
					fmt.Fprintln(s.output)
					continue
				}
				return err
			}
		} else {
			s.printPrompt()
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return err
				}
				return nil
			}
			raw = scanner.Text()
		}

		input := strings.TrimSpace(raw)
		if input == "" {
			continue
		}
		if s.lineReader != nil {
			s.lineReader.AppendHistory(input)
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			exit, err := s.handleCommand(ctx, input)
			if err != nil {
				s.printError(err.Error())
			}
			if exit {
				return nil
			}
			continue
		}

		// Send message to AI
		if err := s.sendMessage(ctx, input); err != nil {
			s.printError(err.Error())
			continue
		}
	}
}

func (s *Session) ensureSession(ctx context.Context, firstMessage string) error {
	if s.store == nil || s.sessionID != 0 {
		return nil
	}

	title := strings.TrimSpace(firstMessage)
	if title != "" {
		title = strings.Split(title, "\n")[0]
		if len(title) > 80 {
			title = title[:80]
		}
	}

	id, err := s.store.CreateSession(ctx, title)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	s.sessionID = id
	return nil
}

func (s *Session) persistExchange(ctx context.Context, userMsg, assistantMsg Message) {
	if s.store == nil || s.sessionID == 0 {
		return
	}

	// Use batch operations for better performance
	messages := []storage.Message{
		{Role: userMsg.Role, Content: userMsg.Content},
		{Role: assistantMsg.Role, Content: assistantMsg.Content},
	}

	if err := s.store.AppendMessagesBatch(ctx, s.sessionID, messages); err != nil {
		s.printError(fmt.Sprintf("Failed to save messages batch: %v", err))
	}
}

func (s *Session) handleListSessions(ctx context.Context) error {
	if s.store == nil {
		return errors.New("persistence is disabled")
	}

	sessions, err := s.store.ListSessions(ctx, 0)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	// Create a nice sessions header
	sessionsText := "📁 Saved Sessions"
	width := 50
	if width < 40 {
		width = 40
	}

	// Top border
	fmt.Fprint(s.output, ui.BorderGray+"┌"+strings.Repeat("─", width-2)+"┐"+ui.Reset+"\n")

	// Sessions text
	fmt.Fprint(s.output, ui.BGGray+ui.BrightWhite+" │ "+sessionsText)
	if len(sessionsText) < width-3 {
		fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(sessionsText)))
	}
	fmt.Fprint(s.output, " │"+ui.Reset+"\n")

	// Bottom border
	fmt.Fprint(s.output, ui.BorderGray+"├"+strings.Repeat("─", width-2)+"┤"+ui.Reset+"\n")

	if len(sessions) == 0 {
		// No sessions message
		noSessionsText := "No saved sessions yet."
		fmt.Fprint(s.output, ui.BGSystem+ui.BrightWhite+" │ "+noSessionsText)
		if len(noSessionsText) < width-3 {
			fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(noSessionsText)))
		}
		fmt.Fprint(s.output, " │"+ui.Reset+"\n")

		// Final border
		fmt.Fprint(s.output, ui.BorderGray+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")
		return nil
	}

	for _, summary := range sessions {
		updated := formatRelative(summary.UpdatedAt)
		title := summary.Name
		if strings.TrimSpace(title) == "" {
			title = "Untitled session"
		}

		// Session header
		sessionHeader := fmt.Sprintf("#%d %s", summary.ID, title)
		fmt.Fprint(s.output, ui.BGSystem+ui.BrightWhite+" │ "+sessionHeader)
		if len(sessionHeader) < width-3 {
			fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(sessionHeader)))
		}
		fmt.Fprint(s.output, " │"+ui.Reset+"\n")

		// Session details
		details := fmt.Sprintf("  📝 %d messages │ 🕐 %s", summary.MessageCount, updated)
		fmt.Fprint(s.output, ui.BGSystem+ui.BrightWhite+" │ "+details)
		if len(details) < width-3 {
			fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(details)))
		}
		fmt.Fprint(s.output, " │"+ui.Reset+"\n")

		// Empty line between sessions
		fmt.Fprint(s.output, ui.BGSystem+" │"+strings.Repeat(" ", width-2)+"│"+ui.Reset+"\n")
	}

	// Final border
	fmt.Fprint(s.output, ui.BorderGray+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")

	return nil
}

func (s *Session) handleLoadSession(ctx context.Context, id int64) error {
	if s.store == nil {
		return errors.New("persistence is disabled")
	}

	transcript, err := s.store.LoadSession(ctx, id)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}

	s.sessionID = transcript.Summary.ID
	s.history = s.history[:0]

	for _, msg := range transcript.Messages {
		s.history = append(s.history, Message{Role: msg.Role, Content: msg.Content})
	}

	title := transcript.Summary.Name
	if strings.TrimSpace(title) == "" {
		title = "Untitled session"
	}

	// Create a nice success header
	successText := fmt.Sprintf("✅ Loaded session #%d: %s", transcript.Summary.ID, title)
	width := len(successText) + 4
	if width < 40 {
		width = 40
	}

	// Top border
	fmt.Fprint(s.output, ui.BorderGreen+"┌"+strings.Repeat("─", width-2)+"┐"+ui.Reset+"\n")

	// Success text
	fmt.Fprint(s.output, ui.BGGreen+ui.BrightWhite+" │ "+successText)
	if len(successText) < width-3 {
		fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(successText)))
	}
	fmt.Fprint(s.output, " │"+ui.Reset+"\n")

	// Session details
	details := fmt.Sprintf("📋 %d messages loaded", len(transcript.Messages))
	fmt.Fprint(s.output, ui.BGGreen+ui.BrightWhite+" │ "+details)
	if len(details) < width-3 {
		fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(details)))
	}
	fmt.Fprint(s.output, " │"+ui.Reset+"\n")

	// Final border
	fmt.Fprint(s.output, ui.BorderGreen+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")

	return nil
}

func formatRelative(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	delta := time.Since(t)
	if delta < time.Minute {
		return "just now"
	}
	if delta < time.Hour {
		return fmt.Sprintf("%d min ago", int(delta.Minutes()))
	}
	if delta < 24*time.Hour {
		return fmt.Sprintf("%d hr ago", int(delta.Hours()))
	}
	if delta < 30*24*time.Hour {
		return fmt.Sprintf("%d d ago", int(delta.Hours()/24))
	}
	return t.Format("2006-01-02")
}

func (s *Session) sendMessage(ctx context.Context, input string) error {
	// Validate and sanitize input first
	if err := validation.ValidateMessage(input); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Sanitize the input
	sanitizedInput := validation.SanitizeInput(input, validation.MaxUserMessageLength)

	// Create a child context with timeout for the entire operation
	// Create a child context with timeout for the entire operation
	messageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer func() { cancel() }()

	if s.store != nil && s.sessionID == 0 {
		if err := s.ensureSession(messageCtx, sanitizedInput); err != nil {
			s.printError(fmt.Sprintf("Failed to initialise persistence: %v", err))
			s.store = nil
		}
	}

	// Add user message to history
	userMsg := Message{Role: "user", Content: sanitizedInput}
	s.history = append(s.history, userMsg)

	// Display user message with enhanced formatting
	s.printUserMessage(sanitizedInput)

	var reply string
	var err error

	// Check if context is still valid before proceeding
	select {
	case <-messageCtx.Done():
		// Context cancelled or timed out
		s.history = s.history[:len(s.history)-1] // Remove user message
		return messageCtx.Err()
	default:
		// Continue with message processing
	}

	if s.config.Model.Stream {
		// Streaming mode
		reply, err = s.streamResponse(messageCtx)
	} else {
		// Non-streaming mode
		reply, err = s.client.Chat(messageCtx, s.history, s.config.Model.Name, s.config.Model.Temperature)
		if err == nil {
			s.printAssistant(reply)
		}
	}

	if err != nil {
		// Remove the user message if the request failed
		s.history = s.history[:len(s.history)-1]

		// Handle context cancellation specially
		if messageCtx.Err() != nil {
			return fmt.Errorf("chat request cancelled or timed out: %w", messageCtx.Err())
		}
		return fmt.Errorf("chat request failed: %w", err)
	}

	// Add assistant response to history
	assistantMsg := Message{Role: "assistant", Content: reply}
	s.history = append(s.history, assistantMsg)

	// Persist with a separate timeout for storage operations
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer persistCancel()
	s.persistExchange(persistCtx, userMsg, assistantMsg)

	return nil
}

func (s *Session) streamResponse(ctx context.Context) (string, error) {
	var fullResponse strings.Builder
	var buffer strings.Builder
	var afterThinkingContent strings.Builder
	inThinking := false
	thinkingStarted := false
	thinkingClosed := false
	frameCount := 0

	// Print message header at start
	if !thinkingStarted {
		s.printMessageHeader("Assistant", colorGreen)
		// Show initial loading indicator with background
		loadingMsg := ui.CreateLoadingMessage("🤖", "Thinking...", frameCount)
		if s.useColors {
			fmt.Fprint(s.output, ui.BGAssistant+ui.BrightWhite+" ")
			fmt.Fprint(s.output, loadingMsg)
			if len(loadingMsg) < s.getContentWidth()-2 {
				fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(loadingMsg)))
			}
			fmt.Fprint(s.output, " "+ui.Reset+"\n")
		} else {
			s.println(loadingMsg)
		}
		fmt.Fprint(s.output, "\r\x1b[K") // Clear the line for streaming
		frameCount++
	}

	// Regex patterns for thinking tags - handle both formats
	thinkTagPattern := regexp.MustCompile(`(<thinking>)|(<think>)`)
	thinkClosePattern := regexp.MustCompile(`(</thinking>)|(</think>)`)

	err := s.client.ChatStream(ctx, s.history, s.config.Model.Name, s.config.Model.Temperature, func(chunk string) error {
		fullResponse.WriteString(chunk)

		// Update loading animation frame periodically
		if !thinkingStarted && !inThinking {
			frameCount = (frameCount + 1) % 10
			if frameCount % 3 == 0 { // Update every 3rd frame to avoid too fast updates
				fmt.Fprint(s.output, "\r\x1b[K") // Clear line
				loadingMsg := ui.CreateLoadingMessage("🤖", "Generating response...", frameCount)
				if s.useColors {
					fmt.Fprint(s.output, ui.BGAssistant+ui.BrightWhite+" ")
					fmt.Fprint(s.output, loadingMsg)
					if len(loadingMsg) < s.getContentWidth()-2 {
						fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(loadingMsg)))
					}
					fmt.Fprint(s.output, " "+ui.Reset+"\n")
				} else {
					fmt.Fprint(s.output, loadingMsg)
				}
			}
		}

		// If we're past thinking tags, stream AND collect for markdown rendering
		if thinkingClosed {
			afterThinkingContent.WriteString(chunk)
			// Stream the chunk in real-time
			if s.useColors && afterThinkingContent.Len() == len(chunk) {
				// First chunk after thinking - set color
				fmt.Fprint(s.output, ui.BGAssistant+ui.BrightWhite+" ")
				fmt.Fprint(s.output, chunk)
			} else if s.useColors {
				fmt.Fprint(s.output, chunk)
			} else {
				fmt.Fprint(s.output, chunk)
			}
			return nil
		}

		buffer.WriteString(chunk)
		bufferStr := buffer.String()

		// Check for opening thinking tags
		if !inThinking && thinkTagPattern.MatchString(bufferStr) {
			loc := thinkTagPattern.FindStringIndex(bufferStr)
			if loc != nil {
				// Print content before tag
				beforeTag := bufferStr[:loc[0]]
				if beforeTag != "" && !thinkingStarted {
					if s.useColors {
						fmt.Fprint(s.output, ui.BGAssistant+ui.BrightWhite+" ")
						fmt.Fprint(s.output, beforeTag)
						if len(beforeTag) < s.getContentWidth()-2 {
							fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(beforeTag)))
						}
						fmt.Fprint(s.output, " "+ui.Reset+"\n")
					} else {
						fmt.Fprint(s.output, beforeTag)
					}
				}

				// Switch to thinking mode
				inThinking = true
				thinkingStarted = true
				if s.useColors {
					var buf strings.Builder
					buf.WriteString(ui.Reset)
					buf.WriteString(ui.Faint)
					buf.WriteString(ui.Magenta)
					fmt.Fprint(s.output, buf.String())
				}

				// Print opening tag and content after it
				afterTag := bufferStr[loc[0]:]
				if s.useColors {
					fmt.Fprint(s.output, ui.BGAssistant+ui.Magenta+" ")
					fmt.Fprint(s.output, afterTag)
					if len(afterTag) < s.getContentWidth()-2 {
						fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(afterTag)))
					}
					fmt.Fprint(s.output, " "+ui.Reset+"\n")
				} else {
					fmt.Fprint(s.output, afterTag)
				}
				buffer.Reset()
			}
		} else if inThinking && thinkClosePattern.MatchString(bufferStr) {
			// Check for closing thinking tags
			loc := thinkClosePattern.FindStringIndex(bufferStr)
			if loc != nil {
				// Print content including closing tag
				upToAndIncludingTag := bufferStr[:loc[1]]
				if s.useColors {
					fmt.Fprint(s.output, ui.BGAssistant+ui.Magenta+" ")
					fmt.Fprint(s.output, upToAndIncludingTag)
					if len(upToAndIncludingTag) < s.getContentWidth()-2 {
						fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(upToAndIncludingTag)))
					}
					fmt.Fprint(s.output, " "+ui.Reset+"\n")
				} else {
					fmt.Fprint(s.output, upToAndIncludingTag)
				}

				// Switch back to normal mode
				inThinking = false
				thinkingClosed = true
				if s.useColors {
					fmt.Fprint(s.output, ui.Reset)
				}

				// Start streaming and collecting content after closing tag
				afterTag := bufferStr[loc[1]:]
				if afterTag != "" {
					afterThinkingContent.WriteString(afterTag)
					if s.useColors {
						fmt.Fprint(s.output, ui.BGAssistant+ui.BrightWhite+" ")
						fmt.Fprint(s.output, afterTag)
						if len(afterTag) < s.getContentWidth()-2 {
							fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(afterTag)))
						}
						fmt.Fprint(s.output, " "+ui.Reset+"\n")
					} else {
						fmt.Fprint(s.output, afterTag)
					}
				}
				buffer.Reset()
			}
		} else {
			// Normal streaming - print as we go
			if !thinkingStarted && !inThinking {
				if s.useColors {
					if fullResponse.Len() == len(chunk) {
						// First chunk - add background
						fmt.Fprint(s.output, ui.BGAssistant+ui.BrightWhite+" ")
						fmt.Fprint(s.output, chunk)
					} else {
						fmt.Fprint(s.output, chunk)
					}
					thinkingStarted = true
				} else {
					fmt.Fprint(s.output, chunk)
				}
			} else {
				fmt.Fprint(s.output, chunk)
			}
			buffer.Reset()
			buffer.WriteString(chunk)
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	// Reset colors and add newline after streaming
	if s.useColors {
		fmt.Fprint(s.output, ui.Reset)
	}
	fmt.Fprintln(s.output)

	// Print message footer
	s.printMessageFooter()

	// If we collected content after thinking tags AND markdown is enabled, re-render with markdown
	if thinkingClosed && afterThinkingContent.Len() > 0 && s.renderMarkdown {
		renderer, err := getMarkdownRenderer()
		if err != nil {
			s.printError(fmt.Sprintf("Failed to initialize markdown renderer: %v", err))
		} else {
			finalContent := strings.TrimSpace(afterThinkingContent.String())
			if finalContent != "" {
				rendered, err := renderer.Render(finalContent)
				if err == nil {
					// Print a separator and the markdown-rendered version
					fmt.Fprintln(s.output, s.colorize(ui.Faint+ui.Yellow, ui.CreateSeparatorWithWidth(s.getContentWidth(), "thin")))
					s.printMessageHeader("Formatted Response", colorBlue)
					fmt.Fprint(s.output, rendered)
					s.printMessageFooter()
				}
			}
		}
	} else if !thinkingStarted {
		// No thinking tags - render everything with markdown
		response := fullResponse.String()
		s.printAssistant(response)
	}

	return fullResponse.String(), nil
}

func (s *Session) handleCommand(ctx context.Context, cmd string) (exit bool, err error) {
	// Validate command input
	if err := validation.ValidateCommand(cmd); err != nil {
		return false, fmt.Errorf("invalid command: %w", err)
	}

	// Sanitize command
	sanitizedCmd := validation.SanitizeInput(cmd, validation.MaxCommandLength)

	parts := strings.Fields(sanitizedCmd)
	if len(parts) == 0 {
		return false, nil
	}

	handlers := s.initializeCommandHandlers()
	commandName, reg := findCommand(parts[0])

	if commandName == "" {
		// Simple error message
		s.println(fmt.Sprintf("❓ Unknown command: %q. Use /help to see available commands.", parts[0]))
		return false, fmt.Errorf("unknown command %q. Try /help", parts[0])
	}

	// Validate minimum arguments
	if len(parts) < reg.handler.MinArgs()+1 { // +1 because parts[0] is the command itself
		usageText := ""
		if reg.handler.Usage() != "" {
			usageText = fmt.Sprintf(" (usage: %s)", reg.handler.Usage())
		}
		s.println(fmt.Sprintf("⚠️ Command %q requires at least %d arguments%s", parts[0], reg.handler.MinArgs(), usageText))
		return false, fmt.Errorf("command %q requires at least %d arguments%s", parts[0], reg.handler.MinArgs(), usageText)
	}

	// Execute command handler
	handler, exists := handlers[commandName]
	if !exists {
		return false, fmt.Errorf("handler not found for command %q", commandName)
	}

	return handler.Process(ctx, parts)
}

func (s *Session) printWelcome() {
	// Create a nice welcome header
	welcomeText := fmt.Sprintf("🤖 Chatty v%s - Ready to chat!", s.version)
	width := len(welcomeText) + 4
	if width < 40 {
		width = 40
	}

	// Top border
	fmt.Fprint(s.output, ui.BorderBlue+"┌"+strings.Repeat("─", width-2)+"┐"+ui.Reset+"\n")

	// Welcome text
	fmt.Fprint(s.output, ui.BGBlue+ui.BrightWhite+" │ "+welcomeText)
	if len(welcomeText) < width-3 {
		fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(welcomeText)))
	}
	fmt.Fprint(s.output, " │"+ui.Reset+"\n")

	// Model info
	modelText := fmt.Sprintf("Model: %s | Temperature: %.1f", s.config.Model.Name, s.config.Model.Temperature)
	fmt.Fprint(s.output, ui.BGBlue+ui.BrightWhite+" │ "+modelText)
	if len(modelText) < width-3 {
		fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(modelText)))
	}
	fmt.Fprint(s.output, " │"+ui.Reset+"\n")

	// Bottom border
	fmt.Fprint(s.output, ui.BorderBlue+"├"+strings.Repeat("─", width-2)+"┤"+ui.Reset+"\n")

	// Instructions
	fmt.Fprint(s.output, ui.BGBlue+ui.BrightWhite+" │ "+"Type /help for commands, /exit to quit")
	if len("Type /help for commands, /exit to quit") < width-3 {
		fmt.Fprint(s.output, strings.Repeat(" ", width-3-len("Type /help for commands, /exit to quit")))
	}
	fmt.Fprint(s.output, " │"+ui.Reset+"\n")

	// Final border
	fmt.Fprint(s.output, ui.BorderBlue+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")
}

func (s *Session) printHelp() {
	// Create a nice help header
	helpText := "📚 Available Commands"
	width := len(helpText) + 4
	if width < 40 {
		width = 40
	}

	// Top border
	fmt.Fprint(s.output, ui.BorderGreen+"┌"+strings.Repeat("─", width-2)+"┐"+ui.Reset+"\n")

	// Help text
	fmt.Fprint(s.output, ui.BGGreen+ui.BrightWhite+" │ "+helpText)
	if len(helpText) < width-3 {
		fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(helpText)))
	}
	fmt.Fprint(s.output, " │"+ui.Reset+"\n")

	// Bottom border
	fmt.Fprint(s.output, ui.BorderGreen+"├"+strings.Repeat("─", width-2)+"┤"+ui.Reset+"\n")

	var buf strings.Builder

	// Group commands by category for better organization
	type HelpEntry struct {
		command  string
		aliases  []string
		helpText string
		usage    string
	}

	var helpEntries []HelpEntry
	for _, reg := range commandRegistry {
		// Get primary command name (first alias without slash)
		var primaryCmd string
		for _, alias := range reg.handler.Aliases() {
			if !strings.HasPrefix(alias, "/") {
				primaryCmd = alias
				break
			}
		}
		if primaryCmd == "" && len(reg.handler.Aliases()) > 0 {
			primaryCmd = reg.handler.Aliases()[0]
		}

		helpEntries = append(helpEntries, HelpEntry{
			command:  primaryCmd,
			aliases:  reg.handler.Aliases(),
			helpText: reg.handler.HelpText(),
			usage:    reg.handler.Usage(),
		})
	}

	// Sort commands alphabetically
	for i := 0; i < len(helpEntries); i++ {
		for j := i + 1; j < len(helpEntries); j++ {
			if helpEntries[i].command > helpEntries[j].command {
				helpEntries[i], helpEntries[j] = helpEntries[j], helpEntries[i]
			}
		}
	}

	for _, entry := range helpEntries {
		var cmdBuf strings.Builder
		cmdBuf.WriteString("  ")
		cmdBuf.WriteString(s.colorize(styleBold+colorCyan, entry.aliases[0]))
		if len(entry.aliases) > 1 {
			for i := 1; i < len(entry.aliases); i++ {
				cmdBuf.WriteString(", ")
				cmdBuf.WriteString(s.colorize(styleDim+colorGray, entry.aliases[i]))
			}
		}
		cmdBuf.WriteString(" ")
		cmdBuf.WriteString(s.colorize(styleDim+colorGray, "─"))
		cmdBuf.WriteString(" ")
		cmdBuf.WriteString(s.colorize(colorWhite, entry.helpText))
		if entry.usage != "" {
			cmdBuf.WriteString("\n    ")
			cmdBuf.WriteString(s.colorize(styleDim+colorYellow, fmt.Sprintf("Usage: %s", entry.usage)))
		}
		buf.WriteString(cmdBuf.String())
		buf.WriteString("\n")
	}

	// Print command list with background
	lines := strings.Split(buf.String(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			fmt.Fprint(s.output, ui.BGSystem+ui.BrightWhite+" │ "+line)
			if len(line) < width-3 {
				fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(line)))
			}
			fmt.Fprint(s.output, " │"+ui.Reset+"\n")
		} else {
			fmt.Fprint(s.output, ui.BGSystem+" │"+strings.Repeat(" ", width-2)+"│"+ui.Reset+"\n")
		}
	}

	// Final border
	fmt.Fprint(s.output, ui.BorderGreen+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")
}

func (s *Session) printHistory() {
	if len(s.history) == 0 {
		s.println("No conversation history yet.")
		return
	}

	// Create a nice history header
	historyText := "📜 Conversation History"
	width := len(historyText) + 4
	if width < 40 {
		width = 40
	}

	// Top border
	fmt.Fprint(s.output, ui.BorderGray+"┌"+strings.Repeat("─", width-2)+"┐"+ui.Reset+"\n")

	// History text
	fmt.Fprint(s.output, ui.BGGray+ui.BrightWhite+" │ "+historyText)
	if len(historyText) < width-3 {
		fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(historyText)))
	}
	fmt.Fprint(s.output, " │"+ui.Reset+"\n")

	// Bottom border
	fmt.Fprint(s.output, ui.BorderGray+"├"+strings.Repeat("─", width-2)+"┤"+ui.Reset+"\n")

	for i, msg := range s.history {
		prefix := "User"
		if msg.Role == "assistant" {
			prefix = "AI"
		}

		// Message header
		msgHeader := fmt.Sprintf("[%d] %s:", i+1, prefix)
		fmt.Fprint(s.output, ui.BGSystem+ui.BrightWhite+" │ "+msgHeader)
		if len(msgHeader) < width-3 {
			fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(msgHeader)))
		}
		fmt.Fprint(s.output, " │"+ui.Reset+"\n")

		// Truncate long messages for history view
		content := msg.Content
		if len(content) > 100 {
			content = content[:97] + "..."
		}

		// Message content
		fmt.Fprint(s.output, ui.BGSystem+ui.BrightWhite+" │ "+"    "+content)
		if len("    "+content) < width-3 {
			fmt.Fprint(s.output, strings.Repeat(" ", width-3-len("    "+content)))
		}
		fmt.Fprint(s.output, " │"+ui.Reset+"\n")

		// Empty line between messages
		fmt.Fprint(s.output, ui.BGSystem+" │"+strings.Repeat(" ", width-2)+"│"+ui.Reset+"\n")
	}

	// Final border
	fmt.Fprint(s.output, ui.BorderGray+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")
}

func (s *Session) printPrompt() {
	fmt.Fprint(s.output, s.promptString())
}

func (s *Session) printAssistant(text string) {
	if s.renderMarkdown {
		renderer, err := getMarkdownRenderer()
		if err != nil {
			// Failed to get renderer, fallback to plain text
			s.printMessageHeader("Assistant", colorGreen)
			if s.useColors {
				// Print with background color
				lines := strings.Split(text, "\n")
				for _, line := range lines {
					fmt.Fprint(s.output, ui.BGAssistant+ui.BrightWhite+" ")
					fmt.Fprint(s.output, line)
					if len(line) < s.getContentWidth()-2 {
						fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(line)))
					}
					fmt.Fprint(s.output, " "+ui.Reset+"\n")
				}
			} else {
				s.println(text)
			}
			s.printMessageFooter()
			return
		}
		// Render markdown with enhanced styling
		rendered, err := renderer.Render(text)
		if err != nil {
			// Fallback to plain text if rendering fails
			s.printMessageHeader("Assistant", colorGreen)
			if s.useColors {
				// Print with background color
				lines := strings.Split(text, "\n")
				for _, line := range lines {
					fmt.Fprint(s.output, ui.BGAssistant+ui.BrightWhite+" ")
					fmt.Fprint(s.output, line)
					if len(line) < s.getContentWidth()-2 {
						fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(line)))
					}
					fmt.Fprint(s.output, " "+ui.Reset+"\n")
				}
			} else {
				s.println(text)
			}
			s.printMessageFooter()
			return
		}
		s.printMessageHeader("Assistant", colorGreen)

		// Enhance the rendered markdown with better code block styling
		enhanced := s.enhanceCodeBlocks(rendered)
		fmt.Fprint(s.output, enhanced)
		s.printMessageFooter()
	} else {
		// Plain text mode with enhanced styling
		s.printMessageHeader("Assistant", colorGreen)
		if s.useColors {
			// Print with background color
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				fmt.Fprint(s.output, ui.BGAssistant+ui.BrightWhite+" ")
				fmt.Fprint(s.output, line)
				if len(line) < s.getContentWidth()-2 {
					fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(line)))
				}
				fmt.Fprint(s.output, " "+ui.Reset+"\n")
			}
		} else {
			s.println(text)
		}
		s.printMessageFooter()
	}
}

func (s *Session) printWithThinkingTags(text string) {
	// Split by thinking tags and print with different colors
	thinkTagPattern := regexp.MustCompile(`(<think>|<thinking>)([\s\S]*?)(</think>|</thinking>)`)

	lastEnd := 0
	matches := thinkTagPattern.FindAllStringSubmatchIndex(text, -1)

	for _, match := range matches {
		// Print content before thinking tag (should be minimal/none)
		if match[0] > lastEnd && s.useColors {
			beforeThinking := text[lastEnd:match[0]]
			if strings.TrimSpace(beforeThinking) != "" {
				var buf strings.Builder
				buf.WriteString(colorGreen)
				buf.WriteString(beforeThinking)
				buf.WriteString(colorReset)
				fmt.Fprint(s.output, buf.String())
			}
		}

		// Print thinking content in dimmed magenta
		if s.useColors {
			var buf strings.Builder
			buf.WriteString(styleDim)
			buf.WriteString(colorMagenta)
			buf.WriteString(text[match[0]:match[1]])
			buf.WriteString(colorReset)
			fmt.Fprint(s.output, buf.String())
		} else {
			fmt.Fprint(s.output, text[match[0]:match[1]])
		}

		lastEnd = match[1]
	}

	// Print remaining content after last thinking tag with markdown rendering
	if lastEnd < len(text) {
		finalResponse := text[lastEnd:]
		if strings.TrimSpace(finalResponse) != "" {
			// Render the final response with markdown if enabled
			if s.renderMarkdown {
				renderer, err := getMarkdownRenderer()
				if err != nil {
					// Failed to get renderer, fallback to plain text
					if s.useColors {
						var buf strings.Builder
						buf.WriteString(colorGreen)
						buf.WriteString(finalResponse)
						buf.WriteString(colorReset)
						fmt.Fprintln(s.output, buf.String())
					} else {
						fmt.Fprintln(s.output, finalResponse)
					}
				} else {
					rendered, err := renderer.Render(finalResponse)
					if err != nil {
						// Fallback to plain text
						if s.useColors {
							var buf strings.Builder
							buf.WriteString(colorGreen)
							buf.WriteString(finalResponse)
							buf.WriteString(colorReset)
							fmt.Fprintln(s.output, buf.String())
						} else {
							fmt.Fprintln(s.output, finalResponse)
						}
					} else {
						fmt.Fprint(s.output, rendered)
					}
				}
			} else {
				// Plain text mode
				if s.useColors {
					var buf strings.Builder
					buf.WriteString(colorGreen)
					buf.WriteString(finalResponse)
					buf.WriteString(colorReset)
					fmt.Fprintln(s.output, buf.String())
				} else {
					fmt.Fprintln(s.output, finalResponse)
				}
			}
		}
	} else {
		fmt.Fprintln(s.output)
	}
}

func (s *Session) printMessageFooter() {
	// Add proper message container footer with bottom border
	fmt.Fprint(s.output, "\n")
	fmt.Fprint(s.output, ui.CreateMessageFooter("message", s.getContentWidth()))
	fmt.Fprint(s.output, "\n\n") // Extra spacing between messages
}

func (s *Session) printMessageHeader(role string, roleColor string) {
	now := time.Now()

	var headerText string
	switch role {
	case "User":
		headerText = ui.CreateMessageHeader("user", now)
	case "Assistant":
		headerText = ui.CreateMessageHeader("assistant", now)
	default:
		headerText = ui.CreateMessageHeader("message", now)
	}

	fmt.Fprint(s.output, headerText)
	fmt.Fprint(s.output, "\n") // Single newline after header
}

func (s *Session) printUserMessage(content string) {
	s.printMessageHeader("User", colorCyan)

	// Print content with user background color
	if s.useColors {
		// Wrap content for proper background coloring
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			fmt.Fprint(s.output, ui.BGUser+ui.BrightWhite+" ")
			fmt.Fprint(s.output, line)
			if len(line) < s.getContentWidth()-2 {
				fmt.Fprint(s.output, strings.Repeat(" ", s.getContentWidth()-2-len(line)))
			}
			fmt.Fprint(s.output, " "+ui.Reset+"\n")
		}
	} else {
		s.println(content)
	}
	s.printMessageFooter()
}

func (s *Session) printError(text string) {
	// Enhanced error message with better styling
	errorMsg := ui.CreateStatusMessage("❌", "Error: "+text, "error")

	// Create a nice error header
	width := len(errorMsg) + 4
	if width < 40 {
		width = 40
	}

	// Top border
	fmt.Fprint(s.output, ui.BorderGray+"┌"+strings.Repeat("─", width-2)+"┐"+ui.Reset+"\n")

	// Error text
	fmt.Fprint(s.output, ui.BGSystem+ui.BrightWhite+" │ "+errorMsg)
	if len(errorMsg) < width-3 {
		fmt.Fprint(s.output, strings.Repeat(" ", width-3-len(errorMsg)))
	}
	fmt.Fprint(s.output, " │"+ui.Reset+"\n")

	// Final border
	fmt.Fprint(s.output, ui.BorderGray+"└"+strings.Repeat("─", width-2)+"┘"+ui.Reset+"\n\n")
}

func (s *Session) println(text string) {
	fmt.Fprintln(s.output, text)
}

func (s *Session) colorize(color, text string) string {
	if !s.useColors {
		return text
	}
	var buf strings.Builder
	buf.WriteString(color)
	buf.WriteString(text)
	buf.WriteString(colorReset)
	return buf.String()
}

// SetIO overrides input/output streams (useful for testing).
func (s *Session) SetIO(in io.Reader, out io.Writer) {
	s.closeLineReader()
	if in != nil {
		s.input = in
	}
	if out != nil {
		s.output = out
	}
}

// DisableColors turns off ANSI color output.
func (s *Session) DisableColors() {
	s.useColors = false
}

func (s *Session) promptString() string {
	var prompt strings.Builder

	// Add session context if available
	if s.sessionID > 0 {
		prompt.WriteString(s.colorize(styleDim+colorBlue, fmt.Sprintf("[%d] ", s.sessionID)))
	}

	// Add timestamp if enabled
	if s.config.UI.ShowTimestamps {
		timestamp := time.Now().Format("15:04")
		prompt.WriteString(s.colorize(styleDim+colorGray, fmt.Sprintf("%s ", timestamp)))
	}

	// Main prompt
	prompt.WriteString(s.colorize(styleBold+colorCyan, "└─► "))

	return prompt.String()
}

func (s *Session) plainPromptString() string {
	var prompt strings.Builder

	// Add session context if available
	if s.sessionID > 0 {
		prompt.WriteString(fmt.Sprintf("[%d] ", s.sessionID))
	}

	// Add timestamp if enabled
	if s.config.UI.ShowTimestamps {
		timestamp := time.Now().Format("15:04")
		prompt.WriteString(fmt.Sprintf("%s ", timestamp))
	}

	// Main prompt
	prompt.WriteString("> ")

	return prompt.String()
}

func (s *Session) shouldUseLineEditor() bool {
	stdin, inOK := s.input.(*os.File)
	stdout, outOK := s.output.(*os.File)
	if !inOK || !outOK {
		return false
	}
	if stdin != os.Stdin || stdout != os.Stdout {
		return false
	}
	return term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd()))
}

func (s *Session) closeLineReader() {
	if s.lineReader != nil {
		s.lineReader.Close()
		s.lineReader = nil
	}
}
