package sprbridge

// Contract exit codes emitted by our own `_spr` wrapper code. spr's internal
// exit codes are NOT a stable contract and are never interpreted; only the
// codes below are produced by spr-web itself and may be relied upon by the
// server and the UI.
const (
	// ExitConfig is returned when the repository or spr configuration is
	// invalid (mirrors spr's own behavior of exiting 2 on config errors).
	ExitConfig = 2

	// ExitCommitNotFound is returned when the commit hash sent by the UI can
	// no longer be resolved in the local commit stack (the stack changed
	// between display and execution).
	ExitCommitNotFound = 10

	// ExitNoStaged is returned when amend is invoked without staged changes.
	ExitNoStaged = 11

	// ExitUsage is returned on malformed `_spr` arguments (a spr-web bug).
	ExitUsage = 64
)
