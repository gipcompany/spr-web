# spr-web

A local web UI for managing stacked pull requests created with
[spr](https://github.com/ejoffe/spr).

Run `spr-web` inside a repository configured for spr and your browser opens a
visual view of the commit stack — with one-click access to the spr commands
you would otherwise run in a terminal: `update`, `merge`, `sync`, `check`,
`amend`, and interactive `edit` sessions.

<!-- TODO: demo GIF / screenshot -->

## Features

- **See the stack**: every commit, its pull request, and the same
  `[✔✗·✔]` status bits the spr CLI prints (checks / review / conflicts /
  stacked) — plus a per-PR detail view with a mergeability verdict.
- **Run spr from the browser**: update, sync, check, and merge (with a
  confirmation listing the PRs expected to merge). Live log streaming;
  reloading the page never loses a running command.
- **Amend & edit**: pick a commit, amend it with your staged changes, or
  start an spr edit session (interactive rebase). Conflict resolution happens
  in your local editor; the UI tracks the session state and offers
  done / abort.
- **Local-first**: binds to 127.0.0.1 only. One instance per repository;
  different repositories run on different ports side by side.
- **Zero extra setup**: GitHub authentication reuses spr's token discovery
  (`GITHUB_TOKEN`, the gh CLI config, or the OS keyring). If `git spr` works,
  `spr-web` works.

## Installation

### Homebrew (macOS / Linux)

```sh
brew install gipcompany/tap/spr-web
```

### GitHub Releases

Download a binary for your platform from the
[releases page](https://github.com/gipcompany/spr-web/releases)
(darwin / linux, amd64 / arm64).

### From source

Requires Go 1.23+ and pnpm. `go install` is not supported because the
frontend is embedded at build time:

```sh
git clone https://github.com/gipcompany/spr-web.git
cd spr-web
make build   # builds web/ with pnpm, then the Go binary into bin/spr-web
```

## Usage

```sh
cd your-repo   # a repository already configured for spr
spr-web
```

This checks your spr configuration and GitHub connectivity (exactly like
`git spr status` would), starts a server on `http://127.0.0.1:7780` (the next
free port if taken), and opens your browser.

Flags:

| Flag        | Description                                          |
| ----------- | ---------------------------------------------------- |
| `--port N`  | Listen on a specific port (no fallback scan).        |
| `--no-open` | Do not open the browser automatically.               |
| `--version` | Print the spr-web and embedded spr versions.         |

## How it works

- spr is embedded as a Go library and pinned in `go.mod` — no separate spr
  installation is needed, and the UI always matches the embedded version
  (shown in the footer).
- Every spr command runs as a child process (the binary re-executes itself),
  so spr's `os.Exit`-style error handling can never take the server down.
  Command output is streamed to the browser over SSE and buffered server-side
  for replay.
- One command runs at a time per repository (concurrent requests get 409).
  A running command can be interrupted (SIGINT, then SIGKILL after a grace
  period) with an explicit warning, since this can leave a rebase in
  progress.
- File edits and conflict resolution stay in your local editor; the browser
  only drives spr and shows state.

## Supported platforms

macOS and Linux. Windows is not supported (interrupt handling relies on
process groups and SIGINT semantics that do not translate).

## Known limitations

- If spr-web is restarted in the middle of an edit session it will fail its
  startup check; finish the session in a terminal first
  (`git spr edit --done` or `git spr edit --abort`).
- The server trusts its local environment: no authentication is performed
  (127.0.0.1 binding only).

## Development

```sh
# Terminal 1: backend, inside a spr-configured repository
go run ./cmd/spr-web --no-open

# Terminal 2: frontend with HMR (proxies /api to port 7780)
cd web && pnpm dev
```

Tests: `make test` (Go vet + tests, TypeScript typecheck, Biome).

## License

[MIT](LICENSE) — consistent with spr itself, which this tool embeds.
