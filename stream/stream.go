package stream

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/nicfit/bsides"
	"github.com/nicfit/bsides/factory"
	"github.com/nicfit/bsides/iters"
	"github.com/nicfit/goicy/aac"
	"github.com/nicfit/goicy/config"
	"github.com/nicfit/goicy/cuesheet"
	"github.com/nicfit/goicy/logger"
	"github.com/nicfit/goicy/metadata"
	"github.com/nicfit/goicy/mpeg"
	"github.com/nicfit/goicy/network"
	"github.com/nicfit/goicy/util"
)

type Stream struct {
	totalFramesSent uint64
	totalTimeBegin  time.Time
	abort           bool
	config          *config.StreamConfig
	serverConn      network.Network
	metadata        metadata.Metadata
}

func NewStream(config *config.StreamConfig) *Stream {
	return &Stream{
		config:   config,
		metadata: metadata.Metadata{Config: config},
	}
}

func (s *Stream) Abort() {
	s.abort = true
}
func (s *Stream) IsAborted() bool {
	return s.abort
}

func (s *Stream) StreamFile(filename string) (res error) {
	var (
		br                  float64
		spf, sr, frames, ch int
	)

	cleanUp := func(err error) {
		if err != nil {
			_ = s.serverConn.Close()
			s.serverConn = nil
		}
		res = err
	}

	logger.Log("StreamFile: "+filename+"...", slog.LevelInfo)
	logger.Log("Checking file: "+filename+"...", slog.LevelInfo)

	var err error
	if s.config.Format == "mpeg" {
		err = mpeg.GetFileInfo(filename, &br, &spf, &sr, &frames, &ch)
	} else {
		err = aac.GetFileInfo(filename, &br, &spf, &sr, &frames, &ch)
	}
	if err != nil {
		return err
	}

	if s.serverConn == nil {
		s.serverConn, err = network.ConnectServer(s.config, int(br), sr, ch)
		if err != nil {
			logger.Error("Cannot connect to server", err)
			return err
		}
	}

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	if s.config.Format == "mpeg" {
		mpeg.SeekTo1StFrame(*f)
	} else {
		aac.SeekTo1StFrame(*f)
	}

	logger.Log("Streaming file: "+filename+"...", slog.LevelInfo)

	cuefile := util.Basename(filename) + ".cue"
	if s.config.UpdateMetadata {
		go func() {
			_ = s.metadata.GetTagsFFMPEG(filename)
		}()
		cuesheet.Load(cuefile)
	}

	logger.Term("CTRL-C to stop", slog.LevelInfo)

	framesSent := 0

	// get number of frames to read in 1 iteration
	framesToRead := (sr / spf) + 1
	timeBegin := time.Now()

	for framesSent < frames {
		sendBegin := time.Now()

		var lbuf []byte
		if s.config.Format == "mpeg" {
			lbuf, err = mpeg.GetFrames(*f, framesToRead)
		} else {
			lbuf, err = aac.GetFrames(*f, framesToRead)
		}
		if errors.Is(err, io.EOF) {
			logger.Log("Stream complete", slog.LevelInfo)
			return nil
		} else if err != nil {
			logger.Error("Error reading data stream", err)
			cleanUp(err)
			return err
		}

		if err := s.serverConn.Send(lbuf); err != nil {
			cleanUp(err)
			logger.Error("Error sending data stream", err)
			return err
		}

		framesSent = framesSent + framesToRead

		timeElapsed := int(float64((time.Since(timeBegin)).Seconds()) * 1000)
		timeSent := int(float64(framesSent) * float64(spf) / float64(sr) * 1000)

		bufferSent := 0
		if timeSent > timeElapsed {
			bufferSent = timeSent - timeElapsed
		}

		if s.config.UpdateMetadata {
			cuesheet.Update(uint32(timeElapsed), s.metadata)
		}

		// calculate the send lag
		sendLag := int(float64((time.Since(sendBegin)).Seconds()) * 1000)

		if timeElapsed > 1500 {
			logger.Term("Frames: "+strconv.Itoa(framesSent)+"/"+strconv.Itoa(frames)+"  Time: "+
				strconv.Itoa(timeElapsed/1000)+"/"+strconv.Itoa(timeSent/1000)+"s  Buffer: "+
				strconv.Itoa(bufferSent)+"ms  Frames/Bytes: "+strconv.Itoa(framesToRead)+"/"+strconv.Itoa(len(lbuf)), slog.LevelInfo)
		}

		// regulate sending rate
		timePause := 0
		if bufferSent < (s.config.BufferSize - 100) {
			timePause = 900 - sendLag
		} else {
			if bufferSent > s.config.BufferSize {
				timePause = 1100 - sendLag
			} else {
				timePause = 975 - sendLag
			}
		}

		if s.abort {
			err := errors.New("aborted by user")
			cleanUp(err)
			return err
		}

		time.Sleep(time.Millisecond * time.Duration(timePause))
	}

	// pause to clear up the buffer
	timeBetweenTracks := int(((float64(frames)*float64(spf))/float64(sr))*1000) - int(float64((time.Since(timeBegin)).Seconds())*1000)
	logger.Log("Pausing for "+strconv.Itoa(timeBetweenTracks)+"ms...", slog.LevelDebug)
	time.Sleep(time.Duration(time.Millisecond) * time.Duration(timeBetweenTracks))

	return res
}

