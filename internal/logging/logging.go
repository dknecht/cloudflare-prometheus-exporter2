package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

// InitializeLogger initializes the global logger with standard configurations.
func InitializeLogger() {
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{}) // Use JSON format for structured logs
	log.SetOutput(os.Stdout)                  // Log to standard output
	log.SetLevel(logrus.DebugLevel)           // Set log level
}

// Info logs informational messages.
func Info(message string, fields map[string]interface{}) {
	log.WithFields(fields).Info(message)
}

// Error logs error messages.
func Error(message string, fields map[string]interface{}) {
	log.WithFields(fields).Error(message)
}

// Debug logs debug messages.
func Debug(message string, fields map[string]interface{}) {
	log.WithFields(fields).Debug(message)
}
