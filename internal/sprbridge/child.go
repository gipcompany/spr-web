package sprbridge

// This file implements the child-process side of spr-web: the hidden
// `spr-web _spr <cmd>` subcommands. spr's error model is os.Exit / log.Fatal
// based, so every spr invocation runs in its own process; a fatal error here
// can never take the server down.

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/config/config_parser"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/realgit"
	"github.com/ejoffe/spr/github/githubclient"
	"github.com/ejoffe/spr/spr"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encoding/json"
)

// childEnv holds the spr config plus the rescued stdout. spr logs git
// commands and GitHub calls with bare fmt.Printf (stdout), which would
// corrupt the `info` JSON output and mix verbose logs into command output.
// The wrapper therefore points os.Stdout at stderr during initialization and
// keeps the real stdout aside for intentional output only.
type childEnv struct {
	cfg        *config.Config
	gitcmd     git.GitInterface
	realStdout *os.File
}

func initChild(verbose bool) *childEnv {
	// zerolog defaults to trace-level global logging; quiet it down and keep
	// it on stderr so it never pollutes stdout.
	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, NoColor: true}).
		With().Timestamp().Logger()

	realStdout := os.Stdout
	os.Stdout = os.Stderr

	gitcmd := realgit.NewGitCmd(config.DefaultConfig())
	var status string
	if err := gitcmd.Git("status --porcelain", &status); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ExitConfig)
	}
	cfg := config_parser.ParseConfig(gitcmd)
	if err := config_parser.CheckConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ExitConfig)
	}
	// With --verbose, git/GitHub call logs flow to the current os.Stdout,
	// which is stderr here — exactly the "stderr carries verbose logs"
	// contract of the run stream.
	cfg.User.LogGitCommands = verbose
	cfg.User.LogGitHubCalls = verbose
	gitcmd = realgit.NewGitCmd(cfg)

	return &childEnv{cfg: cfg, gitcmd: gitcmd, realStdout: realStdout}
}

// injectStdin replaces os.Stdin with a pipe that yields the given line. spr
// reads the commit number for amend/edit from stdin; the hash sent by the UI
// is resolved to a number right here in the child, immediately before use,
// so the window for the stack to change underneath is minimal.
//
// This must run BEFORE spr.NewStackedPR, which captures os.Stdin at
// construction time.
func injectStdin(line string) {
	r, w, err := os.Pipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdin = r
	go func() {
		_, _ = io.WriteString(w, line+"\n")
		_ = w.Close()
	}()
}

// resolveOrExit maps a commit hash (prefix) to its 1-based stack number,
// exiting with the ExitCommitNotFound contract code when the commit is gone.
func resolveOrExit(env *childEnv, commitHash string) int {
	commits := git.GetLocalCommitStack(env.cfg, env.gitcmd)
	idx := ResolveCommitIndex(commits, commitHash)
	if idx < 0 {
		fmt.Fprintf(os.Stderr, "commit %s not found in the local stack (the stack may have changed)\n", commitHash)
		os.Exit(ExitCommitNotFound)
	}
	return idx
}

