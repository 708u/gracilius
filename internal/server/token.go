package server

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/708u/gracilius/internal/config"
	"github.com/google/uuid"
)

// loadOrCreateToken loads an existing token from ~/.config/gracilius/token or creates a new one.
func loadOrCreateToken() string {
	dataDir, err := config.DataDir()
	if err != nil {
		log.Printf("Failed to get data directory, using new token: %v", err)
		return uuid.New().String()
	}

	tokenPath := filepath.Join(dataDir, "token")

	// Try to read existing token
	data, err := os.ReadFile(tokenPath)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			log.Printf("Loaded existing token from %s", tokenPath)
			return token
		}
	}

	// Create new token
	token := uuid.New().String()

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Printf("Failed to create directory %s: %v", dataDir, err)
		return token
	}

	// Save token
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		log.Printf("Failed to save token to %s: %v", tokenPath, err)
	} else {
		log.Printf("Created new token at %s", tokenPath)
	}

	return token
}
