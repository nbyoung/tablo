package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const corpus = "/home/nbyoung/Projects/Tableaux/tableaux/corpus"

func TestParseFlow(t *testing.T) {
	got := parseFlow(`{ contributor: opus@example.org, model: claude-opus-5-5, references: [{ url: a, text: b }] }`)
	if got["contributor"] != "opus@example.org" || got["model"] != "claude-opus-5-5" || got["references"] == "" {
		t.Fatalf("parseFlow = %v", got)
	}
}

// key reduces a finding to the fields its rule fixes in expected.yaml.
func key(rule, task, gate, commit, trailer string) string {
	switch rule {
	case "H1":
		return strings.Join([]string{rule, commit, trailer}, "|")
	case "S11":
		return strings.Join([]string{rule, task, gate}, "|")
	}
	return strings.Join([]string{rule, task, gate, commit}, "|")
}

var (
	field   = regexp.MustCompile(`(rule|task|gate|commit|trailer): "?([^,"}]+(?: [^,"}]+)*)"?`)
	message = regexp.MustCompile(`message: ".*?"`)
	taskHdr = regexp.MustCompile(`^  "([0-9a-f]{4})":`)
	authz   = regexp.MustCompile(`authorisation: \{ state: (\w+), commit: (\w+)`)
)

// expectedFindings reads the H1, H2, H3 and S11 findings of expected.yaml.
func expectedFindings(text string) []string {
	var keys []string
	in := false
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		m := map[string]string{}
		for _, f := range field.FindAllStringSubmatch(message.ReplaceAllString(cur.String(), ""), -1) {
			m[f[1]] = strings.TrimSpace(f[2])
		}
		switch m["rule"] {
		case "H1", "H2", "H3", "S11":
			keys = append(keys, key(m["rule"], m["task"], m["gate"], m["commit"], m["trailer"]))
		}
		cur.Reset()
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "findings:"):
			in = true
		case in && strings.HasPrefix(strings.TrimSpace(line), "- {"):
			flush()
			cur.WriteString(line)
		case in && strings.HasPrefix(line, "   "):
			cur.WriteString(" " + line)
		case in && line != "" && !strings.HasPrefix(line, " "):
			flush()
			in = false
		}
	}
	flush()
	sort.Strings(keys)
	return keys
}

// expectedProposed reads task -> "proposed"/"authorised" and the deciding
// commit label, for the tasks whose authorisation expected.yaml states.
func expectedProposed(text, section string) map[string][2]string {
	out := map[string][2]string{}
	if section != "" {
		i := strings.Index(text, "\n  "+section+":")
		if i < 0 {
			return out
		}
		text = text[i+1:]
	}
	id := ""
	for _, line := range strings.Split(text, "\n") {
		if m := taskHdr.FindStringSubmatch(line); m != nil {
			id = m[1]
		}
		if m := authz.FindStringSubmatch(line); m != nil && id != "" {
			out[id] = [2]string{m[1], m[2]}
		}
	}
	return out
}

func labels(t *testing.T, entry string) (byHash, byLabel map[string]string) {
	t.Helper()
	byHash, byLabel = map[string]string{}, map[string]string{}
	for h, l := range readLabels(filepath.Join(corpus, "build", entry+".labels.txt")) {
		byHash[h], byLabel[l] = l, h
	}
	return
}

func TestCorpus(t *testing.T) {
	if _, err := os.Stat(filepath.Join(corpus, "build")); err != nil {
		t.Skip("corpus not built")
	}
	// weather-station holds one finding that expected.yaml does not state:
	// c07d passes the mockup junction that ben reviews with no Reviewed commit.
	extra := map[string][]string{"weather-station": {"S11|c07d|mockup"}}
	for _, entry := range []string{"review-by-non-reviewer", "model-mismatch", "unknown-trailer", "status-unreviewed-gate", "weather-station"} {
		t.Run(entry, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(corpus, "entries", entry, "expected.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			names, _ := labels(t, entry)
			findings, err := Audit(Repo(filepath.Join(corpus, "build", entry)), "main")
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			gotProposed := map[string]string{}
			for _, f := range findings {
				if f.Kind == "proposed" {
					gotProposed[f.Task] = names[f.Commit]
					continue
				}
				got = append(got, key(f.Rule, f.Task, f.Gate, names[f.Commit], f.Trailer))
			}
			want := append(expectedFindings(text), extra[entry]...)
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("findings\n got %q\nwant %q", got, want)
			}
			for id, a := range expectedProposed(strings.SplitN(text, "\nrefs:", 2)[0], "") {
				commit, isProposed := gotProposed[id]
				if (a[0] == "proposed") != isProposed || (isProposed && commit != a[1]) {
					t.Errorf("task %s: expected %v, audit proposed=%v at %q", id, a, isProposed, commit)
				}
			}
		})
	}
}

