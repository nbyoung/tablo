// Command tablo is the Tableaux plumbing command. It reads a repository and
// emits data for the porcelain front ends.
//
// This skeleton prints the version only. The plumbing command task adds the
// validate, audit, view, status, history and trailer subcommands.
package main

import (
	"fmt"
	"os"

	"github.com/nbyoung/tablo"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] != "version" {
		fmt.Fprintf(os.Stderr, "tablo: unknown command %q\nusage: tablo [version]\n", os.Args[1])
		os.Exit(2)
	}
	fmt.Printf("tablo %s (tableaux %d.%d)\n", tablo.Version, tablo.AcceptedMajor, tablo.AcceptedMinor)
}
