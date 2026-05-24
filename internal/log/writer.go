package log

import (
	"errors"
	"io"
	"time"
)

const (
	maxLogQLen           = 128
	queueShutdownTimeout = 500 * time.Millisecond
)

// LogWriter is a non-blocking writer that drops on overflow rather than
// stalling the caller. The underlying writer is drained by a single goroutine.
type LogWriter struct {
	writer io.Writer
	ch     chan []byte
	done   chan struct{}
}

func NewLogWriter(writer io.Writer) *LogWriter {
	lw := &LogWriter{
		writer: writer,
		ch:     make(chan []byte, maxLogQLen),
		done:   make(chan struct{}),
	}
	go lw.loop()
	return lw
}

func (lw *LogWriter) Write(p []byte) (int, error) {
	if p == nil {
		return 0, errors.New("Can't write nil byte slice")
	}
	buf := make([]byte, len(p))
	copy(buf, p)
	select {
	case lw.ch <- buf:
		return len(p), nil
	default:
		return 0, errors.New("Writer queue overflow")
	}
}

func (lw *LogWriter) loop() {
	for p := range lw.ch {
		if p == nil {
			break
		}
		lw.writer.Write(p)
	}
	lw.done <- struct{}{}
}

func (lw *LogWriter) Close() {
	lw.ch <- nil
	timer := time.After(queueShutdownTimeout)
	select {
	case <-timer:
	case <-lw.done:
	}
}
