package sprbridge

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// HandleEditSequence handles the internal `_edit-sequence` command used as a
// git sequence editor during an edit session. spr passes os.Executable() as
// the editor command, which resolves to the spr-web binary when spr is
// embedded, so spr-web has to handle this argv itself (spr's own handler
// lives in its package main and cannot be imported).
//
// Usage (invoked by git): spr-web _edit-sequence <commit-hash-prefix> <todo-file>
//
// The function returns without side effects when argv does not match, and
// exits the process when it does.
func HandleEditSequence() {
	if len(os.Args) < 4 || os.Args[1] != "_edit-sequence" {
		return
	}
	hashPrefix := os.Args[2]
	todoFile := os.Args[3]

	data, err := os.ReadFile(todoFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading todo file: %s\n", err)
		os.Exit(1)
	}

	rewritten := RewriteRebaseTodo(string(data), hashPrefix)

	if err := os.WriteFile(todoFile, []byte(rewritten), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing todo file: %s\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// RewriteRebaseTodo rewrites 'pick <hash>' to 'edit <hash>' for the target
// commit in a git interactive-rebase todo file. Behavior matches spr's own
// _edit-sequence handler.
func RewriteRebaseTodo(todo, hashPrefix string) string {
	scanner := bufio.NewScanner(strings.NewReader(todo))
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "pick "+hashPrefix) {
			line = strings.Replace(line, "pick ", "edit ", 1)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n"
}
