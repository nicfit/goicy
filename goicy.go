package main

import (
	"fmt"

	"github.com/nicfit/goicy/config"
	"github.com/nicfit/goicy/daemon"
	"github.com/nicfit/goicy/logger"
	"github.com/nicfit/goicy/playlist"
	"github.com/nicfit/goicy/stream"
	"github.com/nicfit/goicy/util"

	"os"
	"os/signal"
	"runtime"
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
		fmt.Printf("Usage: %s <inifile>", os.Args[0])
		return 1
	}
	inifile := string(os.Args[1])

	logger.TermLn("Loading config...", logger.LOG_DEBUG)
	err := config.LoadConfig(inifile)
	if err != nil {
		logger.TermLn(err.Error(), logger.LOG_ERROR)
		return 1
	}
	logger.File("---------------------------", logger.LOG_INFO)
	logger.File("goicy v"+config.Version+" started", logger.LOG_INFO)
	logger.Log("Loaded config file: "+inifile, logger.LOG_INFO)

	// daemonizing
	if config.Cfg.IsDaemon && runtime.GOOS == "linux" {
		logger.Log("Daemon mode, detaching from terminal...", logger.LOG_INFO)

		cntxt := &daemon.Context{
			PidFileName: config.Cfg.PidFile,
			PidFilePerm: 0644,
			//LogFileName: "log",
			//LogFilePerm: 0640,
			WorkDir: "./",
			Umask:   027,
			//Args:        []string{"[goicy sample]"},
		}

		d, err := cntxt.Reborn()
		if err != nil {
			logger.File(err.Error(), logger.LOG_ERROR)
			return 1
		}
		if d != nil {
			logger.File("Parent process died", logger.LOG_INFO)
			return 1
		}
		defer cntxt.Release()
		logger.Log("Daemonized successfully", logger.LOG_INFO)
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
