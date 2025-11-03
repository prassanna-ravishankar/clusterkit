package log

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var (
	// Logger is the global logger instance
	Logger *logrus.Logger
)

func init() {
	Logger = logrus.New()
	Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	Logger.SetLevel(logrus.InfoLevel)
}

// Config contains logging configuration
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // text, json
	File       string // Log file path (optional)
	MaxSizeMB  int    // Max size before rotation (optional)
	MaxBackups int    // Max number of old log files to keep (optional)
}

// Setup configures the global logger
func Setup(cfg *Config) error {
	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		return err
	}
	Logger.SetLevel(level)

	// Set formatter
	switch cfg.Format {
	case "json":
		Logger.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
			DisableColors: cfg.File != "", // Disable colors when logging to file
		})
	default:
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// Configure output
	if cfg.File != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		// Open log file
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}

		// Write to both file and stdout
		Logger.SetOutput(io.MultiWriter(os.Stdout, file))
	} else {
		Logger.SetOutput(os.Stdout)
	}

	return nil
}

// WithField adds a field to the log entry
func WithField(key string, value interface{}) *logrus.Entry {
	return Logger.WithField(key, value)
}

// WithFields adds multiple fields to the log entry
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Logger.WithFields(fields)
}

// WithError adds an error to the log entry
func WithError(err error) *logrus.Entry {
	return Logger.WithError(err)
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	Logger.Debug(args...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

// Info logs an info message
func Info(args ...interface{}) {
	Logger.Info(args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	Logger.Warn(args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	Logger.Error(args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	Logger.Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
}

// SetLevel sets the logging level
func SetLevel(level string) error {
	l, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	Logger.SetLevel(l)
	return nil
}

// GetLevel returns the current logging level
func GetLevel() string {
	return Logger.Level.String()
}

// GetLogger returns the global logger instance
func GetLogger() *logrus.Logger {
	return Logger
}
