package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ZaguanLabs/chatty/internal"
	"github.com/ZaguanLabs/chatty/internal/config"
	"github.com/ZaguanLabs/chatty/internal/storage"
	"github.com/ZaguanLabs/chatty/internal/validation"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Message represents a chat message with its rendered view.
type Message struct {
	internal.Message
	Rendered string
}

// Model is the Bubble Tea model for the chat application.
type Model struct {
	client    *internal.Client
	cfg       *config.Config
	store     *storage.Store
	storagePath string
	sessionID int64

	viewport    viewport.Model
	textinput   textinput.Model
	renderer    *glamour.TermRenderer
	err         error

	// Chat State
	messages      []Message
	streaming     bool
	streamContent strings.Builder

	// Dimensions
	width  int
	height int
}

// NewModel initializes the TUI model.
func NewModel(client *internal.Client, cfg *config.Config, _ *storage.Store) Model {
	// Use textinput instead of textarea to avoid multi-line issues
	ti := textinput.New()
	ti.Placeholder = "Type your message here..."
	ti.Focus()
	ti.CharLimit = 10000

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to Chatty! Type a message to begin.\n")

	return Model{
		client:      client,
		cfg:         cfg,
		storagePath: cfg.Storage.Path,
		store:       nil, // Initialized asynchronously
		textinput:   ti,
		viewport:    vp,
		renderer:    nil, // Initialized asynchronously
		messages:    make([]Message, 0),
	}
}

// Init initializes the program.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	// Remove textarea.Blink to avoid input issues
	cmds = append(cmds, initRenderer(m.width))

	if m.storagePath != "disable" {
		cmds = append(cmds, loadStorage(m.storagePath))
	}

	return tea.Batch(cmds...)
}

// Msg types
type (
	streamChunkMsg struct {
		chunk string
		ch    chan string
	}
	streamErrorMsg error
	streamDoneMsg  struct{}
	errMsg         error
	sessionCreatedMsg int64
	storeLoadedMsg *storage.Store
	rendererLoadedMsg *glamour.TermRenderer
	sessionsListedMsg struct {
		sessions []storage.SessionSummary
		message  string
	}
	sessionLoadedMsg struct {
		transcript *storage.Transcript
	}
)

func initRenderer(width int) tea.Cmd {
	return func() tea.Msg {
		if width == 0 {
			width = 80
		}
		// Use a fixed dark style instead of WithAutoStyle to avoid terminal background detection
		renderer, err := glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(width-4),
		)
		if err != nil {
			return errMsg(err)
		}
		return rendererLoadedMsg(renderer)
	}
}

func loadStorage(path string) tea.Cmd {
	return func() tea.Msg {
		store, err := storage.Open(path)
		if err != nil {
			return errMsg(err)
		}
		return storeLoadedMsg(store)
	}
}

