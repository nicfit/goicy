package logger

import (
	"context"
	"fmt"
	"github.com/nicfit/goicy/config"
	"log/slog"
	"os"
	"strings"
)

var log = slog.New(slog.NewTextHandler(os.Stderr, nil))

func Init() error {

	level := new(slog.Level)
	if err := level.UnmarshalText([]byte(config.Cfg.LogLevel)); err != nil {
		return err
	}

	if config.Cfg.LogFile != "" {
		writer, err := os.OpenFile(config.Cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		log = slog.New(slog.NewTextHandler(writer, nil))
	} else {

	}

	return nil
}

const (
	LOG_ERROR = slog.LevelError
	LOG_INFO  = slog.LevelInfo
	LOG_DEBUG = slog.LevelDebug
)

func File(s string, level slog.Level) {
	log.Log(context.Background(), level, s)
}

func Term(s string, level slog.Level) {
	if log.Enabled(context.Background(), level) {
		fmt.Print("\r" + strings.Repeat(" ", 79) + "\r" + s)
	}
}

func TermLn(s string, level slog.Level) {
	if log.Enabled(context.Background(), level) {
		fmt.Println("\r" + strings.Repeat(" ", 79) + "\r" + s)
	}
}

// Log writes both to the terminal and the logger.
// Puts ln at the end of the logged string
func Log(s string, level slog.Level) {
	TermLn(s, level)
	File(s, level)
}