func (s *Stream) StreamFFMPEG(filename string) (res error) {
	var (
		err error
		cmd *exec.Cmd
	)

	cleanUp := func(err error) {
		logger.Log("Killing ffmpeg..", slog.LevelDebug)
		if err := cmd.Process.Kill(); err != nil {
			logger.Log(fmt.Sprintf("Error killing ffmpeg: %s", err), slog.LevelError)
		}
		_ = s.serverConn.Close()
		s.serverConn = nil
		s.totalFramesSent = 0
		res = err
	}

	logger.Log("StreamFFMPEG: "+filename+"...", slog.LevelInfo)
	if s.serverConn == nil {
		var err error
		s.serverConn, err = network.ConnectServer(s.config,
			s.config.FFMPEG.ReEncode.Bitrate/1000, s.config.FFMPEG.ReEncode.SampleRate,
			s.config.FFMPEG.ReEncode.Channels,
		)
		if err != nil {
			logger.Error("Cannot connect to server", err)
			return err
		}
	}

	var cmdArgs []string
	profile := ""
	if s.config.Format == "mpeg" {
		profile = "MPEG"
		if s.config.FFMPEG.ReEncode.Enabled {
			cmdArgs = []string{
				"-i", filename,
				"-c:a", "libmp3lame",
				"-b:a", strconv.Itoa(s.config.FFMPEG.ReEncode.Bitrate),
				"-cutoff", "20000",
				"-ar", strconv.Itoa(s.config.FFMPEG.ReEncode.SampleRate),
				"-ac", strconv.Itoa(s.config.FFMPEG.ReEncode.Channels),
				"-f", "mp3",
				"-write_xing", "0",
				"-id3v2_version", "0",
				"-loglevel", "fatal",
				"-",
			}
		} else {
			cmdArgs = []string{
				"-i", filename,
				"-c:a", "copy",
				"-f", "mp3",
				"-write_xing", "0",
				"-id3v2_version", "0",
				"-loglevel", "fatal",
				"-",
			}
		}
	} else {
		switch s.config.FFMPEG.ReEncode.AACProfile {
		case "lc":
			profile = "aac_low"
		case "he":
			profile = "aac_he"
		default:
			profile = "aac_he_v2"
		}
		if s.config.FFMPEG.ReEncode.Enabled {
			cmdArgs = []string{
				"-i", filename,
				"-c:a", "libfdk_aac",
				"-profile:a", profile,
				"-b:a", strconv.Itoa(s.config.FFMPEG.ReEncode.Bitrate),
				"-cutoff", "20000",
				"-ar", strconv.Itoa(s.config.FFMPEG.ReEncode.SampleRate),
				"-ac", strconv.Itoa(s.config.FFMPEG.ReEncode.Channels),
				"-f", "adts",
				"-loglevel", "fatal",
				"-",
			}
		} else {
			cmdArgs = []string{
				"-i", filename,
				"-c:a", "copy",
				"-f", "adts",
				"-loglevel", "fatal",
				"-",
			}
		}
	}

	logger.Log("Starting ffmpeg: "+s.config.FFMPEG.Path, slog.LevelDebug)
	if s.config.FFMPEG.ReEncode.Enabled {
		logger.Log("Format         : "+profile, slog.LevelDebug)
		logger.Log("Bitrate        : "+strconv.Itoa(s.config.FFMPEG.ReEncode.Bitrate), slog.LevelDebug)
		logger.Log("Samplerate     : "+strconv.Itoa(s.config.FFMPEG.ReEncode.SampleRate), slog.LevelDebug)
	} else {
		logger.Log("Format        : source, no reencoding", slog.LevelDebug)
	}

	cmd = exec.Command(s.config.FFMPEG.Path, cmdArgs...)

	f, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		logger.Log("Error starting ffmpeg", slog.LevelError)
		logger.Log(err.Error(), slog.LevelError)
		return err
	}

	// log stderr output from ffmpeg
	go func() {
		in := bufio.NewScanner(stderr)
		for in.Scan() {
			logger.Log("FFMPEG: "+in.Text(), slog.LevelDebug)
		}
	}()

	logger.Log("Streaming file: "+filename+"...", slog.LevelInfo)

	cuefile := util.Basename(filename) + ".cue"
	if s.config.UpdateMetadata {
		go func() {
			_ = s.metadata.GetTagsFFMPEG(filename)
		}()
		cuesheet.Load(cuefile)
	}

	logger.TermLn("CTRL-C to stop", slog.LevelInfo)

	frames := 0
	timeFileBegin := time.Now()

	sr := 0
	spf := 0
	framesToRead := 1

	for {
		sendBegin := time.Now()

		var lbuf []byte
		if s.config.Format == "mpeg" {
			lbuf, err = mpeg.GetFramesStdin(f, framesToRead)
			if framesToRead == 1 {
				if len(lbuf) < 4 {
					logger.Log("Error reading data stream", slog.LevelError)
					cleanUp(err)
					break
				}
				sr = mpeg.GetSR(lbuf[0:4])
				if sr == 0 {
					logger.Log("Erroneous MPEG sample rate from data stream", slog.LevelError)
					cleanUp(err)
					break
				}
				spf = mpeg.GetSPF(lbuf[0:4])
				framesToRead = (sr / spf) + 1
				mbuf, _ := mpeg.GetFramesStdin(f, framesToRead-1)
				lbuf = append(lbuf, mbuf...)
			}
		} else {
			lbuf, err = aac.GetFramesStdin(f, framesToRead)
			if framesToRead == 1 {
				if len(lbuf) < 7 {
					logger.Log("Error reading data stream", slog.LevelError)
					cleanUp(err)
					break
				}
				sr = aac.GetSR(lbuf[0:7])
				if sr == 0 {
					logger.Log("Erroneous AAC sample rate from data stream", slog.LevelError)
					cleanUp(err)
					break
				}
				spf = aac.GetSPF(lbuf[0:7])
				framesToRead = (sr / spf) + 1
				mbuf, _ := aac.GetFramesStdin(f, framesToRead-1)
				lbuf = append(lbuf, mbuf...)
			}
		}

		if err != nil {
			logger.Log("Error reading data stream", slog.LevelError)
			cleanUp(err)
			break
		}

		if len(lbuf) <= 0 {
			logger.Log("STDIN from ffmpeg ended", slog.LevelDebug)
			break
		}

		if s.totalFramesSent == 0 {
			s.totalTimeBegin = time.Now()
			//stdoutFramesSent = 0
		}

		if err := s.serverConn.Send(lbuf); err != nil {
			logger.Log("Error sending data stream", slog.LevelError)
			cleanUp(err)
			break
		}

		s.totalFramesSent = s.totalFramesSent + uint64(framesToRead)
		frames = frames + framesToRead

		timeElapsed := int(float64((time.Since(s.totalTimeBegin)).Seconds()) * 1000)
		timeSent := int(float64(s.totalFramesSent) * float64(spf) / float64(sr) * 1000)
		timeFileElapsed := int(float64((time.Since(timeFileBegin)).Seconds()) * 1000)

		bufferSent := 0
		if timeSent > timeElapsed {
			bufferSent = timeSent - timeElapsed
		}

		if s.config.UpdateMetadata {
			cuesheet.Update(uint32(timeFileElapsed), s.metadata)
		}

		// calculate the send lag
		sendLag := int(float64((time.Since(sendBegin)).Seconds()) * 1000)

		if timeElapsed > 1500 {
			logger.Term("Frames: "+strconv.Itoa(frames)+"/"+strconv.Itoa(int(s.totalFramesSent))+"  Time: "+
				strconv.Itoa(int(timeElapsed/1000))+"/"+strconv.Itoa(int(timeSent/1000))+"s  Buffer: "+
				strconv.Itoa(int(bufferSent))+"ms  Frames/Bytes: "+strconv.Itoa(framesToRead)+"/"+strconv.Itoa(len(lbuf)),
				slog.LevelDebug)
		}

		// regulate sending rate
		timePause := 0
		if bufferSent < (s.config.BufferSize - 100) {
			timePause = 900 - sendLag
		} else {
			if bufferSent > s.config.BufferSize {
				timePause = 1100 - sendLag
			} else {
				timePause = 975 - sendLag
			}
		}

		if s.abort {
			err := errors.New("aborted by user")
			cleanUp(err)
			break
		}

		time.Sleep(time.Millisecond * time.Duration(timePause))
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	logger.Log("ffmpeg is dead. hoy!", slog.LevelDebug)

	//logger.Log(strconv.Itoa(cmd.ProcessState), logger.LOG_DEBUG)
	return res
}

