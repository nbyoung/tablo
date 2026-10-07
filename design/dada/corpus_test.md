# The corpus test

Test T2 of [design dada](../dada.md#tests), as the trial runs it: it lands as `internal/audit/corpus_test.go`, beside the helpers `corpus` and `derived` that `internal/derive/helpers_test.go` already holds and this package copies until `corpustest` stands (4b4f). It writes one block per entry and ref and compares the whole with [`corpus.expected.txt`](corpus.expected.txt), which lands as `testdata/corpus.expected.txt`.

No adapter from `derive.Facts` to `validate.Facts` stands before the Plumbing command (4ed9), so the test builds the Validator's diagnostics in three parts: `validate.Files` for real; `validate.Derived` with the requirements alone, for R9; and one literal diagnostic per finding that `expected.yaml` states under a history rule, with the entry's own message, H4 and H5 placed in the status file. The history rules' messages in the golden file are therefore the corpus's words, not the Validator's.

An entry prints when one of its findings has an act other than `revise`, or is P5, or when `samples` names it; a second block prints the stale statuses at three days, where there are any; an entry the Derivation refuses prints `refused`.

```go
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// diagnostics gives what the Validator reports: Files and R9 for real, and
// the history rules from the findings expected.yaml states.
func diagnostics(f *derive.Facts, want map[string]any, labels map[string]string) []model.Diagnostic {
	p := f.Project()
	out := validate.Files(p)
	facts := &validate.Facts{}
	for _, id := range f.Order() {
		for i, c := range f.Requires(id) {
			if c.Why != derive.Determined {
				continue
			}
			facts.Requirements = append(facts.Requirements, validate.Condition{Task: id, Index: i, Origin: c.Origin,
				From: c.From, To: c.To, Stands: c.Stands, Met: c.Met, Due: c.Due, MetAtTip: c.AtTrunk == derive.Yes})
		}
	}
	out = append(out, validate.Derived(p, facts)...)
	list, _ := want["findings"].([]any)
	for _, item := range list {
		m := item.(map[string]any)
		rule := str(m["rule"])
		tier, _ := validate.RuleOf(rule)
		if tier.Tier != validate.TierHistory {
			continue
		}
		d := model.Diagnostic{Code: rule, Severity: tier.Severity, Task: str(m["task"]), Gate: str(m["gate"]),
			Commit: labels[str(m["commit"])], Trailer: str(m["trailer"]), Message: str(m["message"])}
		d.Pos.File = str(m["file"])
		if rule == "H4" || rule == "H5" {
			d.Pos.File = "status/" + d.Task + ".yaml"
		}
		out = append(out, d)
	}
	validate.Sort(out)
	return out
}

var samples = map[string]bool{"file-duplicate-key": true, "gates-bad-key": true, "task-bad-filename": true, "siblings-same-order": true, "status-on-parent": true, "status-no-task": true, "subproject-path-missing": true}

func TestCorpus(t *testing.T) {
	build := corpus(t)
	root := filepath.Join(filepath.Dir(build), "entries")
	dirs, _ := os.ReadDir(root)
	var b strings.Builder
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(root, d.Name(), "expected.yaml"))
		if err != nil {
			continue
		}
		var want map[string]any
		if err := yaml.Unmarshal(data, &want); err != nil {
			t.Fatal(err)
		}
		if _, inPlace := want["source"]; inPlace {
			continue
		}
		if _, err := os.Stat(filepath.Join(build, d.Name())); err != nil {
			continue
		}
		ref, trunk := "main", ""
		if r := str(want["ref"]); r != "" {
			ref = r
		}
		if m, ok := want["trunk"].(map[string]any); ok {
			trunk = str(m["caller"])
		}
		replace := map[string]string{}
		if m, ok := want["replace"].(map[string]any); ok {
			for url, name := range m {
				replace[url] = filepath.Join(build, d.Name()+"."+str(name))
			}
		}
		labels, names := map[string]string{}, map[string]string{}
		files, _ := filepath.Glob(filepath.Join(build, d.Name()+".*labels.txt"))
		for _, file := range files {
			data, _ := os.ReadFile(file)
			for _, line := range regexp.MustCompile(`(?m)^(\S+) ([0-9a-f]+)$`).FindAllStringSubmatch(string(data), -1) {
				labels[line[1]] = line[2]
				names[line[2]] = line[1]
			}
		}
		refs := []string{ref}
		if m, ok := want["refs"].(map[string]any); ok {
			for r := range m {
				refs = append(refs, r)
			}
			sort.Strings(refs[1:])
		}
		for _, ref := range refs {
			f := derived(t, load.Options{Replace: replace}, filepath.Join(build, d.Name()), ref, trunk)
			var w map[string]any
			if ref == refs[0] {
				w = want
			}
			for _, stale := range []int{0, 3} {
				rep := Run(Input{Facts: f, Diagnostics: diagnostics(f, w, labels), Stale: stale})
				if rep == nil {
					if stale == 0 {
						fmt.Fprintf(&b, "%s %s: refused\n", d.Name(), ref)
					}
					continue
				}
				if stale == 3 {
					only := rep.Keep(func(x *Finding) bool { return x.Key == Stale })
					if len(only.Findings) == 0 {
						continue
					}
					rep = only
				}
				plain := true
				for _, x := range rep.Findings {
					plain = plain && x.Act == Revise && x.Key != "P5"
				}
				if plain && !samples[d.Name()] {
					continue
				}
				c := rep.Counts()
				fmt.Fprintf(&b, "%s %s: now %s, stale %d: %d error(s), %d warning(s), %d information", d.Name(), ref, rep.Now, rep.Stale, c.Errors, c.Warnings, c.Information)
				if m := rep.Most(); m != nil {
					fmt.Fprintf(&b, "; most %s %d", m.Email, m.Count)
				}
				b.WriteString("\n")
				for _, x := range rep.Rows() {
					var ids, commits []string
					for _, task := range x.Tasks {
						ids = append(ids, task.ID)
					}
					for _, c := range x.Commits {
						commits = append(commits, names[c.ID])
					}
					fmt.Fprintf(&b, "  %s %s [%s] at %q in %v: %s -> %s\n", x.Key, x.Severity, strings.Join(ids, " "), x.Gate, x.Files, x.Act, x.Resolver)
					fmt.Fprintf(&b, "    message: %s\n    action: %s\n    commits: %s\n", x.Message, x.Action, strings.Join(commits, " "))
					for _, fact := range x.Facts {
						fmt.Fprintf(&b, "    %s: %s\n", fact.Name, fact.Value)
					}
					for _, c := range x.Commands {
						text := c.Text
						for hash, name := range names {
							text = strings.ReplaceAll(text, hash, "<"+name+">")
						}
						fmt.Fprintf(&b, "    $ %s  # %s\n", text, c.Comment)
					}
					if x.Sentence == "" || x.Source.URL == "" || x.Kind == "" {
						t.Errorf("%s %s: no sentence, source or kind", d.Name(), x.Key)
					}
					if _, err := json.Marshal(x); err != nil {
						t.Error(err)
					}
				}
			}
		}
	}
	want, err := os.ReadFile(filepath.Join("testdata", "corpus.expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != string(want) {
		t.Errorf("the audit of the corpus differs from testdata/corpus.expected.txt:\n%s", b.String())
	}
}
```
