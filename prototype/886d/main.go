// Command 886d demonstrates the status views as data: the global tableau, the
// contextual tableau, the work-blockage tree and the contributor work queue,
// with the roll-up of children to parents and the default gate window with
// its folded-column counts. It reads a Tableaux project at a Git ref.
//
//	go run ./prototype/886d [flags] tableau|contextual|blockage|queue
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

const corpus = "/home/nbyoung/Projects/Tableaux/tableaux/corpus/build/weather-station"

func main() {
	fs := flag.NewFlagSet("886d", flag.ExitOnError)
	repo := fs.String("repo", corpus, "the Git repository holding .tableaux")
	ref := fs.String("ref", "main", "the ref in view")
	o := Options{}
	fs.StringVar(&o.Task, "task", "", "focus on the subtree under a task")
	fs.StringVar(&o.Person, "person", "", "focus on a person's email")
	fs.IntVar(&o.Window, "window", 1, "columns either side of the next gates (Q2)")
	fs.StringVar(&o.Cell, "cell", "state-at-gate", "state-at-gate or state-at-next (Q5)")
	fs.StringVar(&o.Order, "order", "kind", "kind or dependents (Q3)")
	fs.StringVar(&o.Level, "level", "detail", "glance or detail")
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: 886d [flags] tableau|contextual|blockage|queue")
		os.Exit(2)
	}
	// The view name comes first so that flags may follow it.
	view := os.Args[1]
	_ = fs.Parse(os.Args[2:])
	p, err := Load(*repo, *ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var out any
	switch view {
	case "tableau":
		out = p.Global(o)
	case "contextual":
		out = p.Contextual(o)
	case "blockage":
		out = p.Blockage(o)
	case "queue":
		out = p.Queue(o)
	default:
		fmt.Fprintln(os.Stderr, "unknown view", view)
		os.Exit(2)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