// Update handles events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textinput, tiCmd = m.textinput.Update(msg)
	// Only update viewport if we aren't streaming to avoid conflicts or if necessary
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Update viewport size
		headerHeight := 2
		footerHeight := 5 // textinput + padding
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - footerHeight
		
		// Update textarea width
		m.textinput.Width = msg.Width-4 // Account for padding/borders
		
		// Update renderer width if it exists
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithStylePath("dark"), // Use fixed dark style instead of auto detection
				glamour.WithWordWrap(msg.Width-4),
			)
			// Optional: Re-render history on resize for perfect wrapping
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.streaming {
				return m, nil // Ignore input while streaming
			}
			input := m.textinput.Value()
			if strings.TrimSpace(input) == "" {
				return m, nil
			}

			// Handle commands
			if strings.HasPrefix(input, "/") {
				m.textinput.Reset()
				return m.handleCommand(input)
			}

			m.textinput.Reset()
			return m.sendMessage(input)
		}

	// Streaming messages
	case streamChunkMsg:
		m.streamContent.WriteString(msg.chunk)
		// Append chunk to viewport efficiently
		// Ideally we'd append to the viewport content directly but Viewport doesn't support append easily.
		// Re-rendering the WHOLE history is what killed performance.
		// Instead, we construct the string: History (Pre-rendered) + Current Stream (Raw)
		content := m.renderHistoryCache() + "\n" + m.renderCurrentStream()
		m.viewport.SetContent(content)
		m.viewport.GotoBottom()
		return m, waitForChunk(msg.ch)

	case streamDoneMsg:
		m.streaming = false
		fullResponse := m.streamContent.String()
		
		// Render the full response once
		var rendered string
		var err error
		if m.renderer != nil {
			rendered, err = m.renderer.Render(fullResponse)
		}
		if err != nil || m.renderer == nil {
			rendered = fullResponse
		}

		// Add assistant message to history
		assistantMsg := Message{
			Message: internal.Message{Role: "assistant", Content: fullResponse},
			Rendered: rendered,
		}
		m.messages = append(m.messages, assistantMsg)
		
		// Persist
		if m.store != nil {
			go m.persistLastExchange()
		}

		m.viewport.SetContent(m.renderHistoryCache())
		m.viewport.GotoBottom()
		m.streamContent.Reset()
		return m, nil

	case streamErrorMsg:
		m.streaming = false
		m.err = error(msg)
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleError.Render(fmt.Sprintf("Error: %v", msg)))
		m.viewport.GotoBottom()
		return m, nil

	case sessionCreatedMsg:
		m.sessionID = int64(msg)
		return m, nil

	case storeLoadedMsg:
		m.store = msg
		return m, nil

	case rendererLoadedMsg:
		m.renderer = msg
		// Re-render all messages now that we have a renderer
		// This fixes the issue where early messages (or welcomed text) were plain text
		for i := range m.messages {
			rendered, err := m.renderer.Render(m.messages[i].Content)
			if err == nil {
				m.messages[i].Rendered = rendered
			}
		}
		m.viewport.SetContent(m.renderHistoryCache())
		return m, nil

	case errMsg:
		m.err = msg
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleError.Render(fmt.Sprintf("Error: %v", msg)))
		m.viewport.GotoBottom()
		return m, nil

	case sessionsListedMsg:
		return m.handleSessionsListed(msg)

	case sessionLoadedMsg:
		return m.handleSessionLoaded(msg)
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

// View renders the UI.
func (m Model) View() string {
	headerText := fmt.Sprintf("Chatty AI • %s", m.cfg.Model.Name)
	header := styleHeader.Render(headerText)

	// Use textinput instead of textarea
	textInputView := styleInput.Render(m.textinput.View())

	return fmt.Sprintf("%s\n%s\n%s",
		header,
		m.viewport.View(),
		textInputView,
	)
}

// Helper functions

func (m Model) renderHistoryCache() string {
	var b strings.Builder
	for _, msg := range m.messages {
		roleStyle := styleUserLabel
		name := "You"
		if msg.Role == "assistant" {
			roleStyle = styleAILabel
			name = "AI"
		}

		b.WriteString(roleStyle.Render(name + ":"))
		b.WriteString("\n")
		b.WriteString(msg.Rendered)
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderCurrentStream() string {
	return styleAILabel.Render("AI:") + "\n" + m.streamContent.String()
}

func (m Model) sendMessage(content string) (tea.Model, tea.Cmd) {
	// Render user message immediately
	var rendered string
	var err error
	if m.renderer != nil {
		rendered, err = m.renderer.Render(content)
	}
	if err != nil || m.renderer == nil {
		rendered = content
	}

	// Add user message
	m.messages = append(m.messages, Message{
		Message: internal.Message{Role: "user", Content: content},
		Rendered: rendered,
	})
	
	m.viewport.SetContent(m.renderHistoryCache())
	m.viewport.GotoBottom()

	var sessionCmd tea.Cmd
	// Ensure session (non-blocking)
	if m.store != nil && m.sessionID == 0 {
		sessionCmd = func() tea.Msg {
			ctx := context.Background()
			title := content
			if len(title) > 50 { title = title[:50] }
			id, err := m.store.CreateSession(ctx, title)
			if err != nil {
				return errMsg(err)
			}
			return sessionCreatedMsg(id)
		}
	}

	m.streaming = true
	m.streamContent.Reset()
	
	ch := make(chan string)
	
	// Start streaming command
	streamCmd := startStream(m.client, m.messages, m.cfg.Model.Name, m.cfg.Model.Temperature, ch)
	
	if sessionCmd != nil {
		return m, tea.Batch(sessionCmd, streamCmd)
	}
	return m, streamCmd
}

func startStream(client *internal.Client, messages []Message, model string, temp float64, ch chan string) tea.Cmd {
	// Convert back to internal.Message
	internalMessages := make([]internal.Message, len(messages))
	for i, msg := range messages {
		internalMessages[i] = msg.Message
	}

	return func() tea.Msg {
		go func() {
			ctx := context.Background()
			err := client.ChatStream(ctx, internalMessages, model, temp, func(chunk string) error {
				ch <- chunk
				return nil
			})
			if err != nil {
				// Send error through a side channel or just handle it?
				// Ideally we send a special message to ch or use a separate errCh.
				// For simplicity in this structure, we can't easily send error via ch (string).
				// But we should log it or something.
				// Actually, we can't send tea.Msg from here.
			}
			close(ch)
		}()
		return waitForChunk(ch)()
	}
}

func waitForChunk(ch chan string) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return streamChunkMsg{chunk: chunk, ch: ch}
	}
}

