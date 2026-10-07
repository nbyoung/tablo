// Package validate is the Validator: it applies the rules of the method's
// RULES.md to a project the Loader read and returns diagnostics, each with a
// rule id, a severity and a position. Files applies the rules that the files
// alone decide, and Derived the rules that read derived facts, which arrive as
// plain data. The Loader keeps the rules of reading, L1 to L4, and P4. The
// package reads no file, no history, no environment and no clock, and both
// functions are safe for concurrent use.
package validate

import (
	"fmt"
	"sort"

	"github.com/nbyoung/tablo/internal/model"
)

// run is one call of Files or of Derived: the project, its shape and what the
// rules find. A rule in code is one method named by its id; a rule that ends
// a sequence of rules returns true.
type run struct {
	project *model.Project
	shape   *shape
	linked  map[*model.Project]*shape // the shape of each linked project a rule reads
	facts   *Facts                    // Derived alone
	edges   []edge                    // the requirement edges R2 reads
	found   []model.Diagnostic
}

// Files applies every rule that the files alone decide. It returns the
// project's diagnostics: the Loader's, copied from p.Diagnostics, and the
// Validator's, in the order of Sort, each one once. It reads nothing, never
// fails and never panics, whatever the files hold.
func Files(p *model.Project) []model.Diagnostic {
	if p == nil {
		return nil
	}
	r := &run{project: p, linked: map[*model.Project]*shape{}}
	r.files()
	all := append(append([]model.Diagnostic{}, p.Diagnostics...), r.found...)
	Sort(all)
	return unique(all)
}

// files applies the rules in the order the design gives: the project, the
// version, the gates, the tree, each task file, the requirement cycles and
// each status file.
func (r *run) files() {
	p := r.project
	if r.p1() {
		return
	}
	r.p2()
	if p.Version != nil {
		r.leaves(checkFile(versionFile, p.Version.File, ""))
		if p.Version.WellFormed && !p.Version.Accepted {
			// The Loader raises P4, and the schemas of this language version
			// misjudge a file written for another.
			return
		}
	}
	r.shape = newShape(p)
	r.gating()
	r.tree()
	for _, id := range p.TaskIDs() {
		r.task(p.Tasks[id])
	}
	r.r2()
	for _, id := range sortedKeys(p.Statuses) {
		r.status(p.Statuses[id])
	}
}

// p1 reports a source with no project, and ends the run.
func (r *run) p1() bool {
	if r.project.Exists {
		return false
	}
	where := r.project.Where.Dir
	if where == "" {
		where = "the repository's root"
	}
	r.add("P1", model.Pos{}, "", "", "no .tableaux directory at or above %s", where)
	return true
}

// p2 reports a project with no version file.
func (r *run) p2() {
	if r.file("version.yaml") == nil {
		r.add("P2", model.Pos{File: "version.yaml"}, "", "", "version.yaml is missing")
	}
}

// file returns the file of the project at a path below .tableaux, or nil.
func (r *run) file(path string) *model.File {
	for _, file := range r.project.Files {
		if file.Path == path {
			return file
		}
	}
	return nil
}

// add records a diagnostic of a rule in code, with the rule's severity.
func (r *run) add(code string, pos model.Pos, task, gate, format string, args ...any) {
	rule, _ := RuleOf(code)
	r.found = append(r.found, model.Diagnostic{
		Code:     code,
		Severity: rule.Severity,
		Pos:      pos,
		Task:     task,
		Gate:     gate,
		Message:  fmt.Sprintf(format, args...),
	})
}

// leaves records the diagnostics of the schema leaves of one file or entry,
// and returns the rules they raise.
func (r *run) leaves(found []leaf) map[string]bool {
	codes := map[string]bool{}
	for _, l := range found {
		r.found = append(r.found, l.diagnostic)
		codes[l.diagnostic.Code] = true
	}
	return codes
}

// shapeOf returns the shape of a linked project, built once per run.
func (r *run) shapeOf(p *model.Project) *shape {
	if p == r.project && r.shape != nil {
		return r.shape
	}
	s, ok := r.linked[p]
	if !ok {
		s = newShape(p)
		r.linked[p] = s
	}
	return s
}

// posOf returns the position of the first node that exists, and the file
// alone when none does.
func posOf(file *model.File, nodes ...*model.Value) model.Pos {
	for _, node := range nodes {
		if node != nil {
			return node.Pos
		}
	}
	return model.Pos{File: file.Path}
}

// Sort orders diagnostics as the envelope lists them: by file, a diagnostic
// with none first, then line, column, code, task, gate, commit and message;
// the trailer breaks a tie.
func Sort(d []model.Diagnostic) {
	sort.SliceStable(d, func(i, j int) bool {
		a, b := d[i], d[j]
		switch {
		case a.Pos.File != b.Pos.File:
			return a.Pos.File < b.Pos.File
		case a.Pos.Line != b.Pos.Line:
			return a.Pos.Line < b.Pos.Line
		case a.Pos.Col != b.Pos.Col:
			return a.Pos.Col < b.Pos.Col
		case a.Code != b.Code:
			return a.Code < b.Code
		case a.Task != b.Task:
			return a.Task < b.Task
		case a.Gate != b.Gate:
			return a.Gate < b.Gate
		case a.Commit != b.Commit:
			return a.Commit < b.Commit
		case a.Message != b.Message:
			return a.Message < b.Message
		}
		return a.Trailer < b.Trailer
	})
}

// unique drops each diagnostic of a sorted list that equals the one before it
// in every field, so the two leaves of one oneOf give one diagnostic.
func unique(d []model.Diagnostic) []model.Diagnostic {
	kept := d[:0]
	for i, diagnostic := range d {
		if i == 0 || diagnostic != d[i-1] {
			kept = append(kept, diagnostic)
		}
	}
	return kept
}
