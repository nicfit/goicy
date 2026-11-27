package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Stream   StreamConfig   `yaml:"stream"`
	Playlist PlaylistConfig `yaml:"playlist"`
	Logging  LoggingConfig  `yaml:"logging"`
}

const Version = "0.4"

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

type StreamConfig struct {
	Mount          string       `yaml:"mount"`
	Format         string       `yaml:"format"`
	Server         ServerConfig `yaml:"server"`
	Name           string       `yaml:"name"`
	Desc           string       `yaml:"description"`
	Genre          string       `yaml:"genre"`
	Public         bool         `yaml:"public"`
	URL            string       `yaml:"url"`
	FFMPEG         FfmpegConfig `yaml:"ffmpeg"`
	NumRetries     int          `yaml:"retries"`
	BufferSize     int          `yaml:"buffer-size"`
	UpdateMetadata bool         `yaml:"update-metadata"`
	UserAgent      string       `yaml:"user-agent"`
}

func (cfg *StreamConfig) Validate() error {
	errs := make([]error, 0)
	if cfg.Server.Host == "" || cfg.Server.Port == 0 {
		errs = append(errs, fmt.Errorf("host or port is required"))
	}
	if cfg.Format != "mpeg" && cfg.Format != "aac" {
		errs = append(errs, fmt.Errorf("unsupported format: %s", cfg.Format))
	}
	return errors.Join(errs...)
}

type ServerConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	TLS       bool   `yaml:"tls,omitempty"` // TODO
	Shoutcast bool   `yaml:"shoutcast,omitempty"`
	Password  string `yaml:"password,omitempty"`
}

func (cfg *ServerConfig) ServerType() string {
	if cfg.Shoutcast {
		return "shoutcast"
	}
	return "icecast"
}

type FfmpegConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Path     string         `yaml:"path"`
	ReEncode ReEncodeConfig `yaml:"re-encode"`
}

type ReEncodeConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Bitrate    int    `yaml:"bitrate"`
	Channels   int    `yaml:"channels"`
	SampleRate int    `yaml:"sample-rate"`
	AACProfile string `yaml:"aac-profile"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
	PID   string `yaml:"pid"` // TODO
}
type PlaylistConfig struct {
	Path    string `yaml:"path"`
	Shuffle bool   `yaml:"shuffle"`
	Repeat  int    `yaml:"repeat"`
}
