// Package server implements the spr-web HTTP API and static file serving.
//
// Design invariants:
//   - Single-flight lock per repository: at most one child process (command
//     or info fetch) runs at a time; concurrent requests get 409.
//   - Runs are decoupled from connections: a child process always runs to
//     completion regardless of SSE subscribers, and its log is buffered in
//     memory (latest run only) for replay.
//   - The server itself never executes spr code in-process; everything goes
//     through the Runner (self re-execution), so spr's os.Exit / log.Fatal
//     error model cannot crash the server.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gipcompany/spr-web/internal/sprbridge"
)

const (
	defaultPort  = 7780
	maxPortScan  = 20
	listenAddr   = "127.0.0.1"
	interruptTTL = 5 * time.Second
)

// VersionInfo is rendered in the UI footer.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Spr     string `json:"spr"`
}

// Options configures New.
type Options struct {
	// Port is the explicit port to listen on. 0 means: start at 7780 and
	// scan upward for a free port (instance-per-repo support).
	Port    int
	NoOpen  bool
	Static  fs.FS
	Version VersionInfo
}

// Server holds all state for one repository instance.
type Server struct {
	repoRoot string
	gitDir   string
	runner   sprbridge.Runner
	static   fs.FS
	version  VersionInfo

	// interruptGrace is the SIGINT -> SIGKILL escalation delay
	// (shortened in tests).
	interruptGrace time.Duration

	mu        sync.Mutex
	busy      bool // single-flight: a child process is running
	run       *Run // latest command run (nil until the first command)
	info      json.RawMessage
	fetchedAt time.Time

	opts Options
}

// New builds a Server for the repository containing the current working
// directory.
func New(opts Options) (*Server, error) {
	root, err := gitOutput(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, errors.New("not a git repository (spr-web must be started inside a repository configured for spr)")
	}
	root = strings.TrimSpace(root)
	gitDir, err := gitOutput(root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, fmt.Errorf("resolving git dir: %w", err)
	}
	return newServer(root, strings.TrimSpace(gitDir), &sprbridge.SelfRunner{Dir: root}, opts), nil
}

func newServer(repoRoot, gitDir string, runner sprbridge.Runner, opts Options) *Server {
	return &Server{
		repoRoot:       repoRoot,
		gitDir:         gitDir,
		runner:         runner,
		static:         opts.Static,
		version:        opts.Version,
		interruptGrace: interruptTTL,
		opts:           opts,
	}
}

// Run performs the fail-fast startup check, binds the listener, opens the
// browser and serves until interrupted.
func (s *Server) Run() error {
	// Fail fast: one end-to-end `_spr info` validates git repo, spr config,
	// GitHub token and API connectivity in a single mechanism — the same
	// thing `git spr` itself would do. The result seeds the info cache.
	fmt.Println("spr-web: checking spr configuration and GitHub connectivity...")
	info, err := sprbridge.RunInfo(s.runner)
	if err != nil {
		return err
	}
	s.setInfo(info)

	ln, err := s.listen()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s", ln.Addr().String())
	fmt.Printf("spr-web: serving %s at %s\n", s.repoRoot, url)

	httpServer := &http.Server{Handler: s.Handler()}
	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	if !s.opts.NoOpen {
		openBrowser(url)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		s.killCurrentRun()
		return err
	case <-sigCh:
		fmt.Println("\nspr-web: shutting down")
		// Kill any in-flight child (whole process group) so no orphaned
		// git/spr processes keep mutating the repository.
		s.killCurrentRun()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
		return nil
	}
}

func (s *Server) listen() (net.Listener, error) {
	if s.opts.Port != 0 {
		// Explicit port: never fall back — listening elsewhere than the user
		// asked for would be worse than failing.
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenAddr, s.opts.Port))
		if err != nil {
			return nil, fmt.Errorf("cannot listen on port %d: %w", s.opts.Port, err)
		}
		return ln, nil
	}
	for port := defaultPort; port < defaultPort+maxPortScan; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenAddr, port))
		if err == nil {
			return ln, nil
		}
	}
	return nil, fmt.Errorf("no free port in range %d-%d", defaultPort, defaultPort+maxPortScan-1)
}

func (s *Server) killCurrentRun() {
	s.mu.Lock()
	run := s.run
	s.mu.Unlock()
	if run == nil {
		return
	}
	run.mu.Lock()
	proc := run.proc
	exited := run.exitCode != nil
	run.mu.Unlock()
	if proc != nil && !exited {
		_ = proc.Kill()
	}
}

