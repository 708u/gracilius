package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir returns the path to the gracilius data directory.
// Uses os.UserConfigDir ($XDG_CONFIG_HOME, falling back to ~/.config).
func DataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get config directory: %w", err)
	}
	return filepath.Join(configDir, "gracilius"), nil
}
