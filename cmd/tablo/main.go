// Command tablo is the Tableaux plumbing command. It reads a Tableaux project
// from a Git repository and writes one result per run: any of the ten views
// as data, the validator's diagnostics, a commit trailer, or its own version.
// Scripts, hooks, CI and the conformance run call it; the front ends link the
// library behind it. tablo --help lists every command, option and exit code.
package main

import (
	"context"
	"os"

	"github.com/nbyoung/tablo/internal/cli"
)

func main() { os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) }
