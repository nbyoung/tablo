package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/nbyoung/tablo"
)

// golden reads a file of testdata.
func golden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// streams checks the two streams of a result against its exit code: the
// result on standard output and nothing on standard error for 0 and 1; the
// reverse for 2 and 3, with the hint to --help on 2 alone.
func streams(t *testing.T, name string, r result) {
	t.Helper()
	switch r.code {
	case 0, 1:
		if r.stderr != "" {
			t.Errorf("%s: exit %d writes to standard error: %s", name, r.code, r.stderr)
		}
	case 2, 3:
		if r.stdout != "" {
			t.Errorf("%s: exit %d writes to standard output: %s", name, r.code, r.stdout)
		}
		if !strings.HasPrefix(r.stderr, "tablo: ") || strings.Contains(r.stderr, "Run 'tablo --help' for usage.\n") != (r.code == 2) {
			t.Errorf("%s: exit %d writes %q to standard error", name, r.code, r.stderr)
		}
	default:
		t.Errorf("%s: exit %d", name, r.code)
	}
}

// withoutMessages drops the message of every diagnostic of an envelope.
func withoutMessages(t *testing.T, doc string) any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(doc), &m); err != nil {
		t.Fatalf("%v\n%s", err, doc)
	}
	for _, d := range m["diagnostics"].([]any) {
		delete(d.(map[string]any), "message")
	}
	return m
}

var messagePart = regexp.MustCompile(`(?m)^((?:\S+: )?(?:error|warning|information): \S+) .*$`)

// TestExitCodes is T7 over the corpus entries the design names: the exit
// codes and the two streams, with validate.json and validate.txt as goldens
// all but the message.
func TestExitCodes(t *testing.T) {
	quiet(t)
	r := run(t, "-C", entry(t, "weather-station"), "--ref", "main", "validate")
	streams(t, "weather-station", r)
	if r.code != 0 || !strings.Contains(r.stdout, "warning: R9 ") {
		t.Errorf("weather-station: exit %d, %q", r.code, r.stdout)
	}
	r = run(t, "-C", entry(t, "unknown-trailer"), "--ref", "main", "validate")
	streams(t, "unknown-trailer", r)
	if r.code != 0 || strings.Count(r.stdout, "warning: H1 ") != 2 {
		t.Errorf("unknown-trailer: exit %d, %q", r.code, r.stdout)
	}
	bad := entry(t, "task-bad-email")
	r = run(t, "-C", bad, "--ref", "main", "validate")
	streams(t, "task-bad-email", r)
	if r.code != 1 || messagePart.ReplaceAllString(r.stdout, "$1") != messagePart.ReplaceAllString(golden(t, "validate.txt"), "$1") {
		t.Errorf("task-bad-email: exit %d, %q", r.code, r.stdout)
	}
	r = run(t, "-C", bad, "--ref", "main", "validate", "--json")
	if got, want := withoutMessages(t, r.stdout), withoutMessages(t, golden(t, "validate.json")); r.code != 1 || !equalJSON(got, want) {
		t.Errorf("task-bad-email --json: exit %d\n%s", r.code, r.stdout)
	}
	r = run(t, "-C", entry(t, "no-tableaux-directory"), "--ref", "main", "validate")
	streams(t, "no-tableaux-directory", r)
	if r.code != 1 || !strings.HasPrefix(r.stdout, "error: P1 ") {
		t.Errorf("no-tableaux-directory: exit %d, %q", r.code, r.stdout)
	}
	station := entry(t, "weather-station")
	for name, args := range map[string][]string{
		"no repository":    {"-C", t.TempDir(), "validate"},
		"an unknown ref":   {"-C", station, "--ref", "nowhere", "validate"},
		"an unknown task":  {"-C", station, "--ref", "main", "task", "abcd"},
		"a tree to a view": {"-C", station, "--ref", "main^{tree}", "tableau"},
	} {
		r := run(t, args...)
		streams(t, name, r)
		if r.code != 3 {
			t.Errorf("%s: exit %d, %q", name, r.code, r.stderr)
		}
	}
}

func equalJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

// TestExitCodesBuilt is T7 over repositories the test builds, so that it
// runs without the corpus too: each exit code, and each usage error 2.
func TestExitCodesBuilt(t *testing.T) {
	quiet(t)
	b := small(t)
	for _, tc := range []struct {
		name string
		args []string
		code int
	}{
		{"a valid project", []string{"-C", b.dir, "validate"}, 0},
		{"a view", []string{"-C", b.dir, "tableau"}, 0},
		{"an audit on today's date", []string{"-C", b.dir, "audit", "--viewer", "olive@example.org"}, 0},
		{"a directory below the project", []string{"-C", filepath.Join(b.dir, ".tableaux", "tasks"), "validate"}, 0},
		{"no repository", []string{"-C", t.TempDir(), "validate"}, 3},
		{"a directory that is not there", []string{"-C", filepath.Join(b.dir, "nowhere"), "validate"}, 3},
		{"an unknown ref", []string{"-C", b.dir, "--ref", "nowhere", "validate"}, 3},
		{"an unknown task", []string{"-C", b.dir, "task", "abcd"}, 3},
		{"an unknown gate", []string{"-C", b.dir, "tableau", "--columns", "mockup"}, 3},
		{"a tree to a view", []string{"-C", b.dir, "--ref", "HEAD^{tree}", "gates"}, 3},
		{"a tree to a trailer", []string{"-C", b.dir, "--ref", "HEAD^{tree}", "trailer", "authorised", "b2c9"}, 3},
		{"a tree to validate", []string{"-C", b.dir, "--ref", "HEAD^{tree}", "validate"}, 0},
		{"an unknown command", []string{"status"}, 2},
		{"no command", []string{}, 2},
		{"an option the view does not take", []string{"-C", b.dir, "gates", "--person", "a@b.c"}, 2},
		{"a queue with no person and no viewer", []string{"-C", b.dir, "queue"}, 2},
		{"a context with no task, person or viewer", []string{"-C", b.dir, "context"}, 2},
		{"a malformed date", []string{"-C", b.dir, "audit", "--now", "2026-13-01"}, 2},
		{"a malformed id", []string{"-C", b.dir, "trailer", "authorised", "B2C9"}, 2},
	} {
		r := run(t, tc.args...)
		streams(t, tc.name, r)
		if r.code != tc.code {
			t.Errorf("%s: exit %d, want %d: %s%s", tc.name, r.code, tc.code, r.stdout, r.stderr)
		}
	}
	none := build(t)
	none.put(map[string]string{"README.md": "# Nothing\n"})
	none.commit("olive", "2026-09-01", "Start")
	r := run(t, "-C", none.dir, "validate", "--json")
	streams(t, "no .tableaux", r)
	if r.code != 1 || !strings.Contains(r.stdout, `"code": "P1"`) || !strings.Contains(r.stdout, `"project": null`) {
		t.Errorf("no .tableaux: exit %d, %s", r.code, r.stdout)
	}
	b.put(map[string]string{".tableaux/tasks/b2c9.yaml": strings.Replace(baseLeaf, "pat@example.org", "not-an-email", 1)})
	r = run(t, "-C", b.dir, "tableau")
	streams(t, "an error in the working tree", r)
	if r.code != 1 || !strings.Contains(r.stdout, `"code": "T4"`) {
		t.Errorf("an error in the working tree: exit %d, %s", r.code, r.stdout)
	}
	r = run(t, "--help")
	if r.code != 0 || r.stdout != usage || r.stderr != "" {
		t.Errorf("--help: exit %d, %q", r.code, r.stderr)
	}
}