// RunChild executes one `_spr` subcommand and returns the process exit code.
// Many error paths inside spr exit the process directly; that is fine — the
// parent treats any non-zero exit as a failed run.
func RunChild(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: spr-web _spr <command> [flags]")
		return ExitUsage
	}
	cmd, rest := args[0], args[1:]

	fs := flag.NewFlagSet("_spr "+cmd, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	verbose := fs.Bool("verbose", false, "log git commands and GitHub calls")
	count := fs.Int("count", -1, "limit to N pull requests from the bottom of the stack")
	commitHash := fs.String("commit", "", "target commit hash")
	var reviewers stringList
	fs.Var(&reviewers, "reviewer", "add reviewer to newly created pull requests (repeatable)")
	fs.Bool("no-rebase", false, "disable rebasing")
	fs.Bool("no-fetch", false, "disable fetching")
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	noRebase := flagWasSet(fs, "no-rebase")
	noFetch := flagWasSet(fs, "no-fetch")

	var countArg *uint
	if *count >= 0 {
		c := uint(*count)
		countArg = &c
	}

	env := initChild(*verbose)
	if noRebase {
		env.cfg.User.NoRebase = true
	}
	if noFetch {
		env.cfg.User.NoFetch = true
	}
	ctx := context.Background()

	switch cmd {
	case "info":
		return cmdInfo(ctx, env)

	case "update":
		client := githubclient.NewGitHubClient(ctx, env.cfg)
		os.Stdout = env.realStdout
		sp := spr.NewStackedPR(env.cfg, client, env.gitcmd)
		os.Stdout = os.Stderr
		sp.UpdatePullRequests(ctx, reviewers, countArg)
		return 0

	case "merge":
		client := githubclient.NewGitHubClient(ctx, env.cfg)
		os.Stdout = env.realStdout
		sp := spr.NewStackedPR(env.cfg, client, env.gitcmd)
		os.Stdout = os.Stderr
		sp.MergePullRequests(ctx, countArg)
		return 0

	case "sync":
		client := githubclient.NewGitHubClient(ctx, env.cfg)
		os.Stdout = env.realStdout
		sp := spr.NewStackedPR(env.cfg, client, env.gitcmd)
		os.Stdout = os.Stderr
		sp.SyncStack(ctx)
		return 0

	case "check":
		client := githubclient.NewGitHubClient(ctx, env.cfg)
		// RunMergeCheck prints its verdict and the mergeCheck command output
		// via bare stdout, so the real stdout stays in place here.
		os.Stdout = env.realStdout
		sp := spr.NewStackedPR(env.cfg, client, env.gitcmd)
		sp.RunMergeCheck(ctx)
		return 0

	case "amend":
		if *commitHash == "" {
			fmt.Fprintln(os.Stderr, "usage: spr-web _spr amend --commit <hash>")
			return ExitUsage
		}
		// Pre-check: spr's amend runs `git commit --fixup`, which fails
		// without staged changes. Exit with the contract code instead.
		var porcelain string
		if err := env.gitcmd.Git("status --porcelain", &porcelain); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if !HasStagedChanges(porcelain) {
			fmt.Fprintln(os.Stderr, "no staged changes to amend")
			return ExitNoStaged
		}
		idx := resolveOrExit(env, *commitHash)
		injectStdin(fmt.Sprintf("%d", idx+1))
		// AmendCommit never touches the GitHub client.
		os.Stdout = env.realStdout
		sp := spr.NewStackedPR(env.cfg, nil, env.gitcmd)
		os.Stdout = os.Stderr
		sp.AmendCommit(ctx)
		return 0

	case "edit":
		if *commitHash == "" {
			fmt.Fprintln(os.Stderr, "usage: spr-web _spr edit --commit <hash>")
			return ExitUsage
		}
		idx := resolveOrExit(env, *commitHash)
		injectStdin(fmt.Sprintf("%d", idx+1))
		os.Stdout = env.realStdout
		sp := spr.NewStackedPR(env.cfg, nil, env.gitcmd)
		os.Stdout = os.Stderr
		sp.EditCommit(ctx)
		return 0

	case "edit-done":
		os.Stdout = env.realStdout
		sp := spr.NewStackedPR(env.cfg, nil, env.gitcmd)
		os.Stdout = os.Stderr
		sp.EditCommitDone(ctx, false)
		return 0

	case "edit-abort":
		os.Stdout = env.realStdout
		sp := spr.NewStackedPR(env.cfg, nil, env.gitcmd)
		os.Stdout = os.Stderr
		sp.EditCommitAbort(ctx)
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown _spr command: %s\n", cmd)
		return ExitUsage
	}
}

// cmdInfo runs the read-only status path (github.GetInfo +
// git.GetLocalCommitStack — no fetch, no rebase) and writes the InfoPayload
// JSON to the real stdout. All spr logging is already diverted to stderr.
func cmdInfo(ctx context.Context, env *childEnv) int {
	client := githubclient.NewGitHubClient(ctx, env.cfg)
	info := client.GetInfo(ctx, env.gitcmd)
	commits := git.GetLocalCommitStack(env.cfg, env.gitcmd)

	payload := BuildInfoPayload(env.cfg, info, commits)
	if err := json.NewEncoder(env.realStdout).Encode(payload); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

type stringList []string

func (s *stringList) String() string { return fmt.Sprint(*s) }

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}
