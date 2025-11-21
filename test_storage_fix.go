package main

import (
	"fmt"
	"os"

	"github.com/ZaguanLabs/chatty/internal/config"
	"github.com/ZaguanLabs/chatty/internal/storage"
)

func main() {
	// Load configuration with empty storage path (default)
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Test that storage can be opened with empty path (should use default)
	store, err := storage.Open(cfg.Storage.Path)
	if err != nil {
		fmt.Printf("Error opening storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Test listing sessions
	sessions, err := store.ListSessions(nil, 0)
	if err != nil {
		fmt.Printf("Error listing sessions: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully listed %d sessions\n", len(sessions))
}