func (m Model) persistLastExchange() {
	if m.store == nil {
		return
	}
	if len(m.messages) < 2 {
		return
	}
	userMsg := m.messages[len(m.messages)-2].Message
	aiMsg := m.messages[len(m.messages)-1].Message
	
	ctx := context.Background()
	batch := []storage.Message{
		{Role: userMsg.Role, Content: userMsg.Content},
		{Role: aiMsg.Role, Content: aiMsg.Content},
	}
	m.store.AppendMessagesBatch(ctx, m.sessionID, batch)
}

func (m Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	// Validate command input
	if err := validation.ValidateCommand(input); err != nil {
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleError.Render("Invalid command: "+err.Error()))
		m.viewport.GotoBottom()
		return m, nil
	}

	// Sanitize command
	sanitizedCmd := validation.SanitizeInput(input, validation.MaxCommandLength)

	parts := strings.Fields(sanitizedCmd)
	cmd := parts[0]

	switch cmd {
	case "/exit", "/quit":
		return m, tea.Quit

	case "/clear", "/reset":
		m.messages = []Message{}
		m.viewport.SetContent("History cleared.")
		m.sessionID = 0
		return m, nil

	case "/help":
		help := `Available commands:
/exit, /quit           - Exit application
/clear, /reset         - Clear conversation history
/help                  - Show this help
/history               - Show conversation history
/markdown              - Toggle markdown rendering on/off
/list, /sessions       - List saved conversations
/load <id>             - Load a saved conversation by ID

You can also ask questions directly like:
"What is an LLM?" or "Explain Go programming"`
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleSystem.Render(help))
		m.viewport.GotoBottom()
		return m, nil

	case "/history":
		if len(m.messages) == 0 {
			m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleSystem.Render("No conversation history yet."))
		} else {
			history := "Conversation History:\n" + strings.Repeat("=", 50) + "\n"
			for i, msg := range m.messages {
				role := "User"
				if msg.Role == "assistant" {
					role = "Assistant"
				}
				history += fmt.Sprintf("[%d] %s:\n", i+1, role)
				history += strings.Repeat("-", 30) + "\n"
				history += msg.Content + "\n\n"
			}
			m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleSystem.Render(history))
		}
		m.viewport.GotoBottom()
		return m, nil

	case "/markdown":
		// Toggle markdown rendering
		// This would need to be implemented as a state in the model
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleSystem.Render("Markdown toggle functionality not yet implemented in TUI mode."))
		m.viewport.GotoBottom()
		return m, nil

	case "/list", "/sessions":
		return m.handleListCommand()

	case "/load":
		if len(parts) < 2 {
			m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleError.Render("Usage: /load <session-id>"))
			m.viewport.GotoBottom()
			return m, nil
		}
		return m.handleLoadCommand(parts[1])

	default:
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleError.Render("Unknown command: "+cmd+"\nUse /help to see available commands."))
		m.viewport.GotoBottom()
		return m, nil
	}
}

