package supervisor

import (
	"bytes"
	"strings"
	"sync"

	"github.com/omar0ali/watchd/internal/api"
)

const (
	logCapacity = 2000      // lines kept per app
	maxLineLen  = 16 * 1024 // longer lines are split
	subBuffer   = 256       // slow subscribers drop lines past this
)

// LogBuffer keeps the most recent lines of an app and fans new lines out to
// subscribers (logs -f).
type LogBuffer struct {
	mu     sync.Mutex
	ring   []api.LogLine
	head   int // index of the oldest line
	size   int
	subs   map[chan api.LogLine]struct{}
	closed bool
}

func NewLogBuffer() *LogBuffer {
	return &LogBuffer{
		ring: make([]api.LogLine, logCapacity),
		subs: map[chan api.LogLine]struct{}{},
	}
}

func (b *LogBuffer) Add(l api.LogLine) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	idx := (b.head + b.size) % len(b.ring)
	b.ring[idx] = l
	if b.size < len(b.ring) {
		b.size++
	} else {
		b.head = (b.head + 1) % len(b.ring)
	}
	for ch := range b.subs {
		select {
		case ch <- l:
		default: // subscriber is too slow; drop rather than block the app
		}
	}
}

// TailAndSubscribe returns the last n lines (all if n <= 0) and, if
// subscribe is set, a channel of lines added afterwards with no gap or
// overlap. The channel is closed by cancel or Close.
func (b *LogBuffer) TailAndSubscribe(n int, subscribe bool) ([]api.LogLine, <-chan api.LogLine, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if n <= 0 || n > b.size {
		n = b.size
	}
	out := make([]api.LogLine, 0, n)
	for i := b.size - n; i < b.size; i++ {
		out = append(out, b.ring[(b.head+i)%len(b.ring)])
	}
	if !subscribe {
		return out, nil, func() {}
	}

	ch := make(chan api.LogLine, subBuffer)
	if b.closed {
		close(ch)
		return out, ch, func() {}
	}
	b.subs[ch] = struct{}{}
	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
	}
	return out, ch, cancel
}

// Close ends all subscriptions; later lines are dropped.
func (b *LogBuffer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.subs {
		delete(b.subs, ch)
		close(ch)
	}
}

// lineWriter splits process output into lines for a LogBuffer. exec calls
// Write from a single goroutine per stream, so it needs no locking.
type lineWriter struct {
	emit func(string)
	buf  []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.emit(strings.TrimRight(string(w.buf[:i]), "\r"))
		w.buf = w.buf[i+1:]
	}
	for len(w.buf) > maxLineLen {
		w.emit(string(w.buf[:maxLineLen]))
		w.buf = w.buf[maxLineLen:]
	}
	return len(p), nil
}

func (w *lineWriter) Flush() {
	if len(w.buf) > 0 {
		w.emit(string(w.buf))
		w.buf = nil
	}
}
