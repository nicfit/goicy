package metadata

import (
	"encoding/base64"
	"log/slog"
	"net/url"
	"os/exec"
	"strings"

	"github.com/nicfit/goicy/config"
	"github.com/nicfit/goicy/logger"
	"github.com/nicfit/goicy/network"
	"gopkg.in/ini.v1"
)

type Metadata struct {
	Config *config.StreamConfig
}

func (m *Metadata) FormatMetadata(artist, title string) string {
	md := ""
	if artist != "" {
		md = artist + " - " + title
	} else {
		md = title
	}
	if md == "" {
		md = m.Config.Name
	}
	return md
}

func (m *Metadata) SendMetadata(metadata string) error {
	logger.Log("Setting metadata: "+metadata, slog.LevelInfo)
	sock, err := network.Connect(m.Config.Server.Host, m.Config.Server.Port)
	if err != nil {
		return err
	}

	headers := ""
	if m.Config.Server.ServerType() == "shoutcast" {
		headers = "GET /admin.cgi?pass=" + url.QueryEscape(m.Config.Server.Password) +
			"&mode=updinfo&song=" + strings.ReplaceAll(url.QueryEscape(metadata), "+", "%20") + " HTTP/1.0\r\n" +
			"User-Agent: (Mozilla Compatible)\r\n\r\n"
	} else {
		headers = "GET /admin/metadata?mode=updinfo&mount=/" + m.Config.Mount +
			"&song=" + strings.ReplaceAll(url.QueryEscape(metadata), "+", "%20") + " HTTP/1.0\r\n" +
			"User-Agent: goicy/" + config.Version + "\r\n" +
			"Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("source:"+m.Config.Server.Password)) + "\r\n\r\n"
	}
	if err := sock.Send([]byte(headers)); err != nil {
		return err
	}
	return nil
}

func (m *Metadata) GetTagsFFMPEG(filename string) error {
	cmdName := m.Config.FFMPEG.Path
	cmdArgs := []string{
		"-i", filename,
		"-f", "ffmetadata",
		"-",
	}

	logger.Log("Launching FFMPEG to read tags...", slog.LevelDebug)
	cmd := exec.Command(cmdName, cmdArgs...)

	out, err := cmd.Output()
	if err != nil {
		return err
	}

	ini, err := ini.Load(out)
	if err != nil {
		return err
	}

	section, _ := ini.GetSection("")
	artist := section.Key("artist").Value()
	if artist == "" {
		artist = section.Key("ARTIST").Value()
	}

	title := section.Key("title").Value()
	if title == "" {
		title = section.Key("TITLE").Value()
	}

	logger.Log("Artist: "+artist, slog.LevelDebug)
	logger.Log("Title: "+title, slog.LevelDebug)

	// format metadata
	metadata := m.FormatMetadata(artist, title)

	// send it
	if err := m.SendMetadata(metadata); err != nil {
		return err
	}

	return nil
}
