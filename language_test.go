package tablo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// crossing builds the repository of design cdb0's crossing.sh under a
// temporary directory and returns its path, the commits it labels and the
// labels in order. It skips the test when the host has no shell or no git.
func crossing(t *testing.T) (repo string, hashes map[string]string, order []string) {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("the host has no sh executable")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("the host has no git executable")
	}
	script, err := filepath.Abs(filepath.Join("internal", "derive", "testdata", "crossing.sh"))
	if err != nil {
		t.Fatal(err)
	}
	repo = filepath.Join(t.TempDir(), "crossing")
	cmd := exec.Command(sh, script, repo)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("crossing.sh: %v\n%s", err, out)
	}
	data, err := os.ReadFile(repo + ".labels.txt")
	if err != nil {
		t.Fatal(err)
	}
	hashes = map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		label, hash, _ := strings.Cut(line, " ")
		hashes[label] = hash
		order = append(order, label)
	}
	return repo, hashes, order
}

// TestCrossingValidates is T10, the Validator's half: tablo.Validate at each
// label of crossing.sh reports what crossing.expected.txt states under
// validate, in the form tablo validate prints. H7 stays silent at X3, whose
// language is 0.3.2, and fires at X5; S11 at X5 lifts once olive reviews at X6.
func TestCrossingValidates(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("internal", "derive", "testdata", "crossing.expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{}
	label, validating := "", false
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "#"):
		case !strings.HasPrefix(line, " "):
			label, _, _ = strings.Cut(line, ":")
			validating = false
		case line == "  validate: nothing":
			want[label] = []string{}
		case line == "  validate:":
			want[label], validating = []string{}, true
		case validating && strings.HasPrefix(line, "    "):
			want[label] = append(want[label], strings.TrimPrefix(line, "    "))
		default:
			validating = false
		}
	}
	repo, hashes, order := crossing(t)
	for _, name := range order {
		var got []string
		for _, d := range loadAt(t, repo, hashes[name], Options{}).Validate().Diagnostics {
			line := ""
			if d.Path != "" {
				line = d.Path + ":"
				if d.Line > 0 {
					line += strconv.Itoa(d.Line) + ":"
					if d.Col > 0 {
						line += strconv.Itoa(d.Col) + ":"
					}
				}
				line += " "
			}
			got = append(got, line+d.Severity+": "+d.Code+" "+d.Message)
		}
		if strings.Join(got, "\n") != strings.Join(want[name], "\n") {
			t.Errorf("%s:\n got  %s\n want %s", name, strings.Join(got, "\n      "), strings.Join(want[name], "\n      "))
		}
	}
	if len(want) != 6 || len(order) != 6 {
		t.Errorf("crossing.expected.txt states %d labels and crossing.sh makes %d; want 6", len(want), len(order))
	}
}