// TestSources is T8: the four sources in a built repository.
func TestSources(t *testing.T) {
	quiet(t)
	b := small(t)
	b.put(map[string]string{".tableaux/tasks/c3d7.yaml": strings.Replace(baseSecond, "Second leaf", "Edited leaf", 1)})
	disk := run(t, "-C", b.dir, "task", "c3d7")
	head := run(t, "-C", b.dir, "--ref", "HEAD", "task", "c3d7")
	if !strings.Contains(disk.stdout, "Edited leaf") || strings.Contains(head.stdout, "Edited leaf") {
		t.Errorf("an edit shows %v without --ref and %v with --ref HEAD",
			strings.Contains(disk.stdout, "Edited leaf"), strings.Contains(head.stdout, "Edited leaf"))
	}
	if !strings.Contains(disk.stdout, `"worktree": true`) || !strings.Contains(disk.stdout, `"ref": "main"`) ||
		!strings.Contains(head.stdout, `"worktree": false`) || !strings.Contains(head.stdout, `"ref": "HEAD"`) {
		t.Errorf("the sources read wrong:\n%s\n%s", disk.stdout[:300], head.stdout[:300])
	}
	b.git("olive", "2026-09-04", "checkout", "-q", ".")

	// A tree from git write-tree, as a pre-commit hook gives it: the index
	// and not the working tree, and the rules that need no history. As
	// tabloio 1422's T11 has it, good and bad staged, each with the opposite
	// in the working tree: the four pairs of the index and the disk.
	good := baseSecond
	bad := strings.Replace(baseSecond, "ben@example.org", "ben", 1)
	const second = ".tableaux/tasks/c3d7.yaml"
	for _, tc := range []struct {
		name         string
		staged, disk string
		code         int // of the staged tree
	}{
		{"good staged, good on disk", good, good, 0},
		{"good staged, bad on disk", good, bad, 0},
		{"bad staged, good on disk again", bad, good, 1},
		{"bad staged, bad on disk", bad, bad, 1},
	} {
		b.put(map[string]string{second: tc.staged})
		b.git("olive", "2026-09-04", "add", "-A")
		b.put(map[string]string{second: tc.disk})
		tree := b.git("olive", "2026-09-04", "write-tree")
		r := run(t, "-C", b.dir, "--ref", tree, "validate", "--json")
		streams(t, tc.name, r)
		if r.code != tc.code || !strings.Contains(r.stdout, `"commit": null`) || strings.Contains(r.stdout, `"code": "T4"`) != (tc.code == 1) {
			t.Errorf("%s: the staged tree exits %d\n%s", tc.name, r.code, r.stdout)
		}
		diskCode := 0
		if tc.disk == bad {
			diskCode = 1
		}
		if r := run(t, "-C", b.dir, "validate"); r.code != diskCode {
			t.Errorf("%s: the working tree exits %d, want %d", tc.name, r.code, diskCode)
		}
		b.git("olive", "2026-09-04", "reset", "-q", "--hard")
	}
	// version.yaml removed from the index alone, the file still on disk.
	b.git("olive", "2026-09-04", "rm", "-q", "--cached", ".tableaux/version.yaml")
	if _, err := os.Stat(filepath.Join(b.dir, ".tableaux", "version.yaml")); err != nil {
		t.Fatal(err)
	}
	if r := run(t, "-C", b.dir, "--ref", b.git("olive", "2026-09-04", "write-tree"), "validate", "--json"); r.code != 1 || !strings.Contains(r.stdout, `"code": "P2"`) {
		t.Errorf("a version file removed from the index: exit %d\n%s", r.code, r.stdout)
	}
	if r := run(t, "-C", b.dir, "validate"); r.code != 0 {
		t.Errorf("the working tree with its version file: exit %d %s", r.code, r.stdout)
	}
	b.git("olive", "2026-09-04", "reset", "-q", "--hard")

	// A range on history.
	first := b.git("olive", "2026-09-04", "rev-list", "--max-parents=0", "HEAD")
	r := run(t, "-C", b.dir, "--ref", first+"..", "history")
	if r.code != 0 || !strings.Contains(r.stdout, `"from": {`) || !strings.Contains(r.stdout, `"commit": "`+first+`"`) {
		t.Errorf("a range: exit %d\n%s", r.code, r.stdout)
	}

	// A bare clone reads at a ref and has no working tree.
	bare := filepath.Join(t.TempDir(), "bare.git")
	b.git("olive", "2026-09-04", "clone", "-q", "--bare", b.dir, bare)
	if r := run(t, "-C", bare, "--ref", "main", "validate"); r.code != 0 {
		t.Errorf("a bare clone at main: exit %d %s", r.code, r.stderr)
	}
	if r := run(t, "-C", bare, "validate"); r.code != 3 {
		t.Errorf("a bare clone without --ref: exit %d", r.code)
	}
}

