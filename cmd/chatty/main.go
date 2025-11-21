package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ZaguanLabs/chatty/internal"
	"github.com/ZaguanLabs/chatty/internal/config"
	chattyErrors "github.com/ZaguanLabs/chatty/internal/errors"
	"github.com/ZaguanLabs/chatty/internal/storage"
	"github.com/ZaguanLabs/chatty/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	version = "0.4.0"
	commit  = "none"
	date    = "unknown"
)

// handleDirectQuestion processes a direct question from command line arguments
func handleDirectQuestion(configPath string, args []string) {
	// Check if this is a command (starts with /)
	if len(args) > 0 && strings.HasPrefix(args[0], "/") {
		handleCLICommand(configPath, args)
		return
	}

	// Join all arguments into a single question
	question := strings.Join(args, " ")

	// Load configuration securely
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Create API client securely
	client, err := internal.NewSecureClient(cfg.API.Key, cfg.API.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create secure client: %v\n", err)
		os.Exit(1)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create message with the question
	messages := []internal.Message{
		{Role: "user", Content: question},
	}

	// Get response from API
	response, err := client.Chat(ctx, messages, cfg.Model.Name, cfg.Model.Temperature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Output the response directly
	fmt.Print(response)
}

// handleCLICommand processes slash commands in CLI mode
func handleCLICommand(configPath string, args []string) {
	command := args[0]
	commandArgs := args[1:]

	// Load configuration for commands that need it
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "/help":
		showCLIHelp()
	case "/list", "/sessions":
		handleListCommand(cfg)
	case "/load":
		if len(commandArgs) == 0 {
			fmt.Fprintf(os.Stderr, "Usage: ./chatty /load <session-id>\n")
			os.Exit(1)
		}
		handleLoadCommand(cfg, commandArgs[0])
	case "/history":
		fmt.Println("History command is only available in interactive mode.")
		fmt.Println("Use './chatty' to start an interactive session.")
	case "/reset", "/clear":
		fmt.Println("Reset command is only available in interactive mode.")
		fmt.Println("Use './chatty' to start an interactive session.")
	case "/markdown":
		fmt.Println("Markdown toggle is only available in interactive mode.")
		fmt.Println("Use './chatty' to start an interactive session.")
	case "/exit", "/quit":
		// No-op in CLI mode
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Use './chatty /help' to see available commands.\n")
		os.Exit(1)
	}
}

// showCLIHelp displays help for CLI mode
func showCLIHelp() {
	fmt.Println("Chatty CLI Commands")
	fmt.Println("==================")
	fmt.Println()
	fmt.Println("Direct Questions:")
	fmt.Println("  ./chatty \"What is an LLM?\"           Ask a question directly")
	fmt.Println("  ./chatty \"Explain Go in detail\"       Multi-word questions")
	fmt.Println()
	fmt.Println("Session Management:")
	fmt.Println("  ./chatty /list                         List saved conversations")
	fmt.Println("  ./chatty /sessions                     Alias for /list")
	fmt.Println("  ./chatty /load <id>                    Load a saved conversation")
	fmt.Println()
	fmt.Println("Other Commands:")
	fmt.Println("  ./chatty /help                         Show this help")
	fmt.Println("  ./chatty /exit                         Exit (no-op in CLI mode)")
	fmt.Println()
	fmt.Println("Interactive Mode:")
	fmt.Println("  ./chatty                               Start interactive TUI session")
	fmt.Println("  ./chatty --config <path>               Use custom config file")
	fmt.Println()
	fmt.Println("For more commands, use interactive mode with './chatty'")
}

// handleListCommand lists saved sessions
func handleListCommand(cfg *config.Config) {
	// Initialize storage
	store, err := storage.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := context.Background()
	sessions, err := store.ListSessions(ctx, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No saved sessions found.")
		return
	}

	fmt.Println("Saved Sessions:")
	fmt.Println("===============")
	for _, session := range sessions {
		title := session.Name
		if strings.TrimSpace(title) == "" {
			title = "Untitled session"
		}
		fmt.Printf("#%d: %s\n", session.ID, title)
		fmt.Printf("     %d messages • Last updated %s\n", session.MessageCount, formatRelative(session.UpdatedAt))
		fmt.Println()
	}
}

// handleLoadCommand loads and displays a saved session
func handleLoadCommand(cfg *config.Config, sessionIDStr string) {
	// Parse session ID
	sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid session ID: %v\n", err)
		os.Exit(1)
	}

	// Initialize storage
	store, err := storage.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := context.Background()
	transcript, err := store.LoadSession(ctx, sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load session: %v\n", err)
		os.Exit(1)
	}

	title := transcript.Summary.Name
	if strings.TrimSpace(title) == "" {
		title = "Untitled session"
	}

	fmt.Printf("Session #%d: %s\n", transcript.Summary.ID, title)
	fmt.Printf("%d messages • Created %s\n", len(transcript.Messages), transcript.Summary.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Println(strings.Repeat("=", 50))

	for _, msg := range transcript.Messages {
		timestamp := msg.CreatedAt.Format("15:04")
		if msg.Role == "user" {
			fmt.Printf("\n[%s] User:\n", timestamp)
		} else {
			fmt.Printf("\n[%s] Assistant:\n", timestamp)
		}
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(msg.Content)
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("End of session #%d\n", transcript.Summary.ID)
}

// formatRelative formats a time relative to now
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

func main() {
	// Set error security level to production by default
	chattyErrors.SetErrorSecurityLevel(chattyErrors.ErrorLevelProduction)

	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	// Check if a direct question was provided
	args := flag.Args()
	if len(args) > 0 {
		// Direct question mode
		handleDirectQuestion(configPath, args)
		return
	}

	// Load configuration securely
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Create API client securely - the client will handle the API key securely
	client, err := internal.NewSecureClient(cfg.API.Key, cfg.API.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create secure client: %v\n", err)
		os.Exit(1)
	}

	// Clean version string
	cleanVersion := strings.TrimPrefix(version, "v")
	if commit != "none" && commit != "" {
		cleanVersion = fmt.Sprintf("%s (build %s)", cleanVersion, commit)
	}

	// Start TUI
	model := tui.NewModel(client, cfg, nil)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}