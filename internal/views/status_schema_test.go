package views

import (
	"bytes"
	"testing"
)

// T3: each level differs. Each status view of the weather station validates
// at its own level and fails under each other level; a tableau's glance and
// detail hold the same keys, and the schema tells provenance from the two.
func TestSchemaTellsTheStatusLevels(t *testing.T) {
	s := envelopeSchema(t)
	f, _ := station(t, "main")
	for command, q := range map[string]Query{
		"tableau":  {Person: "ada@example.org", Window: 1},
		"context":  {Task: "4e2b", Window: 1},
		"queue":    {Task: "a1c0", Person: "ada@example.org"},
		"blockage": {Task: "a1c0"},
	} {
		for level := range levels {
			q.Level = level
			data := encoded(t, viewOf(f, command, q))
			for under, name := range levels {
				as := q
				as.Level = under
				err := s.Validate(envelope(t, command, as, data))
				same := under == level || (command == "tableau" || command == "context") && under < Provenance && level < Provenance
				switch {
				case same && err != nil:
					t.Errorf("%s at %s does not validate at %s: %v", command, levels[level], name, err)
				case !same && err == nil:
					t.Errorf("%s at %s validates at %s", command, levels[level], name)
				}
			}
		}
	}
	// A brief stands at provenance alone.
	q := Query{Task: "a1c0", Person: "ada@example.org", Level: Provenance, Brief: &BriefOf{Task: "9f31", Gate: "performance"}}
	brief := encoded(t, Queue(f, q))
	if !bytes.Contains(brief, []byte(`"follows": "plain"`)) {
		t.Fatalf("the queue holds no brief: %s", brief)
	}
	for under, name := range levels {
		as := q
		as.Level = under
		if err := s.Validate(envelope(t, "queue", as, brief)); (err == nil) != (under == Provenance) {
			t.Errorf("a brief at %s: %v", name, err)
		}
	}
}

// T3: the schema types the closed sets of words of the status views as
// enums (decision 6, and decision 11 for what a snapshot advances): a word
// outside a set fails.
func TestSchemaClosesTheStatusWords(t *testing.T) {
	s := envelopeSchema(t)
	f, _ := station(t, "main")
	for _, c := range []struct {
		command string
		q       Query
		changes [][2]string
	}{
		{"context", Query{Task: "9f31", Level: Detail, Window: 1}, [][2]string{
			{`"kind": "marks"`, `"kind": "symbols"`}, // a cell's kind
			{`"kind": "empty"`, `"kind": "blank"`},
			{`"label": "spine"`, `"label": "ancestor"`}, // a row's label
			{`"label": "sibling"`, `"label": "own"`},
		}},
		{"queue", Query{Task: "a1c0", Person: "ada@example.org", Level: Glance}, [][2]string{
			{`"kind": "ready"`, `"kind": "work"`}, // an item's kind
			{`"kind": "reaffirmation"`, `"kind": "reaffirm"`},
		}},
		{"blockage", Query{Task: "a1c0", Level: Detail}, [][2]string{
			{`"kind": "authorisation"`, `"kind": "authorization"`}, // a cause's kind
			{`"act": "records"`, `"act": "record"`},                // the act that resolves it
			{`"advance": "pin"`, `"advance": "commit"`},            // what a snapshot advances
			{`"via": "parent"`, `"via": "child"`},                  // how a cause holds a task
			{`"via": "self"`, `"via": "own"`},
		}},
	} {
		valid := encoded(t, viewOf(f, c.command, c.q))
		if err := s.Validate(envelope(t, c.command, c.q, valid)); err != nil {
			t.Fatalf("%s does not validate: %v", c.command, err)
		}
		for _, change := range c.changes {
			if !bytes.Contains(valid, []byte(change[0])) {
				t.Fatalf("%s holds no %s", c.command, change[0])
			}
			broken := bytes.Replace(valid, []byte(change[0]), []byte(change[1]), 1)
			if err := s.Validate(envelope(t, c.command, c.q, broken)); err == nil {
				t.Errorf("%s: the schema accepts %s", c.command, change[1])
			}
		}
	}
}
