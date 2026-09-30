// Command 4f60 is the loader prototype: it reads .tableaux at a Git ref
// through git ls-tree and git cat-file and prints the typed model with
// positions.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	dir := flag.String("C", ".", "repository")
	labels := flag.String("labels", "", "corpus labels file mapping label to commit")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: 4f60 [-C repo] [-labels file] ref")
		os.Exit(2)
	}
	ref := flag.Arg(0)
	if *labels != "" {
		if h, err := lookup(*labels, ref); err == nil {
			ref = h
		} else {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	p, err := Loader{*dir}.Load(ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report(os.Stdout, p)
	if len(p.Diags) > 0 {
		os.Exit(1)
	}
}

func lookup(file, label string) (string, error) {
	f, err := os.Open(file) //nolint:gosec // a path given on the command line
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), " "); ok && k == label {
			return v, nil
		}
	}
	return "", fmt.Errorf("label %s not in %s", label, file)
}

func report(w *os.File, p *Project) {
	_, _ = fmt.Fprintf(w, "commit %.8s (%s)\n", p.Commit, p.Ref)
	_, _ = fmt.Fprintf(w, "version.yaml  tableaux %s at %s, trunk %s\n", p.Version.V, p.Version.Pos, p.Trunk.V)
	ids := make([]string, 0, len(p.Tasks))
	for id := range p.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t := p.Tasks[id]
		_, _ = fmt.Fprintf(w, "task %s  %-14q parent %-4s assignee %s at %s\n",
			id, t.Title.V, t.ParentID.V, t.Assignee.V, t.Assignee.Pos)
	}
	_, _ = fmt.Fprintf(w, "%d gates, %d tasks, %d statuses\n", len(p.Gates), len(p.Tasks), len(p.Statuses))
	for _, d := range p.Diags {
		_, _ = fmt.Fprintln(w, "error", d)
	}
}
