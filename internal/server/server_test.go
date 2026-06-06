package server

// Integration tests against a fake Runner. The Runner interface is the
// process boundary, so an in-process fake gives deterministic control over
// timing, hangs and exit codes — exactly the concurrency-sensitive behaviors
// that are impractical to verify manually: the single-flight lock, SSE
// replay/mid-join, the exit → info-updated event order, and interrupt
// escalation.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gipcompany/spr-web/internal/sprbridge"
)

// --- fake process / runner ---

type fakeProcess struct {
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
	stderrR *io.PipeReader
	stderrW *io.PipeWriter
	exitCh  chan sprbridge.ExitStatus

	mu          sync.Mutex
	interrupted bool
	killed      bool

	// onInterrupt simulates the child's SIGINT response (nil = ignore it,
	// like a child stalled on a network call).
	onInterrupt func(p *fakeProcess)
}

func newFakeProcess() *fakeProcess {
	p := &fakeProcess{exitCh: make(chan sprbridge.ExitStatus, 1)}
	p.stdoutR, p.stdoutW = io.Pipe()
	p.stderrR, p.stderrW = io.Pipe()
	return p
}

func (p *fakeProcess) Stdout() io.Reader { return p.stdoutR }
func (p *fakeProcess) Stderr() io.Reader { return p.stderrR }

func (p *fakeProcess) Wait() sprbridge.ExitStatus { return <-p.exitCh }

func (p *fakeProcess) Interrupt() error {
	p.mu.Lock()
	p.interrupted = true
	fn := p.onInterrupt
	p.mu.Unlock()
	if fn != nil {
		fn(p)
	}
	return nil
}

func (p *fakeProcess) Kill() error {
	p.mu.Lock()
	p.killed = true
	p.mu.Unlock()
	p.finish(sprbridge.ExitStatus{Code: -1, Signal: "killed"})
	return nil
}

func (p *fakeProcess) wasInterrupted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.interrupted
}

func (p *fakeProcess) wasKilled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.killed
}

// finish closes the output streams and lets Wait return.
func (p *fakeProcess) finish(status sprbridge.ExitStatus) {
	_ = p.stdoutW.Close()
	_ = p.stderrW.Close()
	select {
	case p.exitCh <- status:
	default: // already finished
	}
}

type fakeRunner struct {
	mu sync.Mutex
	// commandProc is handed out for non-info commands, one at a time.
	pending []*fakeProcess
	// infoPayload is returned by `info` children (the post-run refresh).
	infoPayload string
	infoCalls   int
}

func (r *fakeRunner) Start(args []string) (sprbridge.Process, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(args) > 0 && args[0] == "info" {
		r.infoCalls++
		p := newFakeProcess()
		go func() {
			_, _ = io.WriteString(p.stdoutW, r.infoPayload)
			p.finish(sprbridge.ExitStatus{Code: 0})
		}()
		return p, nil
	}
	if len(r.pending) == 0 {
		return nil, fmt.Errorf("fakeRunner: unexpected Start(%v)", args)
	}
	p := r.pending[0]
	r.pending = r.pending[1:]
	return p, nil
}

func (r *fakeRunner) countInfoCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.infoCalls
}