func TestCorpusBranchProposed(t *testing.T) {
	if _, err := os.Stat(filepath.Join(corpus, "build")); err != nil {
		t.Skip("corpus not built")
	}
	raw, err := os.ReadFile(filepath.Join(corpus, "entries", "weather-station", "expected.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	names, _ := labels(t, "weather-station")
	findings, err := Audit(Repo(filepath.Join(corpus, "build", "weather-station")), "sensor-board")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range findings {
		if f.Kind == "proposed" && names[f.Commit] == "W3" {
			got[f.Task] = true
		}
	}
	// Every task the branch holds is proposed; 3c5d does not exist there yet.
	for _, id := range []string{"a1c0", "4e2b", "7b2e", "c07d", "9f31"} {
		if !strings.Contains(string(raw), `"`+id+`": { authorisation: { state: proposed }`) || !got[id] {
			t.Errorf("task %s: not proposed at W3 on sensor-board", id)
		}
	}
}

// gitIn runs git in dir as one identity, with a fixed date.
func gitIn(t *testing.T, dir, email string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL="+email, "GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL="+email,
		"GIT_AUTHOR_DATE=2026-09-01T12:00:00+00:00", "GIT_COMMITTER_DATE=2026-09-01T12:00:00+00:00")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestTaskEditIsWorkAtDefined checks D8: a task-file edit under a model
// outside the defined junction's model is a mismatch at defined, whatever
// gate the task's status stands at.
func TestTaskEditIsWorkAtDefined(t *testing.T) {
	dir := t.TempDir()
	write := func(path, text string) {
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".tableaux/version.yaml", "tableaux: 0.2.1\ntrunk: main\n")
	write(".tableaux/gates.yaml", "gates:\n  - { key: undefined, symbol: a }\n  - { key: defined, symbol: b }\n  - { key: function, symbol: c }\nstates:\n  - { key: nominal }\n")
	write(".tableaux/tasks/aaaa.yaml", "title: Root\nassignee: olive@example.org\njunctions:\n  defined: { contributor: bot@example.org, model: claude-fable }\n")
	write(".tableaux/status/aaaa.yaml", "gate: function\nstate: nominal\n")
	gitIn(t, dir, "olive@example.org", "init", "-q", "-b", "main")
	gitIn(t, dir, "olive@example.org", "add", ".")
	gitIn(t, dir, "olive@example.org", "commit", "-q", "-m", "Plan")
	write(".tableaux/tasks/aaaa.yaml", "title: Root, revised\nassignee: olive@example.org\njunctions:\n  defined: { contributor: bot@example.org, model: claude-fable }\n")
	gitIn(t, dir, "bot@example.org", "commit", "-q", "-a", "-m", "Revise", "-m", "Model: claude-sonnet-5")
	write(".tableaux/tasks/aaaa.yaml", "title: Root, revised again\nassignee: olive@example.org\njunctions:\n  defined: { contributor: bot@example.org, model: claude-fable }\n")
	gitIn(t, dir, "bot@example.org", "commit", "-q", "-a", "-m", "Revise again", "-m", "Model: claude-fable-5-1")
	findings, err := Audit(Repo(dir), "main")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, f := range findings {
		if f.Rule == "H3" {
			n++
			if f.Task != "aaaa" || f.Gate != "defined" {
				t.Errorf("H3 at %s %s, want aaaa defined", f.Task, f.Gate)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d H3 findings, want 1 (the prefix match passes)", n)
	}
}
