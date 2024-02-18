package playlist

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Playlist interface {
	Len() int
	Next() (string, error)
}

type Options struct {
	RepeatList    bool
	RepeatCurrent bool
}

type playlist struct {
	items []string
	iter  int
	opts  *Options
}

func New(filename string, opts *Options) (Playlist, error) {
	if opts == nil {
		opts = &Options{}
	}
	var p = &playlist{
		items: make([]string, 0),
		iter:  0,
		opts:  opts,
	}

	if err := loadFromFile(p, filename); err != nil {
		return nil, fmt.Errorf("error loading playlist file: %w", err)
	}

	return p, nil
}

func loadFromFile(p *playlist, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.Trim(scanner.Text(), " \n\r\t")
		if text != "" {
			p.items = append(p.items, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		p.items = nil
		return err
	}
	return nil
}

func (p *playlist) Len() int {
	return len(p.items)
}

func (p *playlist) Next() (string, error) {
	if len(p.items) == 0 {
		return "", fmt.Errorf("empty")
	}
	if p.iter >= len(p.items) {
		if !p.opts.RepeatList {
			return "", fmt.Errorf("eol")
		}
		p.iter = 0
	}

	// FIXME: needs a rethink on iter/next
	item := p.items[p.iter]
	if !p.opts.RepeatCurrent {
		p.iter++
	}
	return item, nil
}