// Handler builds the HTTP routing table.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/info", s.handleGetInfo)
	mux.HandleFunc("POST /api/info/refresh", s.handleRefreshInfo)
	mux.HandleFunc("GET /api/state", s.handleGetState)
	mux.HandleFunc("POST /api/commands/{cmd}", s.handleCommand)
	mux.HandleFunc("GET /api/runs/current", s.handleGetRun)
	mux.HandleFunc("GET /api/runs/current/events", s.handleRunEvents)
	mux.HandleFunc("POST /api/runs/current/interrupt", s.handleInterrupt)
	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.Handle("/", s.staticHandler())
	return mux
}

// --- info cache ---

func (s *Server) setInfo(info json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.info = info
	s.fetchedAt = time.Now()
}

// acquire takes the single-flight slot. It returns false when another child
// process is already running.
func (s *Server) acquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return false
	}
	s.busy = true
	return true
}

func (s *Server) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busy = false
}

// refreshInfoHoldingSlot runs `_spr info` and updates the cache. The caller
// must hold the single-flight slot.
func (s *Server) refreshInfoHoldingSlot() error {
	info, err := sprbridge.RunInfo(s.runner)
	if err != nil {
		return err
	}
	s.setInfo(info)
	return nil
}

// --- handlers ---

func (s *Server) handleGetInfo(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	info := s.info
	fetchedAt := s.fetchedAt
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"info":      info,
		"fetchedAt": fetchedAt.Format(time.RFC3339),
	})
}

func (s *Server) handleRefreshInfo(w http.ResponseWriter, _ *http.Request) {
	state := detectRepoState(s.repoRoot, s.gitDir)
	if state.editing() || state.RebaseInProgress {
		// Reading the stack mid-rebase yields bogus (or panicking) results.
		writeError(w, http.StatusConflict, "cannot refresh while a rebase or edit session is in progress")
		return
	}
	if !s.acquire() {
		writeError(w, http.StatusConflict, "a command is already running")
		return
	}
	defer s.release()
	if err := s.refreshInfoHoldingSlot(); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.handleGetInfo(w, nil)
}

func (s *Server) handleGetState(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, detectRepoState(s.repoRoot, s.gitDir))
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.version)
}

// CommandRequest is the JSON body of POST /api/commands/{cmd}. All fields
// are optional; which ones apply depends on the command.
type CommandRequest struct {
	Commit    string   `json:"commit"`
	Count     *int     `json:"count"`
	Reviewers []string `json:"reviewers"`
	NoRebase  bool     `json:"noRebase"`
	NoFetch   bool     `json:"noFetch"`
	Verbose   bool     `json:"verbose"`
}

var knownCommands = map[string]bool{
	"update": true, "sync": true, "check": true, "merge": true,
	"amend": true, "edit": true, "edit-done": true, "edit-abort": true,
}

