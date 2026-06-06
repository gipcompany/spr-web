package sprbridge

// Parent-process side: spawning `spr-web _spr <cmd>` children and decoding
// their output. The server depends only on the Runner interface so tests can
// substitute a fake child process.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// ExitStatus describes how a child process ended.
type ExitStatus struct {
	// Code is the exit code, or -1 when the process was killed by a signal.
	Code int
	// Signal is the terminating signal name ("interrupt", "killed", ...) when
	// the process died from a signal, empty otherwise.
	Signal string
}

// Process is a running spr child process.
type Process interface {
	Stdout() io.Reader
	Stderr() io.Reader
	// Wait blocks until the process exits and returns its status. It must be
	// called exactly once.
	Wait() ExitStatus
	// Interrupt sends SIGINT to the child's process group.
	Interrupt() error
	// Kill sends SIGKILL to the child's process group.
	Kill() error
}

// Runner starts spr child processes.
type Runner interface {
	// Start spawns `_spr` with the given arguments (e.g. ["update", "--count", "2"]).
	Start(args []string) (Process, error)
}

// SelfRunner re-executes the current binary in child-process mode. Children
// are placed in their own process group so that interrupting or killing a
// run also reaches anything the child spawned (git, mergeCheck commands).
type SelfRunner struct {
	// Dir is the working directory for children (the repository root).
	Dir string
}

func (r *SelfRunner) Start(args []string) (Process, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, append([]string{"_spr"}, args...)...)
	cmd.Dir = r.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &osProcess{cmd: cmd, stdout: stdout, stderr: stderr}, nil
}

type osProcess struct {
	cmd    *exec.Cmd
	stdout io.Reader
	stderr io.Reader
}

func (p *osProcess) Stdout() io.Reader { return p.stdout }
func (p *osProcess) Stderr() io.Reader { return p.stderr }

func (p *osProcess) Wait() ExitStatus {
	err := p.cmd.Wait()
	if err == nil {
		return ExitStatus{Code: 0}
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		status := ExitStatus{Code: ee.ExitCode()}
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			status.Signal = ws.Signal().String()
		}
		return status
	}
	return ExitStatus{Code: -1, Signal: err.Error()}
}

func (p *osProcess) signalGroup(sig syscall.Signal) error {
	if p.cmd.Process == nil {
		return errors.New("process not started")
	}
	return syscall.Kill(-p.cmd.Process.Pid, sig)
}

func (p *osProcess) Interrupt() error { return p.signalGroup(syscall.SIGINT) }
func (p *osProcess) Kill() error      { return p.signalGroup(syscall.SIGKILL) }

// RunInfo runs `_spr info` to completion and returns the raw InfoPayload
// JSON. On failure the returned error carries the child's stderr so it can
// be surfaced as-is (fail-fast startup does exactly that).
func RunInfo(r Runner) (json.RawMessage, error) {
	proc, err := r.Start([]string{"info"})
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdout, proc.Stdout())
		close(done)
	}()
	_, _ = io.Copy(&stderr, proc.Stderr())
	<-done

	status := proc.Wait()
	if status.Code != 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("spr info failed with exit code %d", status.Code)
		}
		return nil, errors.New(msg)
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if !json.Valid(raw) {
		return nil, errors.New("spr info produced invalid JSON")
	}
	return json.RawMessage(raw), nil
}
