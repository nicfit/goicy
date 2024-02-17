package dj

type Dj struct {
	Name string
}

type Queue []Dj

var queue = make(Queue, 0)
