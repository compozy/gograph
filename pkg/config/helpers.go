package config

import (
	"fmt"
)

// GetProjectIDFromPath loads the configuration from the given path and returns the project ID.
// It searches for config files starting from the given path and traversing up the directory tree.
func GetProjectIDFromPath(path string) (string, error) {
	cfg, err := LoadProjectConfig(path)
	if err != nil {
		return "", fmt.Errorf("failed to load project config: %w", err)
	}

	if cfg.Project.ID == "" {
		return "", fmt.Errorf("project ID not found in configuration")
	}

	return cfg.Project.ID, nil
}

// EnsureProjectID returns the provided project ID if it's not empty, otherwise loads it from the config.
// This is useful for maintaining backward compatibility where project_id can be explicitly provided
// or auto-derived from the configuration file.
func EnsureProjectID(providedID, projectPath string) (string, error) {
	// If project ID is explicitly provided, use it
	if providedID != "" {
		return providedID, nil
	}

	// Otherwise, try to load from config
	return GetProjectIDFromPath(projectPath)
}
