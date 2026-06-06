package server

import (
	"bufio"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/gipcompany/spr-web/internal/sprbridge"
)

// maxBufferedEvents bounds the in-memory log buffer of the current run. When
// exceeded, the oldest log events are dropped and replays are prefixed with a
// truncation notice.
const maxBufferedEvents = 20000

// subscriberBuffer is the per-subscriber channel capacity. A subscriber that
// cannot keep up is dropped (its stream closes); the client simply
// reconnects and receives a full replay.
const subscriberBuffer = 8192

// Event is a single SSE event of the run stream.
type Event struct {
	Name string
	Data []byte // pre-marshaled JSON
}

func logEvent(stream, text string) Event {
	data, _ := json.Marshal(map[string]string{"stream": stream, "text": text})
	return Event{Name: "log", Data: data}
}

func exitEvent(code int, signal string) Event {
	payload := map[string]any{"code": code}
	if signal != "" {
		payload["signal"] = signal
	}
	data, _ := json.Marshal(payload)
	return Event{Name: "exit", Data: data}
}

func infoUpdatedEvent() Event {
	return Event{Name: "info-updated", Data: []byte("{}")}
}

// Run is the single in-flight (or most recently finished) command execution.
// Logs are buffered in memory so the run survives page reloads: a client can
// attach at any time and replay the stream from the start.
type Run struct {
	cmd       string
	args      []string
	startedAt time.Time

	mu         sync.Mutex
	proc       sprbridge.Process
	events     []Event
	truncated  bool
	subs       map[chan Event]struct{}
	exitCode   *int
	exitSignal string
	closed     bool

	interruptOnce sync.Once
}

func newRun(cmd string, args []string) *Run {
	return &Run{
		cmd:       cmd,
		args:      args,
		startedAt: time.Now(),
		subs:      make(map[chan Event]struct{}),
	}
}

func (r *Run) setProc(p sprbridge.Process) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.proc = p
}

func (r *Run) append(ev Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.events = append(r.events, ev)
	if len(r.events) > maxBufferedEvents {
		r.events = r.events[len(r.events)-maxBufferedEvents:]
		r.truncated = true
	}
	for ch := range r.subs {
		select {
		case ch <- ev:
		default:
			// Slow subscriber: drop it; the client reconnects and replays.
			delete(r.subs, ch)
			close(ch)
		}
	}
}

func (r *Run) appendLog(stream, text string) {
	r.append(logEvent(stream, text))
}

func (r *Run) markExit(code int, signal string) {
	r.mu.Lock()
	r.exitCode = &code
	r.exitSignal = signal
	r.mu.Unlock()
	r.append(exitEvent(code, signal))
}

func (r *Run) exited() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.exitCode != nil
}

// close ends the stream: all subscriber channels are closed and no further
// events are accepted.
func (r *Run) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	for ch := range r.subs {
		close(ch)
	}
	r.subs = nil
}

// subscribe returns a snapshot of buffered events for replay plus a live
// channel for subsequent events. The channel is closed when the run ends (or
// immediately when it already has). cancel must be called when the consumer
// goes away.
func (r *Run) subscribe() (replay []Event, ch chan Event, cancel func()) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.truncated {
		replay = append(replay, logEvent("server", "(log truncated: output exceeded the buffer limit)"))
	}
	replay = append(replay, r.events...)

	ch = make(chan Event, subscriberBuffer)
	if r.closed {
		close(ch)
		return replay, ch, func() {}
	}
	r.subs[ch] = struct{}{}
	cancel = func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, ok := r.subs[ch]; ok {
			delete(r.subs, ch)
			close(ch)
		}
	}
	return replay, ch, cancel
}

// pumpLines reads a child output stream line by line into the run buffer.
// The final unterminated line (e.g. an interactive prompt) is emitted on EOF.
func pumpLines(r io.Reader, stream string, run *Run) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		run.appendLog(stream, sc.Text())
	}
}
