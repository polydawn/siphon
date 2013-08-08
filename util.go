package siphon

import (
	"io"
	"sync"
)

func NewWriteBroadcaster() *WriteBroadcaster {
	return &WriteBroadcaster{writers: make(map[io.WriteCloser]bool)}
}

type WriteBroadcaster struct {
	sync.Mutex
	writers map[io.WriteCloser]bool
}

func (w *WriteBroadcaster) AddWriter(writer io.WriteCloser) {
	w.Lock()
	defer w.Unlock()
	if w.writers != nil {
		w.writers[writer] = true
	} else {
		writer.Close()
	}
}

func (w *WriteBroadcaster) Write(p []byte) (n int, err error) {
	w.Lock()
	defer w.Unlock()
	for x := range w.writers {
		if n, err := x.Write(p); err != nil || n != len(p) {
			// On error, evict the writer
			delete(w.writers, x)
			x.Close()
		}
	}
	return len(p), nil
}

func (w *WriteBroadcaster) CloseWriters() {
	w.Lock()
	defer w.Unlock()
	for x := range w.writers {
		x.Close()
	}
	w.writers = nil
}


