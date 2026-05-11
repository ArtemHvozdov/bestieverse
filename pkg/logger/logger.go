package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// New creates a logger with colored console output.
// If logFile is non-empty, output is also written to that file.
func New(level, logFile string) zerolog.Logger {
	writers := []io.Writer{newConsoleWriter()}

	if logFile != "" {
		if err := os.MkdirAll(dirOf(logFile), 0755); err == nil {
			f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				writers = append(writers, f)
			}
		}
	}

	var out io.Writer
	if len(writers) == 1 {
		out = writers[0]
	} else {
		out = zerolog.MultiLevelWriter(writers...)
	}

	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	return zerolog.New(out).Level(lvl).With().Timestamp().Logger()
}

// NewWithWriter creates a logger that writes to the given writer (useful in tests).
func NewWithWriter(w io.Writer, level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	return zerolog.New(w).Level(lvl).With().Timestamp().Logger()
}

// WithChat returns a child logger with the chat_id field set.
func WithChat(log zerolog.Logger, chatID int64) zerolog.Logger {
	return log.With().Int64("chat", chatID).Logger()
}

// WithUser returns a child logger with the user field set as "(id|@username)".
// If username is empty, the format is "(id)".
func WithUser(log zerolog.Logger, userID int64, username string) zerolog.Logger {
	return log.With().Str("user", UserValue(userID, username)).Logger()
}

// UserValue formats a user identifier as "(id|username)" or "(id)" when username is empty.
// Use this when adding the user field inline to a log event.
func UserValue(userID int64, username string) string {
	if username == "" {
		return fmt.Sprintf("(%d)", userID)
	}
	return fmt.Sprintf("( %d | %s)", userID, username)
}

func newConsoleWriter() zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.DateTime,
	}
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
