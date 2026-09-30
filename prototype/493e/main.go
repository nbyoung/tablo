// Command 493e demonstrates the structural views as data: the gate definition,
// task definition, authority delegation and task assignment views, derived from
// a Tableaux project read through Git. It is a prototype of task 493e, with
// stand-ins for the loader and derivation (see README.md).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	repo := flag.String("repo", ".", "the Git repository of the project")
	ref := flag.String("ref", "HEAD", "the commit whose files and history are in view")
	view := flag.String("view", "gate", "gate, task, authority or assignment")
	task := flag.String("task", "", "one task, or the subtree under it")
	person := flag.String("person", "", "one email")
	level := flag.String("level", "provenance", "glance, detail or provenance")
	columns := flag.String("columns", "", "comma-separated gate keys")
	window := flag.Int("window", -1, "gate columns either side of the next gates in view")
	proposed := flag.Bool("proposed", false, "authority view: proposed tasks only")
	flag.Parse()
	pr := Params{Task: *task, Person: *person, Level: *level, Columns: split(*columns), Window: *window, Proposed: *proposed}
	r, err := newRepo(*repo, *ref)
	if err == nil {
		var out M
		if out, err = render(r, *view, pr); err == nil {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			err = enc.Encode(out)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "493e:", err)
		os.Exit(1)
	}
}