// TestSourceBelowTheRoot is T8's -C lib in subproject-same-repository.
func TestSourceBelowTheRoot(t *testing.T) {
	quiet(t)
	r := run(t, "-C", filepath.Join(entry(t, "subproject-same-repository"), "lib"), "--ref", "main", "validate", "--json")
	if r.code != 0 || !strings.Contains(r.stdout, `"root": "5a00"`) || !strings.Contains(r.stdout, `"dir": "lib",`) {
		t.Errorf("-C lib: exit %d\n%s", r.code, r.stdout)
	}
	r = run(t, "-C", entry(t, "subproject-same-repository"), "--ref", "main", "validate", "--json")
	if r.code != 0 || !strings.Contains(r.stdout, `"root": "e4a1"`) || !strings.Contains(r.stdout, `"dir": ".",`) {
		t.Errorf("-C the root: exit %d\n%s", r.code, r.stdout)
	}
}

// commands are the command lines T9 and T10 run, each over -C and the
// viewer and date they name.
var commands = [][]string{
	{"validate", "--json"}, {"validate"},
	{"gates", "--task", "b2c9"}, {"task", "b2c9"}, {"authority"}, {"assignment"},
	{"queue"}, {"queue", "--person", "bot@example.org", "--brief", "b2c9", "design"}, {"blockage"},
	{"tableau"}, {"tableau", "--columns", "design,release", "--historical"}, {"context"}, {"context", "--task", "b2c9"},
	{"history"}, {"history", "--task", "b2c9", "--level", "provenance"},
	{"audit", "--now", "2026-09-28"}, {"audit", "--now", "2026-09-28", "--minimum", "information", "--yaml"},
	{"trailer", "reviewed", "b2c9", "design", "--json"}, {"trailer", "authorised", "c3d7"},
	{"version", "--json"},
}

