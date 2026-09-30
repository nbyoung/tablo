package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const corpus = "/home/nbyoung/Projects/Tableaux/tableaux/corpus/build/weather-station"

func invoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func needCorpus(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(corpus); err != nil {
		t.Skip("corpus absent")
	}
}

func TestTrailerLines(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"trailer", "authorised", "9f31"}, "Authorised: 9f31"},
		{[]string{"trailer", "reviewed", "9f31", "design"}, "Reviewed: 9f31 design"},
		{[]string{"trailer", "reaffirmed", "c07d"}, "Reaffirmed: c07d"},
	} {
		code, out, _ := invoke(t, c.args...)
		if code != 0 || out != c.want+"\n" {
			t.Errorf("%v: code %d, out %q", c.args, code, out)
		}
	}
}

func TestTrailerErrors(t *testing.T) {
	for _, args := range [][]string{
		{"trailer", "authorised", "9F31"},
		{"trailer", "authorised", "9f3"},
		{"trailer", "reviewed", "9f31"},
		{"trailer", "reaffirmed", "9f31", "design"},
		{"trailer", "noted", "9f31"},
		{"nonsense"},
		{},
	} {
		if code, _, _ := invoke(t, args...); code != 2 {
			t.Errorf("%v: code %d, want 2", args, code)
		}
	}
}

func TestTrailerGateChecked(t *testing.T) {
	needCorpus(t)
	if code, out, _ := invoke(t, "-C", corpus, "trailer", "reviewed", "9f31", "mockup"); code != 0 || out != "Reviewed: 9f31 mockup\n" {
		t.Errorf("code %d, out %q", code, out)
	}
	if code, _, _ := invoke(t, "-C", corpus, "trailer", "reviewed", "9f31", "bogus"); code != 3 {
		t.Errorf("unknown gate: code %d, want 3", code)
	}
}

func TestViewTasksEnvelope(t *testing.T) {
	needCorpus(t)
	code, out, _ := invoke(t, "-C", corpus, "--ref", "main", "view", "tasks")
	if code != 0 {
		t.Fatalf("code %d", code)
	}
	var env struct {
		Schema      string
		Command     string
		Ref         struct{ Name, Commit string }
		Diagnostics []any
		Data        struct {
			Tasks []struct{ ID, Title, Parent string }
		}
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	if env.Schema != Schema || env.Command != "view" || len(env.Ref.Commit) != 40 || env.Diagnostics == nil {
		t.Errorf("envelope %+v", env)
	}
	got := map[string]string{}
	for _, tk := range env.Data.Tasks {
		got[tk.ID] = tk.Title
	}
	if got["9f31"] != "Sensor board" || got["a1c0"] != "Weather station" || len(got) != 6 {
		t.Errorf("tasks %v", got)
	}
}

func TestRefSelectsContent(t *testing.T) {
	needCorpus(t)
	// W1 (c4a7fcb) precedes the sensor board task.
	_, out, _ := invoke(t, "-C", corpus, "--ref", "c4a7fcbeb3d4a27d280c438eeec5ee2e96fb86b9", "view", "tasks")
	if strings.Contains(out, "Sensor board") {
		t.Error("W1 lists the sensor board")
	}
}

func TestYAMLAndHistory(t *testing.T) {
	needCorpus(t)
	code, out, _ := invoke(t, "-C", corpus, "--yaml", "history")
	if code != 0 || !strings.HasPrefix(out, "command: \"history\"\n") || !strings.Contains(out, "kind: \"reaffirmed\"") {
		t.Errorf("code %d\n%s", code, out)
	}
}

func TestValidateAndStatus(t *testing.T) {
	needCorpus(t)
	if code, _, _ := invoke(t, "-C", corpus, "validate"); code != 0 {
		t.Errorf("validate code %d", code)
	}
	_, out, _ := invoke(t, "-C", corpus, "status")
	if !strings.Contains(out, `"reason": "blocked"`) {
		t.Errorf("status lacks the blocked reason:\n%s", out)
	}
}