// buildChildArgs translates an API command request into `_spr` child argv.
func buildChildArgs(cmd string, req CommandRequest) ([]string, error) {
	args := []string{cmd}
	switch cmd {
	case "update":
		if req.Count != nil {
			if *req.Count < 1 {
				return nil, errors.New("count must be >= 1")
			}
			args = append(args, "--count", fmt.Sprint(*req.Count))
		}
		for _, r := range req.Reviewers {
			if r = strings.TrimSpace(r); r != "" {
				args = append(args, "--reviewer", r)
			}
		}
		if req.NoRebase {
			args = append(args, "--no-rebase")
		}
		if req.NoFetch {
			args = append(args, "--no-fetch")
		}
	case "merge":
		if req.Count != nil {
			if *req.Count < 1 {
				return nil, errors.New("count must be >= 1")
			}
			args = append(args, "--count", fmt.Sprint(*req.Count))
		}
	case "amend", "edit":
		if req.Commit == "" {
			return nil, errors.New("commit is required")
		}
		args = append(args, "--commit", req.Commit)
	case "sync", "check", "edit-done", "edit-abort":
		// no command-specific options
	}
	if req.Verbose {
		args = append(args, "--verbose")
	}
	return args, nil
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	cmd := r.PathValue("cmd")
	if !knownCommands[cmd] {
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown command %q", cmd))
		return
	}

	var req CommandRequest
	if r.Body != nil {
		// An empty body is fine; a malformed one is not.
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	args, err := buildChildArgs(cmd, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// State guards. The edit session is a cross-process state machine owned
	// by spr (file based), so it survives server restarts and is re-checked
	// on every command.
	state := detectRepoState(s.repoRoot, s.gitDir)
	isEditCmd := cmd == "edit-done" || cmd == "edit-abort"
	switch {
	case state.editing() && !isEditCmd:
		writeError(w, http.StatusConflict, "an edit session is in progress; finish it (done) or abort it first")
		return
	case !state.editing() && isEditCmd:
		writeError(w, http.StatusConflict, "no edit session is in progress")
		return
	case !state.editing() && state.RebaseInProgress:
		writeError(w, http.StatusConflict, "a git rebase is in progress outside spr-web; resolve it in a terminal (e.g. git rebase --abort)")
		return
	}
	if cmd == "amend" && state.WorkingTree.Staged == 0 {
		writeError(w, http.StatusUnprocessableEntity, "no staged changes to amend")
		return
	}

	if !s.acquire() {
		writeError(w, http.StatusConflict, "a command is already running")
		return
	}

	run := newRun(cmd, args)
	s.mu.Lock()
	s.run = run
	s.mu.Unlock()

	go s.runLifecycle(run)
	w.WriteHeader(http.StatusAccepted)
}

// runLifecycle drives a command run from spawn to stream close. It owns the
// single-flight slot taken by handleCommand and releases it at the end —
// after the post-run info refresh, so the refresh can never race a new
// command.
func (s *Server) runLifecycle(run *Run) {
	defer func() {
		run.close()
		s.release()
	}()

	proc, err := s.runner.Start(run.args)
	if err != nil {
		run.appendLog("server", "failed to start command: "+err.Error())
		run.markExit(-1, "")
		return
	}
	run.setProc(proc)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		pumpLines(proc.Stdout(), "stdout", run)
	}()
	go func() {
		defer wg.Done()
		pumpLines(proc.Stderr(), "stderr", run)
	}()
	wg.Wait()

	status := proc.Wait()
	run.markExit(status.Code, status.Signal)

	// Refresh the cached info so the UI reflects the new stack — unless the
	// repository is mid-rebase (edit session just started, or an interrupted
	// run left a rebase behind): reading the stack would be unreliable.
	state := detectRepoState(s.repoRoot, s.gitDir)
	if state.editing() || state.RebaseInProgress {
		return
	}
	if err := s.refreshInfoHoldingSlot(); err != nil {
		run.appendLog("server", "info refresh after run failed: "+err.Error())
		return
	}
	run.append(infoUpdatedEvent())
}

func (s *Server) currentRun() *Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.run
}

func (s *Server) handleGetRun(w http.ResponseWriter, _ *http.Request) {
	run := s.currentRun()
	if run == nil {
		writeJSON(w, http.StatusOK, map[string]any{"state": "idle"})
		return
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	resp := map[string]any{
		"cmd":       run.cmd,
		"startedAt": run.startedAt.Format(time.RFC3339),
	}
	if run.exitCode != nil {
		resp["state"] = "exited"
		resp["exitCode"] = *run.exitCode
		if run.exitSignal != "" {
			resp["signal"] = run.exitSignal
		}
	} else {
		resp["state"] = "running"
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRunEvents(w http.ResponseWriter, r *http.Request) {
	run := s.currentRun()
	if run == nil {
		writeError(w, http.StatusNotFound, "no run")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	replay, ch, cancel := run.subscribe()
	defer cancel()

	for _, ev := range replay {
		writeSSE(w, ev)
	}
	flusher.Flush()

	for {
		select {
		case ev, open := <-ch:
			if !open {
				return // run ended (post-run refresh included): close the stream
			}
			writeSSE(w, ev)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleInterrupt(w http.ResponseWriter, _ *http.Request) {
	run := s.currentRun()
	if run == nil {
		writeError(w, http.StatusNotFound, "no run")
		return
	}
	run.mu.Lock()
	proc := run.proc
	exited := run.exitCode != nil
	run.mu.Unlock()
	if proc == nil || exited {
		writeError(w, http.StatusConflict, "no running command to interrupt")
		return
	}

	run.interruptOnce.Do(func() {
		// SIGINT first: git installs signal handlers that clean up its lock
		// files. Escalate to SIGKILL only if the child does not exit within
		// the grace period (e.g. a stalled GitHub API call).
		run.appendLog("server", "interrupt requested: sending SIGINT")
		_ = proc.Interrupt()
		grace := s.interruptGrace
		go func() {
			time.Sleep(grace)
			if !run.exited() {
				run.appendLog("server", "still running after grace period: sending SIGKILL")
				_ = proc.Kill()
			}
		}()
	})
	w.WriteHeader(http.StatusAccepted)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeSSE(w http.ResponseWriter, ev Event) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Name, ev.Data)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}
