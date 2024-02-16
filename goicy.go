package main

import (
	"fmt"

	"github.com/nicfit/goicy/config"
	"github.com/nicfit/goicy/logger"
	"github.com/nicfit/goicy/playlist"
	"github.com/nicfit/goicy/stream"
	"github.com/nicfit/goicy/util"

	"os"
	"os/signal"
	"syscall"
	"time"
)

func Main() int {

	fmt.Println("=====================================================================")
	fmt.Println(" goicy v" + config.Version + " -- A hz reincarnate rewritten in Go")
	fmt.Println(" AAC/AACplus/AACplusV2 & MP1/MP2/MP3 Icecast/Shoutcast source client")
	fmt.Println(" Copyright (C) 2006-2016 Roman Butusov <reaxis at mail dot ru>")
	fmt.Println(" Copyright (C) 2024 Travis Shirk <travis at pobox dot com>")
	fmt.Println("=====================================================================")
	fmt.Println()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		stream.Abort = true
		logger.Log("Aborted by user/SIGTERM", logger.LOG_INFO)
	}()

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

	retries := 0
	filename := playlist.Next()
	for {
		var err error
		if config.Cfg.StreamType == "file" {
			err = stream.StreamFile(filename)
		} else {
			err = stream.StreamFFMPEG(filename)
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
			// if that was a file error
			switch err.(type) {
			case *util.FileError:
				filename = playlist.Next()
			default:

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
			continue
		}
		retries = 0
		filename = playlist.Next()
	}

	return 0
}

func main() {
	os.Exit(Main())
}
