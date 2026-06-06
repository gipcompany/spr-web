// Command spr-web serves a local web UI for managing stacked pull requests
// created with spr (https://github.com/ejoffe/spr).
//
// The same binary doubles as its own child process: spr commands are executed
// by re-executing this binary with the hidden `_spr` subcommand so that spr's
// os.Exit / log.Fatal based error handling cannot take the server down.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gipcompany/spr-web/internal/server"
	"github.com/gipcompany/spr-web/internal/sprbridge"
	"github.com/gipcompany/spr-web/web"
)

// Set via -ldflags at release time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Invoked by git as a sequence editor during an edit session: spr passes
	// os.Executable() as the editor, which points at this binary.
	sprbridge.HandleEditSequence()

	// Hidden child-process mode: `spr-web _spr <cmd> [flags]`.
	if len(os.Args) >= 2 && os.Args[1] == "_spr" {
		os.Exit(sprbridge.RunChild(os.Args[2:]))
	}

	port := flag.Int("port", 0, "port to listen on (default: 7780, scanning upward when busy)")
	noOpen := flag.Bool("no-open", false, "do not open the browser on startup")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("spr-web %s (spr %s, commit %s, built %s)\n",
			version, sprbridge.SprVersion(), commit, date)
		return
	}

	srv, err := server.New(server.Options{
		Port:   *port,
		NoOpen: *noOpen,
		Static: web.Dist(),
		Version: server.VersionInfo{
			Version: version,
			Commit:  commit,
			Date:    date,
			Spr:     sprbridge.SprVersion(),
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "spr-web:", err)
		os.Exit(1)
	}
	if err := srv.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "spr-web:", err)
		os.Exit(1)
	}
}
