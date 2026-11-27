package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/nicfit/goicy/config"
	slogmulti "github.com/samber/slog-multi"
)

var (
	LogLevel slog.LevelVar
	Logger   = slog.Default()
)

func Init(cfg config.LoggingConfig) {
	if err := LogLevel.UnmarshalText([]byte(cfg.Level)); err != nil {
		LogLevel.Set(slog.LevelDebug)
	}

	term_handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: &LogLevel,
	})

	log_file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return
	}
	file_handler := slog.NewJSONHandler(log_file, &slog.HandlerOptions{
		Level: &LogLevel,
	})

	Logger = slog.New(slogmulti.Fanout(term_handler, file_handler))
}

func Log(msg string, level slog.Level, attrs ...slog.Attr) {
	Logger.LogAttrs(context.Background(), level, msg, attrs...)
}

func Error(msg string, err error, attrs ...slog.Attr) {
	attrs = append(attrs, slog.String("err", err.Error()))
	Logger.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
}

func Term(s string, level slog.Level) {
	if !Logger.Handler().Enabled(context.Background(), level) {
		return
	}
	fmt.Print("\r" + strings.Repeat(" ", 79) + "\r" + s)
}

func TermLn(s string, level slog.Level) {
	if !Logger.Handler().Enabled(context.Background(), level) {
		return
	}
	fmt.Println("\r" + strings.Repeat(" ", 79) + "\r" + s)
}
