package network

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/nicfit/goicy/config"
	"github.com/nicfit/goicy/logger"
)

type Network interface {
	Send(buf []byte) error
	Recv() ([]byte, error)
	Close() error
}

type network struct {
	net.Conn
	connected    bool
	streamConfig *config.StreamConfig
}

func Connect(host string, port int) (Network, error) {
	h := host + ":" + strconv.Itoa(port)
	sock, err := net.Dial("tcp", h)
	if err != nil {
		return nil, err
	}
	return &network{sock, true, &config.StreamConfig{}}, nil
}

func (s *network) Send(buf []byte) error {
	n, err := s.Write(buf)
	if err != nil {
		s.connected = false
		return err
	}
	if n != len(buf) {
		s.connected = false
		return fmt.Errorf("error sending all data: %d < %d", n, len(buf))
	}
	return nil
}

func (s *network) Recv() ([]byte, error) {
	var buf = make([]byte, 1024)

	n, err := s.Read(buf)
	//fmt.Println(n, err, string(buf), len(buf))
	if err != nil {
		logger.Log(err.Error(), slog.LevelError)
		return nil, err
	}
	return buf[0:n], err
}

func (s *network) Close() error {
	s.connected = false
	return s.Conn.Close()
}

func ConnectServer(stream *config.StreamConfig, bitrate int, samplerate, channels int) (Network, error) {
	if stream.Server.Shoutcast {
		stream.Server.Port++
	}
	logger.Log("Connecting to "+stream.Server.ServerType()+" at "+stream.Server.Host+":"+strconv.Itoa(stream.Server.Port)+"...", slog.LevelDebug)
	sock, err := Connect(stream.Server.Host, stream.Server.Port)
	if err != nil {
		return nil, err
	}
	sock.(*network).streamConfig = stream

	time.Sleep(time.Second)

	headers := ""
	contenttype := ""
	if stream.Format == "mpeg" {
		contenttype = "audio/mpeg"
	} else {
		contenttype = "audio/aacp"
	}

	if stream.Server.Shoutcast {
		if err := sock.Send([]byte(stream.Server.Password + "\r\n")); err != nil {
			logger.Log("Error sending password", slog.LevelError)
			sock.(*network).connected = false
			return sock, err
		}

		time.Sleep(time.Second)

		resp, err := sock.Recv()
		if err != nil {
			logger.Log("Error receiving ShoutCast response", slog.LevelError)
			sock.(*network).connected = false
			return sock, err
		}
		//fmt.Println(string(resp[0:3]))
		if string(resp[0:3]) != "OK2" {
			logger.Log("Shoutcast password rejected: "+string(resp), slog.LevelError)
			sock.(*network).connected = false
			return sock, err
		}
		//fmt.Println("password accepted")
		headers = "content-type:" + contenttype + "\r\n" +
			"icy-name:" + stream.Name + "\r\n" +
			"icy-genre:" + stream.Genre + "\r\n" +
			"icy-url:" + stream.URL + "\r\n" +
			"icy-pub:0\r\n" +
			fmt.Sprintf("icy-br:%d\r\n\r\n", bitrate)
	} else {
		user_agent := stream.UserAgent
		if user_agent == "" {
			user_agent = "goicy/" + config.Version
		}
		headers = "SOURCE /" + stream.Mount + " HTTP/1.0\r\n" +
			"Content-Type: " + contenttype + "\r\n" +
			"Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("source:"+stream.Server.Password)) + "\r\n" +
			"User-Agent: " + user_agent + "\r\n" +
			"ice-name: " + stream.Name + "\r\n" +
			"ice-public: 0\r\n" +
			"ice-url: " + stream.URL + "\r\n" +
			"ice-genre: " + stream.Genre + "\r\n" +
			"ice-description: " + stream.Desc + "\r\n" +
			"ice-audio-info: bitrate=" + strconv.Itoa(bitrate) +
			";channels=" + strconv.Itoa(channels) +
			";samplerate=" + strconv.Itoa(samplerate) + "\r\n" +
			"\r\n"
	}

	if err := sock.Send([]byte(headers)); err != nil {
		logger.Log("Error sending headers", slog.LevelError)
		sock.(*network).connected = false
		return sock, err
	}

	if !stream.Server.Shoutcast {
		time.Sleep(time.Second)
		resp, err := sock.Recv()
		if err != nil {
			sock.(*network).connected = false
			return sock, err
		}
		if string(resp[9:12]) != "200" {
			sock.(*network).connected = false
			return sock, errors.New("Invalid Icecast response: " + string(resp))
		}
	}

	logger.Log("Server connect successful", slog.LevelInfo)
	sock.(*network).connected = true
	return sock, nil
}

const (
	TYPE_ICECAST   = "icecast"
	TYPE_SHOUTCAST = "shoutcast"
)
