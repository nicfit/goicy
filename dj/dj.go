package dj

import (
	"fmt"
	"slices"
)

type Dj struct {
	Name string
}

type Queue []Dj

type Booth interface {
	Join(dj Dj)
	Leave(dj Dj)
	Size() int
	Cycle() (Dj, error)
}

func NewBooth() Booth {
	return &booth{
		queue: make(Queue, 0),
	}
}

type booth struct {
	queue Queue
}

func (b *booth) Size() int {
	return len(b.queue)
}

func (b *booth) Join(dj Dj) {
	if !slices.Contains(b.queue, dj) {
		b.queue = append(b.queue, dj)
	}
}

func (b *booth) Leave(dj Dj) {
	if idx := slices.Index(b.queue, dj); idx >= 0 {
		b.queue = slices.Delete(b.queue, idx, idx+1)
	}
}

func (b *booth) Cycle() (Dj, error) {
	if len(b.queue) == 0 {
		return Dj{}, fmt.Errorf("Booth queue is empty")
	}
	dj := b.queue[0]
	b.queue = b.queue[1:]
	b.queue = append(b.queue, dj)
	return dj, nil
}