func NewStreamPlayer(config *config.StreamConfig, playlist_cfg *config.PlaylistConfig) (*Player, error) {
	var (
		iterator      bsides.Iterator
		playlist, err = factory.Load(playlist_cfg.Path)
	)
	if playlist_cfg.Shuffle {
		iterator = iters.NewShuffleIterator(playlist)
	} else {
		iterator = bsides.NewSequentialIterator(playlist)
	}
	if playlist_cfg.Repeat > 0 {
		iterator = iters.NewRepeatingIterator(iterator, playlist_cfg.Repeat)
	}

	if err != nil {
		return nil, fmt.Errorf("unable to load playlist: %w", err)
	}

	return &Player{
		config:   config,
		playlist: playlist,
		iterator: iterator,
	}, nil
}

type Player struct {
	goicy    *Stream
	config   *config.StreamConfig
	playlist bsides.Playlist
	iterator bsides.Iterator
}

func (s *Player) Start(ctx context.Context) (context.Context, context.CancelCauseFunc) {
	s.goicy = NewStream(s.config)

	var cancel context.CancelCauseFunc
	ctx, cancel = context.WithCancelCause(ctx)

	go func() {
		var err error
		defer cancel(err)

		for s.iterator.HasNext() {
			var next bsides.Entry

			// Check for cancellation
			select {
			case <-ctx.Done():
				s.Stop()
				err = ctx.Err()
				logger.Error("stream cancelled", err)
				return
			default:
				next = s.iterator.Next()
				if next.IsZero() {
					logger.Log("End of playlist", slog.LevelInfo)
					return
				}
			}

			track := next.Item.(bsides.Track)
			logger.Log(fmt.Sprintf("Next track: %v", track), slog.LevelDebug)
			if stream_err := s.doStream(track); stream_err != nil {
				err = stream_err
				return
			}
		}

		s.Stop()
		logger.Log("Stream "+s.config.Mount+" ended", slog.LevelInfo, slog.String("err", err.Error()))
	}()

	return ctx, cancel
}

func (s *Player) Stop() {
	s.goicy.Abort()
}

func (s *Player) doStream(track bsides.Track) error {
	var (
		retries    = 0
		stream_err error
	)
	for retries < s.config.NumRetries {
		stream_err = nil
		switch s.config.FFMPEG.Enabled {
		case true:
			if stream_err = s.goicy.StreamFFMPEG(track.URI()); stream_err != nil {
				stream_err = fmt.Errorf("StreamFFMPEG error: %w", stream_err)
			}
		case false:
			if stream_err = s.goicy.StreamFile(track.URI()); stream_err != nil {
				stream_err = fmt.Errorf("StreamFile error: %w", stream_err)
			}
		}

		if stream_err == nil {
			return nil
		}

		logger.Error("doStream error:", stream_err)
		if s.goicy.IsAborted() {
			return stream_err
		}

		retries++
		sleep_t := time.Second * time.Duration(3*retries)
		logger.Log(fmt.Sprintf("%d/%d retry stream in %s...",
			retries, s.config.NumRetries, sleep_t), slog.LevelWarn)
		time.Sleep(sleep_t)
	}

	return stream_err
}
