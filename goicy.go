package main

import (
	"context"
	"fmt"
	"log/slog"

	"os"
	"os/signal"
	"syscall"

	"github.com/nicfit/goicy/config"
	"github.com/nicfit/goicy/logger"
	"github.com/nicfit/goicy/stream"
)

func main() {

	fmt.Println("=====================================================================")
	fmt.Println(" goicy v" + config.Version + " -- A hz reincarnate rewritten in Go")
	fmt.Println(" AAC/AACplus/AACplusV2 & MP1/MP2/MP3 Icecast/Shoutcast source client")
	fmt.Println(" Copyright (C) 2006-2016 Roman Butusov <reaxis at mail dot ru>")
	fmt.Println(" Copyright (C) 2026 Travis Shirk <travis at pobox dot com>")
	fmt.Println("=====================================================================")
	fmt.Println()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if len(os.Args) < 2 {
		fmt.Println("Usage: goicy <config.yml>")
		return
	}
	ymlfile := os.Args[1]

	logger.Term("Loading config...", slog.LevelDebug)
	cfg, err := config.LoadConfig(ymlfile)
	if err != nil {
		logger.Term(err.Error(), slog.LevelError)
		return
	}
	logger.Init(cfg.Logging)
	logger.Log("goicy v"+config.Version+" started", slog.LevelInfo)
	logger.Log("Loaded config file: "+ymlfile, slog.LevelInfo)

	_stream, err := stream.NewStreamPlayer(&cfg.Stream, &cfg.Playlist)
	if err != nil {
		logger.Error("error creating stream player", err)
		os.Exit(1)
	}

	ctx, cancel := _stream.Start(context.Background())
	select {
	case <-ctx.Done():
		logger.Log("Stream exited", slog.LevelInfo)
	case <-sigs:
		logger.Log("Aborted by user/SIGTERM", slog.LevelInfo)
		_stream.Stop()
		cancel(nil)
	}

	defer logger.Log("goicy exiting", slog.LevelInfo)
}
