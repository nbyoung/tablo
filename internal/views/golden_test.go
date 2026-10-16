package views

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
)

// golden is one file of testdata/weather-station: a view of the weather
// station at main, named <command>[-<focus>].<level>.json.
type golden struct {
	file    string
	command string
	query   Query
}

// The five golden files of design 493e, the six of design 886d and the three
// of design 8ed1.
var goldens = []golden{
	{"tableau.detail.json", "tableau", Query{Level: Detail, Window: 1}},
	{"context-ben.detail.json", "context", Query{Person: "ben@example.org", Level: Detail, Window: 1}},
	{"context-4e2b.glance.json", "context", Query{Task: "4e2b", Level: Glance, Window: 1}},
	{"queue-ada.glance.json", "queue", Query{Task: "a1c0", Person: "ada@example.org", Level: Glance}},
	{"queue-dan.glance.json", "queue", Query{Task: "a1c0", Person: "dan@example.org", Level: Glance}},
	{"blockage.detail.json", "blockage", Query{Task: "a1c0", Level: Detail}},
	{"gates-9f31.provenance.json", "gates", Query{Task: "9f31", Level: Provenance}},
	{"task-c07d.provenance.json", "task", Query{Task: "c07d", Person: "ben@example.org", Level: Provenance}},
	{"task-3c5d.detail.json", "task", Query{Task: "3c5d", Level: Detail}},
	{"authority.provenance.json", "authority", Query{Task: "a1c0", Level: Provenance}},
	{"assignment.glance.json", "assignment", Query{Task: "a1c0", Level: Glance}},
	{"history.glance.json", "history", Query{Level: Glance}},
	{"history-4e2b.detail.json", "history", Query{Task: "4e2b", Level: Detail}},
	{"history-c07d.provenance.json", "history", Query{Task: "c07d", Level: Provenance}},
}

// T2: the golden data, byte for byte, from weather-station at main.
func TestGolden(t *testing.T) {
	f := derived(t, load.Options{}, builtAt(t, "weather-station"), "main", "")
	for _, g := range goldens {
		want, err := os.ReadFile(filepath.Join("testdata", "weather-station", g.file))
		if err != nil {
			t.Fatal(err)
		}
		got := encoded(t, viewOf(f, g.command, g.query))
		if !bytes.Equal(got, want) {
			out := filepath.Join(t.TempDir(), g.file)
			if err := os.WriteFile(out, got, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Errorf("%s: the view differs from the golden file; the view is in %s", g.file, out)
		}
	}
}

// T3: each level differs. A golden file validates at its own level and
// fails under each other level. A tableau holds the same keys at glance and
// at detail, where the rows alone differ, so the schema tells neither from
// the other.
func TestSchemaTellsTheLevels(t *testing.T) {
	s := envelopeSchema(t)
	for _, g := range goldens {
		data, err := os.ReadFile(filepath.Join("testdata", "weather-station", g.file))
		if err != nil {
			t.Fatal(err)
		}
		for level, name := range levels {
			q := g.query
			q.Level = level
			err := s.Validate(envelope(t, g.command, q, data))
			switch {
			case level == g.query.Level && err != nil:
				t.Errorf("%s does not validate at its own level: %v", g.file, err)
			case (g.command == "tableau" || g.command == "context") && level < Provenance:
				if err != nil {
					t.Errorf("%s does not validate at %s: %v", g.file, name, err)
				}
			case level != g.query.Level && err == nil:
				t.Errorf("%s validates at %s", g.file, name)
			}
		}
	}
}
