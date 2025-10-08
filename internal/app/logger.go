package app

import "strings"

import "github.com/charlesng35/shellcn/pkg/logger"

// ConfigureLogging initialises the global logger with the provided level, defaulting to info.
func ConfigureLogging(level string) error {
	level = strings.TrimSpace(level)
	if level == "" {
		level = "info"
	}
	return logger.Init(level)
}