func newTestServer(t *testing.T, runner *fakeRunner) (*Server, *httptest.Server) {
	t.Helper()
	if runner.infoPayload == "" {
		runner.infoPayload = `{"repo":{},"pullRequests":[],"commits":[]}`
	}
	dir := t.TempDir()
	s := newServer(dir, dir+"/.git", runner, Options{})
	s.interruptGrace = 50 * time.Millisecond
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

func post(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	res, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func runState(t *testing.T, baseURL string) map[string]any {
	t.Helper()
	res, err := http.Get(baseURL + "/api/runs/current")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

// --- tests ---

func TestSingleFlightLock(t *testing.T) {
	proc := newFakeProcess()
	runner := &fakeRunner{pending: []*fakeProcess{proc}}
	_, ts := newTestServer(t, runner)

	if res := post(t, ts.URL+"/api/commands/sync", ""); res.StatusCode != http.StatusAccepted {
		t.Fatalf("first POST = %d, want 202", res.StatusCode)
	}
	// While the child runs, every other command and the manual refresh are
	// rejected with 409.
	if res := post(t, ts.URL+"/api/commands/update", ""); res.StatusCode != http.StatusConflict {
		t.Fatalf("second POST = %d, want 409", res.StatusCode)
	}
	if res := post(t, ts.URL+"/api/info/refresh", ""); res.StatusCode != http.StatusConflict {
		t.Fatalf("refresh during run = %d, want 409", res.StatusCode)
	}

	proc.finish(sprbridge.ExitStatus{Code: 0})

	// The slot frees only after the post-run info refresh completes.
	runner.mu.Lock()
	runner.pending = append(runner.pending, newFakeProcess())
	runner.mu.Unlock()
	waitFor(t, "lock release", func() bool {
		res := post(t, ts.URL+"/api/commands/sync", "")
		return res.StatusCode == http.StatusAccepted
	})
}

func TestSSEReplayAndMidJoin(t *testing.T) {
	proc := newFakeProcess()
	runner := &fakeRunner{pending: []*fakeProcess{proc}}
	s, ts := newTestServer(t, runner)

	if res := post(t, ts.URL+"/api/commands/sync", ""); res.StatusCode != http.StatusAccepted {
		t.Fatalf("POST = %d, want 202", res.StatusCode)
	}

	// Emit two lines before anyone subscribes.
	_, _ = io.WriteString(proc.stdoutW, "line1\nline2\n")
	waitFor(t, "lines buffered", func() bool {
		run := s.currentRun()
		run.mu.Lock()
		defer run.mu.Unlock()
		return len(run.events) >= 2
	})

	// A subscriber joining mid-run replays the buffer from the start.
	res, err := http.Get(ts.URL + "/api/runs/current/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	events := make(chan string, 64)
	go func() {
		defer close(events)
		readSSE(res.Body, events)
	}()

	expectEvent(t, events, "log", `"line1"`)
	expectEvent(t, events, "log", `"line2"`)

	// New output is tailed live to the same subscriber.
	_, _ = io.WriteString(proc.stdoutW, "line3\n")
	expectEvent(t, events, "log", `"line3"`)

	proc.finish(sprbridge.ExitStatus{Code: 0})
	expectEvent(t, events, "exit", `"code":0`)
}

func TestExitThenInfoUpdatedOrder(t *testing.T) {
	proc := newFakeProcess()
	runner := &fakeRunner{pending: []*fakeProcess{proc}}
	_, ts := newTestServer(t, runner)

	if res := post(t, ts.URL+"/api/commands/update", ""); res.StatusCode != http.StatusAccepted {
		t.Fatalf("POST = %d, want 202", res.StatusCode)
	}

	res, err := http.Get(ts.URL + "/api/runs/current/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	_, _ = io.WriteString(proc.stdoutW, "done\n")
	proc.finish(sprbridge.ExitStatus{Code: 0})

	// Collect the full stream until the server closes it and verify the
	// exact tail order: ... log, exit, info-updated, then EOF.
	var names []string
	events := make(chan string, 64)
	go func() {
		defer close(events)
		readSSE(res.Body, events)
	}()
	for ev := range events {
		name, _, _ := strings.Cut(ev, "\x00")
		names = append(names, name)
	}
	if len(names) < 3 {
		t.Fatalf("stream too short: %v", names)
	}
	tail := names[len(names)-2:]
	if tail[0] != "exit" || tail[1] != "info-updated" {
		t.Errorf("event tail = %v, want [exit info-updated]", tail)
	}
	if runner.countInfoCalls() != 1 {
		t.Errorf("info refresh calls = %d, want 1", runner.countInfoCalls())
	}
}

func TestInterruptEscalation(t *testing.T) {
	// The child ignores SIGINT (onInterrupt does nothing), simulating a
	// stalled GitHub API call; the server must escalate to SIGKILL after the
	// grace period.
	proc := newFakeProcess()
	runner := &fakeRunner{pending: []*fakeProcess{proc}}
	_, ts := newTestServer(t, runner)

	if res := post(t, ts.URL+"/api/commands/merge", ""); res.StatusCode != http.StatusAccepted {
		t.Fatalf("POST = %d, want 202", res.StatusCode)
	}

	if res := post(t, ts.URL+"/api/runs/current/interrupt", ""); res.StatusCode != http.StatusAccepted {
		t.Fatalf("interrupt = %d, want 202", res.StatusCode)
	}
	waitFor(t, "SIGINT", proc.wasInterrupted)
	if proc.wasKilled() {
		t.Fatal("SIGKILL must not fire before the grace period")
	}
	waitFor(t, "SIGKILL escalation", proc.wasKilled)

	waitFor(t, "exited state", func() bool {
		return runState(t, ts.URL)["state"] == "exited"
	})
	state := runState(t, ts.URL)
	if state["exitCode"].(float64) != -1 || state["signal"] != "killed" {
		t.Errorf("run state = %v, want exitCode -1 signal killed", state)
	}
}

func TestInterruptHonoredWithinGrace(t *testing.T) {
	// A child that exits on SIGINT must never receive SIGKILL.
	proc := newFakeProcess()
	proc.onInterrupt = func(p *fakeProcess) {
		p.finish(sprbridge.ExitStatus{Code: -1, Signal: "interrupt"})
	}
	runner := &fakeRunner{pending: []*fakeProcess{proc}}
	_, ts := newTestServer(t, runner)

	post(t, ts.URL+"/api/commands/merge", "")
	post(t, ts.URL+"/api/runs/current/interrupt", "")

	waitFor(t, "exited state", func() bool {
		return runState(t, ts.URL)["state"] == "exited"
	})
	time.Sleep(100 * time.Millisecond) // past the 50ms grace period
	if proc.wasKilled() {
		t.Error("SIGKILL fired although the child exited within the grace period")
	}
}

func TestBuildChildArgs(t *testing.T) {
	intp := func(v int) *int { return &v }
	tests := []struct {
		name    string
		cmd     string
		req     CommandRequest
		want    []string
		wantErr bool
	}{
		{"plain update", "update", CommandRequest{}, []string{"update"}, false},
		{
			"update with everything",
			"update",
			CommandRequest{Count: intp(3), Reviewers: []string{"alice", " bob "}, NoRebase: true, NoFetch: true, Verbose: true},
			[]string{"update", "--count", "3", "--reviewer", "alice", "--reviewer", "bob", "--no-rebase", "--no-fetch", "--verbose"},
			false,
		},
		{"merge with count", "merge", CommandRequest{Count: intp(2)}, []string{"merge", "--count", "2"}, false},
		{"merge bad count", "merge", CommandRequest{Count: intp(0)}, nil, true},
		{"amend", "amend", CommandRequest{Commit: "abc123"}, []string{"amend", "--commit", "abc123"}, false},
		{"amend without commit", "amend", CommandRequest{}, nil, true},
		{"edit", "edit", CommandRequest{Commit: "abc123", Verbose: true}, []string{"edit", "--commit", "abc123", "--verbose"}, false},
		{"edit-done", "edit-done", CommandRequest{}, []string{"edit-done"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildChildArgs(tt.cmd, tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("args = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- SSE reading helpers ---

// readSSE parses "event:/data:" frames and emits "name\x00data" strings.
func readSSE(r io.Reader, out chan<- string) {
	sc := bufio.NewScanner(r)
	var name, data string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "" && name != "":
			out <- name + "\x00" + data
			name, data = "", ""
		}
	}
}

func expectEvent(t *testing.T, events <-chan string, wantName, wantDataSubstr string) {
	t.Helper()
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatalf("stream closed while waiting for %s", wantName)
		}
		name, data, _ := strings.Cut(ev, "\x00")
		if name != wantName || !bytes.Contains([]byte(data), []byte(wantDataSubstr)) {
			t.Fatalf("event = %s %s, want %s containing %s", name, data, wantName, wantDataSubstr)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for event %s", wantName)
	}
}
