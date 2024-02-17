package main

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/nicfit/goicy/config"
	"github.com/nicfit/goicy/logger"
	"github.com/nicfit/goicy/playlist"
	"github.com/nicfit/goicy/stream"
	"github.com/nicfit/goicy/util"

	"os"
	"time"
)

func init() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		stream.Abort = true
		logger.Log("Aborted by user/SIGTERM", logger.LOG_INFO)
	}()
}

func Main() int {
	ctx := context.Background()

	fmt.Println("=====================================================================")
	fmt.Println(" goicy v" + config.Version + " -- A hz reincarnate rewritten in Go")
	fmt.Println(" AAC/AACplus/AACplusV2 & MP1/MP2/MP3 Icecast/Shoutcast source client")
	fmt.Println(" Copyright (C) 2006-2016 Roman Butusov <reaxis at mail dot ru>")
	fmt.Println(" Copyright (C) 2024 Travis Shirk <travis at pobox dot com>")
	fmt.Println("=====================================================================")
	fmt.Println()

	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <inifile>\n", os.Args[0])
		return 1
	}
	inifile := os.Args[1]

	logger.TermLn("Loading config...", logger.LOG_DEBUG)
	if err := config.LoadConfig(inifile); err != nil {
		logger.TermLn(err.Error(), logger.LOG_ERROR)
		return 1
	}
	if err := logger.Init(); err != nil {
		logger.TermLn(err.Error(), logger.LOG_ERROR)
		return 1
	}

	logger.File("---------------------------", logger.LOG_INFO)
	logger.File("goicy v"+config.Version+" started", logger.LOG_INFO)
	logger.Log("Loaded config file: "+inifile, logger.LOG_INFO)

	if config.Cfg.PidFile != "" {
		if err := os.WriteFile(config.Cfg.PidFile,
			[]byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
			logger.File(fmt.Sprintf("error '%s' writing PID file: %s", config.Cfg.PidFile, err), logger.LOG_ERROR)
		}
	}
	defer logger.Log("goicy exiting", logger.LOG_INFO)

	if err := playlist.Load(); err != nil {
		logger.Log("Cannot load playlist file", logger.LOG_ERROR)
		logger.Log(err.Error(), logger.LOG_ERROR)
		return 1
	}

	filename := playlist.Next()
	for !stream.Abort {
		if err := streamFile(ctx, playlist.Next()); err != nil {
			logger.Log(fmt.Sprintf("Failed to stream: %s", filename), logger.LOG_ERROR)
		}
	}

	return 0
}

func streamFile(_ context.Context, filename string) error {
	var (
		retries       = 0
		err     error = nil
		ctx           = context.Background()
	)
	for {
		if config.Cfg.StreamType == "file" {
			err = stream.StreamFile(ctx, filename)
		} else {
			err = stream.StreamFFMPEG(ctx, filename)
		}

		if err != nil {
			// if aborted break immediately
			if stream.Abort {
				break
			}
			retries++
			logger.Log("Error streaming: "+err.Error(), logger.LOG_ERROR)

			if retries == config.Cfg.ConnAttempts {
				logger.Log("No more retries", logger.LOG_INFO)
				break
			}
			var fileError *util.FileError
			if errors.As(err, &fileError) {
				filename = playlist.Next()
			}

			logger.Log("Retrying in 10 sec...", logger.LOG_INFO)
			for i := 0; i < 10; i++ {
				time.Sleep(time.Second * 1)
				if stream.Abort {
					break
				}
			}
			if stream.Abort {
				break
			}
		}
	}

	return err
}

func main() {
	os.Exit(Main())
}
