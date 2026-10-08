# The corpus test

Test T2 of [design dada](../dada.md#tests): it lands as `internal/audit/corpus_test.go`, beside the helpers `corpus` and `derived` that `internal/derive/helpers_test.go` already holds and this package copies until `corpustest` stands (4b4f). It writes one block per entry and ref and compares the whole with [`corpus.expected.txt`](corpus.expected.txt), which lands as `testdata/corpus.expected.txt`.

No adapter from `derive.Facts` to `validate.Facts` stands before the Plumbing command (4ed9), so the test builds the Validator's diagnostics in three parts: `validate.Files` for real; `validate.Derived` with the requirements alone, for R9; and one literal diagnostic per finding that `expected.yaml` states under a history rule, with the entry's own message, H4 and H5 placed in the status file. The history rules' messages in the golden file are therefore the corpus's words, not the Validator's.

The test passes `Now` as the author date of the source commit, since the library requires a date, and lists at the minimum `information`, so the golden file shows every row. An entry prints when one of its findings has an act other than `revise` and is no sole review, or is P5, or when `samples` names it; a second block prints the stale statuses at three days, where there are any; an entry the Derivation refuses prints `refused`. A row prints its typed facts by name, per task: T7 and T14 hold each to the value of the Derivation. A row of sole reviews prints one line.

The file as it stands is no single run. Its diagnostics, messages, commits, commands and the facts each row names come from the first trial's output, rewritten by a script: the action lines go, the worded facts become their names, a path takes `.tableaux/`, `advance` reads `await`, and `most` skips the act `none`. Its rows of sole reviews and its counts of information come from a second trial over `derive.Facts` on `main` at `0883ec1`, which also finds that no entry demotes a finding; a third script drops the rows at the junction the authorisation reviews and lowers the counts by them. The implementation regenerates the file and reads the difference.

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

// factNames names the typed facts a finding holds, in the order of Facts.
func factNames(f Facts) []string {
	var has []string
	for _, x := range []struct {
		name string
		set  bool
	}{
		{"junction", f.Junction != nil}, {"status", f.Status != nil}, {"requirement", f.Requirement != nil},
		{"authorisation", f.Authorisation != nil}, {"linkage", f.Snapshot != nil}, {"model", f.Reading != nil},
	} {
		if x.set {
			has = append(has, x.name)
		}
	}
	return has
}

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
			now := ""
			if c := f.Log().Commit(f.Log().Source); c != nil {
				now = c.Date()
			}
			for _, stale := range []int{0, 3} {
				rep, err := Run(Input{Facts: f, Diagnostics: diagnostics(f, w, labels), Now: now, Stale: stale})
				if err != nil {
					t.Fatalf("%s %s: %v", d.Name(), ref, err)
				}
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
					plain = plain && (x.Act == Revise || x.Key == SoleReview) && x.Key != "P5"
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
				for _, row := range rep.Rows(model.Information) {
					x := row[0]
					var ids, files, commits []string
					seen := map[string]bool{}
					for _, y := range row {
						if y.Task != "" {
							ids = append(ids, y.Task)
						}
						if y.File != "" && !seen[y.File] {
							seen[y.File] = true
							files = append(files, y.File)
						}
						for _, c := range y.Commits {
							if !seen[c.ID] {
								seen[c.ID] = true
								commits = append(commits, names[c.ID])
							}
						}
					}
					fmt.Fprintf(&b, "  %s %s [%s] at %q in [%s]: %s -> %s\n", x.Key, x.Severity, strings.Join(ids, " "), x.Gate, strings.Join(files, " "), x.Act, x.Resolver)
					if x.Sentence == "" || x.Source.URL == "" || x.Kind == "" {
						t.Errorf("%s %s: no sentence, source or kind", d.Name(), x.Key)
					}
					if x.Key == SoleReview {
						continue
					}
					fmt.Fprintf(&b, "    message: %s\n    commits: %s\n", x.Message, strings.Join(commits, " "))
					for _, y := range row {
						if has := factNames(y.Facts); len(has) > 0 {
							fmt.Fprintf(&b, "    facts %s: %s\n", y.Task, strings.Join(has, " "))
						}
					}
					for _, c := range x.Commands {
						text := c.Text
						for hash, name := range names {
							text = strings.ReplaceAll(text, hash, "<"+name+">")
						}
						fmt.Fprintf(&b, "    $ %s  # %s\n", text, c.Comment)
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
