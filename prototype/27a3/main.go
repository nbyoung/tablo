// Command 27a3 demonstrates the derivations that read Git history and the
// inheritance chain: authorisation from the trunk's first-parent deciding
// commit, review from Reviewed: trailers by the junction's reviewer, and each
// junction resolved through the ancestor chain with the provenance of every
// field. With -expected it compares the result with a corpus expected.yaml.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	repo := flag.String("repo", "", "path of the Git repository")
	ref := flag.String("ref", "HEAD", "branch or commit in view")
	caller := flag.String("trunk", "", "trunk named by the caller, used when the project names none")
	labels := flag.String("labels", "", "corpus <entry>.labels.txt, to print labels for hashes")
	expected := flag.String("expected", "", "corpus expected.yaml to compare with")
	flag.Parse()
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "usage: 27a3 -repo DIR [-ref REF] [-trunk BRANCH] [-labels FILE] [-expected FILE]")
		os.Exit(2)
	}
	d, err := derive(*repo, *ref, *caller)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	lab := readLabels(*labels)
	var b strings.Builder
	d.print(&b, lab)
	fmt.Print(b.String())
	if *expected != "" {
		src, err := os.ReadFile(*expected)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		out, bad := compare(*repo, d, lab, asMap(parseYAML(string(src))))
		fmt.Println("\ncomparison with", *expected)
		fmt.Print(out)
		if bad > 0 {
			os.Exit(1)
		}
	}
}

func readLabels(file string) map[string]string {
	lab := map[string]string{}
	if file == "" {
		return lab
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return lab
	}
	for _, l := range strings.Split(string(b), "\n") {
		if f := strings.Fields(l); len(f) == 2 {
			lab[f[1]] = f[0]
		}
	}
	return lab
}
