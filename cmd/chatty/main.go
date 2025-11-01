package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/PromptShieldLabs/chatty/internal"
	"github.com/PromptShieldLabs/chatty/internal/config"
)

func main() {
	var configPath string
	var showVersion bool
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.Parse()

	// Show version and exit if requested
	if showVersion {
		fmt.Printf("Chatty v%s\n", internal.Version)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Create API client
	client, err := internal.NewClient(cfg.API.Key, cfg.API.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create client: %v\n", err)
		os.Exit(1)
	}

	// Create chat session
	session, err := internal.NewSession(client, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create session: %v\n", err)
		os.Exit(1)
	}

	// Run the chat loop
	ctx := context.Background()
	if err := session.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
