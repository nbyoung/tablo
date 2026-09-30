// Command dada demonstrates the audit: it reads a Tableaux project from a Git
// ref and prints the discrepancies between its files and its history as a
// Markdown findings table.
//
//	dada [-labels file] <repo> [ref]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	labels := flag.String("labels", "", "corpus labels file (default <repo>.labels.txt when present)")
	flag.Parse()
	if flag.NArg() < 1 || flag.NArg() > 2 {
		fmt.Fprintln(os.Stderr, "usage: dada [-labels file] <repo> [ref]")
		os.Exit(2)
	}
	repo, ref := flag.Arg(0), "main"
	if flag.NArg() == 2 {
		ref = flag.Arg(1)
	}
	if *labels == "" {
		*labels = strings.TrimSuffix(repo, "/") + ".labels.txt"
	}
	findings, err := Audit(Repo(repo), ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dada:", err)
		os.Exit(1)
	}
	fmt.Print(render(findings, readLabels(*labels)))
}

// readLabels maps commit hashes to corpus labels; a missing file gives none.
func readLabels(path string) map[string]string {
	m := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if a, b, ok := strings.Cut(sc.Text(), " "); ok {
			m[strings.TrimSpace(b)] = a
		}
	}
	return m
}

func render(fs []Finding, labels map[string]string) string {
	if len(fs) == 0 {
		return "No findings.\n"
	}
	var b strings.Builder
	b.WriteString("| Finding | Rule | Task | Gate | Commit | Action |\n|---|---|---|---|---|---|\n")
	for _, f := range fs {
		commit := f.Commit[:7]
		if l := labels[f.Commit]; l != "" {
			commit = l + " " + commit
		}
		action := f.Action
		if f.Trailer != "" {
			action = "`" + f.Trailer + "`: " + action
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", f.Kind, f.Rule, f.Task, f.Gate, commit, action)
	}
	return b.String()
}