// TestDeterminism is T9: each command runs in two clones at two paths,
// under two TZ and two LC_ALL values and two Git configurations, with
// --viewer and --now: the bytes are equal and hold neither path.
func TestDeterminism(t *testing.T) {
	quiet(t)
	b := small(t)
	configs := []string{"", "[log]\n\tdate = iso\n\tshowSignature = true\n[color]\n\tui = always\n[core]\n\tquotePath = false\n[diff]\n\trenames = copies\n"}
	var outs [2][]string
	var dirs [2]string
	for i, env := range []struct{ tz, lc string }{{"UTC", "C"}, {"Pacific/Kiritimati", "C.UTF-8"}} {
		dirs[i] = filepath.Join(t.TempDir(), "clone"+string(rune('a'+i)))
		b.git("olive", "2026-09-04", "clone", "-q", b.dir, dirs[i])
		config := filepath.Join(t.TempDir(), "gitconfig")
		if err := os.WriteFile(config, []byte(configs[i]), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TZ", env.tz)
		t.Setenv("LC_ALL", env.lc)
		t.Setenv("GIT_CONFIG_GLOBAL", config)
		for _, c := range commands {
			r := run(t, append([]string{"-C", dirs[i], "--viewer", "pat@example.org"}, c...)...)
			if r.code > 1 {
				t.Fatalf("%v: exit %d %s", c, r.code, r.stderr)
			}
			outs[i] = append(outs[i], r.stdout)
		}
	}
	for j, c := range commands {
		if outs[0][j] != outs[1][j] {
			t.Errorf("%v differs between the clones:\n%s\n%s", c, outs[0][j], outs[1][j])
		}
		for _, d := range dirs {
			if strings.Contains(outs[0][j], d) || strings.Contains(outs[1][j], filepath.Base(d)) {
				t.Errorf("%v holds a path", c)
			}
		}
	}
}

// TestLibraryAndCommandAgree is T10: for each command, the envelope the
// library returns encodes as the command's standard output.
func TestLibraryAndCommandAgree(t *testing.T) {
	quiet(t)
	b := small(t)
	ctx := context.Background()
	repo, err := tablo.Open(ctx, b.dir, tablo.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := repo.Load(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range commands {
		args := append([]string{"-C", b.dir, "--viewer", "pat@example.org"}, c...)
		inv, err := parse(args)
		if err != nil {
			t.Fatal(err)
		}
		var e *tablo.Envelope
		switch inv.command {
		case "validate":
			e = s.Validate()
		case "trailer":
			if err := s.CheckTrailer(inv.trailer); err != nil {
				t.Fatal(err)
			}
			if e, err = trailerEnvelope(inv.trailer, s); err != nil {
				t.Fatal(err)
			}
		case "version":
			e = versionEnvelope()
		default:
			p := inv.params
			p.Viewer = inv.viewer
			if e, err = s.View(ctx, inv.view, p); err != nil {
				t.Fatalf("%v: %v", c, err)
			}
		}
		want := e.JSON()
		if inv.format == "yaml" {
			want = e.YAML()
		}
		r := run(t, args...)
		if inv.format == "" && (inv.view == "" || inv.command == "trailer") {
			continue // a plain form, which TestPlainForms covers
		}
		if r.stdout != string(want) {
			t.Errorf("%v: the command writes\n%s\nthe library\n%s", c, r.stdout, want)
		}
		if r.code != e.ExitCode() {
			t.Errorf("%v: exit %d, the library %d", c, r.code, e.ExitCode())
		}
	}
}

// TestPlainForms covers the three plain forms: a diagnostic per line
// without the parts it lacks, the trailer's line and the version's (T13).
func TestPlainForms(t *testing.T) {
	quiet(t)
	got := string(plainDiagnostics([]tablo.Diagnostic{
		{Severity: "error", Code: "T4", Path: ".tableaux/tasks/b2c9.yaml", Line: 4, Col: 11, Message: "m1"},
		{Severity: "warning", Code: "L4", Path: ".tableaux/notes.txt", Message: "m2"},
		{Severity: "warning", Code: "X", Path: ".tableaux/gates.yaml", Line: 3, Message: "m3"},
		{Severity: "information", Code: "H4", Message: "m4"},
	}))
	want := ".tableaux/tasks/b2c9.yaml:4:11: error: T4 m1\n.tableaux/notes.txt: warning: L4 m2\n.tableaux/gates.yaml:3: warning: X m3\ninformation: H4 m4\n"
	if got != want {
		t.Errorf("the plain diagnostics read\n%s", got)
	}
	b := small(t)
	if r := run(t, "-C", b.dir, "validate"); r.code != 0 || r.stdout != ".tableaux/status/b2c9.yaml: information: H4 the newest event is the contributor's and the status states no review; pat@example.org may have design to review\n" {
		t.Errorf("validate writes %q, exit %d", r.stdout, r.code)
	}
	r := run(t, "version")
	if r.code != 0 || r.stdout != "tablo "+tablo.Version+" (tableaux 0.4, schema "+tablo.SchemaVersion+")\n" {
		t.Errorf("version writes %q", r.stdout)
	}
	if r := run(t, "version", "--json"); r.stdout != golden(t, "version.json") {
		t.Errorf("version --json differs from version.json:\n%s", r.stdout)
	}
	if r := run(t, "version", "--yaml"); !strings.Contains(r.stdout, "schema: \"tablo/1\"\n") {
		t.Errorf("version --yaml writes\n%s", r.stdout)
	}
}

// TestTrailerLines is T11: the four lines, with trailer.json as golden on
// the weather station; git interpret-trailers reads each back; a malformed
// id exits 2; an unknown task, an unknown gate and an exempt gate exit 3;
// --no-check reads no repository.
func TestTrailerLines(t *testing.T) {
	quiet(t)
	b := small(t)
	nowhere := filepath.Join(t.TempDir(), "nowhere")
	for _, tc := range []struct {
		args []string
		code int
		line string
	}{
		{[]string{"-C", b.dir, "trailer", "authorised", "c3d7"}, 0, "Authorised: c3d7"},
		{[]string{"-C", b.dir, "trailer", "reviewed", "b2c9", "design"}, 0, "Reviewed: b2c9 design"},
		{[]string{"-C", b.dir, "trailer", "reaffirmed", "b2c9"}, 0, "Reaffirmed: b2c9"},
		{[]string{"-C", nowhere, "trailer", "model", "claude-fable-5-1"}, 0, "Model: claude-fable-5-1"},
		{[]string{"-C", nowhere, "trailer", "reviewed", "abcd", "nowhere", "--no-check"}, 0, "Reviewed: abcd nowhere"},
		{[]string{"-C", b.dir, "trailer", "authorised", "B2C9"}, 2, ""},
		{[]string{"-C", b.dir, "trailer", "reviewed", "b2c9", "Design"}, 2, ""},
		{[]string{"-C", b.dir, "trailer", "authorised", "abcd"}, 3, ""},
		{[]string{"-C", b.dir, "trailer", "reviewed", "b2c9", "mockup"}, 3, ""},
		{[]string{"-C", b.dir, "trailer", "reviewed", "c3d7", "release"}, 3, ""},
		{[]string{"-C", b.dir, "trailer", "reviewed", "b2c9", "undefined"}, 3, ""},
		{[]string{"-C", nowhere, "trailer", "authorised", "b2c9"}, 3, ""},
	} {
		r := run(t, tc.args...)
		streams(t, strings.Join(tc.args[2:], " "), r)
		if r.code != tc.code || tc.line != "" && r.stdout != tc.line+"\n" {
			t.Errorf("%v: exit %d, %q", tc.args[2:], r.code, r.stdout)
			continue
		}
		if tc.line == "" {
			continue
		}
		cmd := exec.Command("git", "interpret-trailers", "--parse")
		cmd.Stdin = strings.NewReader("Subject\n\n" + r.stdout)
		out, err := cmd.Output()
		if err != nil || string(out) != r.stdout {
			t.Errorf("git reads %q back as %q, %v", r.stdout, out, err)
		}
	}
	r := run(t, "-C", entry(t, "weather-station"), "--ref", "main", "trailer", "reviewed", "9f31", "validate", "--json")
	if r.code != 0 || r.stdout != golden(t, "trailer.json") {
		t.Errorf("the weather station's trailer differs from trailer.json:\n%s", r.stdout)
	}
}

// TestTrunkAndReplace is T12 on the corpus: --trunk and --replace reach the
// derivation, and the URL resolves with no network.
func TestTrunkAndReplace(t *testing.T) {
	quiet(t)
	undetermined := entry(t, "trunk-undetermined")
	if r := run(t, "-C", undetermined, "--ref", "main", "validate"); !strings.Contains(r.stdout, "warning: P5 ") {
		t.Errorf("trunk-undetermined without --trunk: %q", r.stdout)
	}
	if r := run(t, "-C", undetermined, "--ref", "main", "--trunk", "main", "validate", "--json"); strings.Contains(r.stdout, `"P5"`) ||
		!strings.Contains(r.stdout, `"resolved": "caller"`) {
		t.Errorf("trunk-undetermined with --trunk main:\n%s", r.stdout)
	}
	byURL := entry(t, "subproject-by-url")
	if r := run(t, "-C", byURL, "--ref", "main", "validate"); r.code != 1 || !strings.Contains(r.stdout, "error: J8 ") {
		t.Errorf("subproject-by-url without --replace: exit %d %q", r.code, r.stdout)
	}
	lib := byURL + ".lib"
	if r := run(t, "-C", byURL, "--ref", "main", "--replace", "https://example.org/lib.git="+lib, "validate"); r.code != 0 || r.stdout != "" {
		t.Errorf("subproject-by-url with --replace: exit %d %q", r.code, r.stdout)
	}
}

// TestTrunkAndReplaceBuilt is T12 over repositories the test builds.
func TestTrunkAndReplaceBuilt(t *testing.T) {
	quiet(t)
	lib := build(t)
	lib.put(map[string]string{
		".tableaux/version.yaml": baseVersion, ".tableaux/gates.yaml": baseGates,
		".tableaux/tasks/5a00.yaml": strings.Replace(baseRoot, "olive", "ben", 1),
	})
	commit := lib.commit("ben", "2026-09-01", "Plan the library")
	b := small(t)
	b.put(map[string]string{
		".tableaux/version.yaml":    "tableaux: 0.3.1\n",
		".tableaux/tasks/c3d7.yaml": baseSecond + "requires:\n  - { subproject: { url: \"https://example.org/lib.git\", id: \"5a00\", commit: \"" + commit + "\" } }\n",
	})
	b.commit("olive", "2026-09-04", "Require the library")
	if r := run(t, "-C", b.dir, "validate"); r.code != 1 || !strings.Contains(r.stdout, "warning: P5 ") || !strings.Contains(r.stdout, "error: J8 ") {
		t.Errorf("without --trunk and --replace: exit %d %q", r.code, r.stdout)
	}
	r := run(t, "-C", b.dir, "--trunk", "main", "--replace", "https://example.org/lib.git="+lib.dir, "validate")
	if strings.Contains(r.stdout, "P5") || strings.Contains(r.stdout, "J8") || r.code != 0 {
		t.Errorf("with --trunk and --replace: exit %d %q", r.code, r.stdout)
	}
}

// TestViewerDefault is T4's viewer: --viewer, else GIT_AUTHOR_EMAIL, else
// git config user.email in -C, else nobody; and --now's default, today.
func TestViewerDefault(t *testing.T) {
	quiet(t)
	b := small(t)
	viewer := func(args ...string) string {
		t.Helper()
		r := run(t, append([]string{"-C", b.dir, "gates"}, args...)...)
		var e struct {
			Viewer struct {
				Email *string `json:"email"`
			} `json:"viewer"`
		}
		if err := json.Unmarshal([]byte(r.stdout), &e); err != nil {
			t.Fatalf("%v: %s", err, r.stderr)
		}
		if e.Viewer.Email == nil {
			return ""
		}
		return *e.Viewer.Email
	}
	if got := viewer(); got != "" {
		t.Errorf("with nothing set the viewer is %q", got)
	}
	b.git("olive", "2026-09-04", "config", "user.email", "ben@example.org")
	if got := viewer(); got != "ben@example.org" {
		t.Errorf("from the configuration the viewer is %q", got)
	}
	t.Setenv("GIT_AUTHOR_EMAIL", "pat@example.org")
	if got := viewer(); got != "pat@example.org" {
		t.Errorf("from the variable the viewer is %q", got)
	}
	if got := viewer("--viewer", "dan@example.org"); got != "dan@example.org" {
		t.Errorf("from the option the viewer is %q", got)
	}
	t.Setenv("GIT_AUTHOR_EMAIL", "not an email")
	if got := viewer(); got != "" {
		t.Errorf("a variable that is no email gives the viewer %q", got)
	}
	r := run(t, "-C", b.dir, "audit", "--viewer", "olive@example.org")
	if !strings.Contains(r.stdout, `"now": "2026-09-28"`) {
		t.Errorf("--now does not default to today:\n%s", r.stdout)
	}
	if r := run(t, "-C", b.dir, "queue"); r.code != 2 {
		t.Errorf("a queue with a viewer that is no email: exit %d", r.code)
	}
}