func (m Model) handleListCommand() (tea.Model, tea.Cmd) {
	if m.store == nil {
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleError.Render("Storage not available. Check your configuration."))
		m.viewport.GotoBottom()
		return m, nil
	}

	return m, func() tea.Msg {
		ctx := context.Background()
		sessions, err := m.store.ListSessions(ctx, 0)
		if err != nil {
			return errMsg(fmt.Errorf("failed to list sessions: %w", err))
		}

		if len(sessions) == 0 {
			return sessionsListedMsg{sessions: []storage.SessionSummary{}, message: "No saved sessions found."}
		}

		return sessionsListedMsg{sessions: sessions, message: ""}
	}
}

func (m Model) handleLoadCommand(sessionIDStr string) (tea.Model, tea.Cmd) {
	if m.store == nil {
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleError.Render("Storage not available. Check your configuration."))
		m.viewport.GotoBottom()
		return m, nil
	}

	// Parse session ID
	var sessionID int64
	if _, err := fmt.Sscanf(sessionIDStr, "%d", &sessionID); err != nil {
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleError.Render("Invalid session ID: "+sessionIDStr))
		m.viewport.GotoBottom()
		return m, nil
	}

	return m, func() tea.Msg {
		ctx := context.Background()
		transcript, err := m.store.LoadSession(ctx, sessionID)
		if err != nil {
			return errMsg(fmt.Errorf("failed to load session %d: %w", sessionID, err))
		}

		return sessionLoadedMsg{transcript: transcript}
	}
}

var styleSystem = lipgloss.NewStyle().Foreground(ColorSystem)

func (m Model) handleSessionsListed(msg sessionsListedMsg) (tea.Model, tea.Cmd) {
	if msg.message != "" {
		m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleSystem.Render(msg.message))
		m.viewport.GotoBottom()
		return m, nil
	}

	sessionsList := "Saved Sessions:\n" + strings.Repeat("=", 50) + "\n"
	for _, session := range msg.sessions {
		title := session.Name
		if strings.TrimSpace(title) == "" {
			title = "Untitled session"
		}
		sessionsList += fmt.Sprintf("#%d: %s\n", session.ID, title)
		sessionsList += fmt.Sprintf("     %d messages • Last updated %s\n\n",
			session.MessageCount, formatRelative(session.UpdatedAt))
	}

	m.viewport.SetContent(m.renderHistoryCache() + "\n" + styleSystem.Render(sessionsList))
	m.viewport.GotoBottom()
	return m, nil
}

func (m Model) handleSessionLoaded(msg sessionLoadedMsg) (tea.Model, tea.Cmd) {
	transcript := msg.transcript
	title := transcript.Summary.Name
	if strings.TrimSpace(title) == "" {
		title = "Untitled session"
	}

	// Clear current messages and load from transcript
	m.messages = make([]Message, 0, len(transcript.Messages))
	m.sessionID = transcript.Summary.ID

	// Convert storage messages to TUI messages
	for _, storageMsg := range transcript.Messages {
		tuiMsg := Message{
			Message: internal.Message{
				Role:    storageMsg.Role,
				Content: storageMsg.Content,
			},
			Rendered: "", // Will be rendered when renderer is available
		}

		// Render if renderer is available
		if m.renderer != nil {
			rendered, err := m.renderer.Render(storageMsg.Content)
			if err == nil {
				tuiMsg.Rendered = rendered
			} else {
				tuiMsg.Rendered = storageMsg.Content
			}
		} else {
			tuiMsg.Rendered = storageMsg.Content
		}

		m.messages = append(m.messages, tuiMsg)
	}

	// Update viewport content
	m.viewport.SetContent(m.renderHistoryCache())
	m.viewport.GotoBottom()

	// Show success message
	successMsg := fmt.Sprintf("Loaded session #%d: %s\n%d messages loaded",
		transcript.Summary.ID, title, len(transcript.Messages))
	m.viewport.SetContent(m.viewport.View() + "\n" + styleSystem.Render(successMsg))
	m.viewport.GotoBottom()

	return m, nil
}

// formatRelative formats a time relative to now (copied from main.go)
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
