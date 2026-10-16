package view

import (
	"reflect"
	"strings"
	"testing"
)

// The keys the history and each of its lines add at each level, in the order
// the data writes them.
const (
	historyGlance     = "legend subject events commits kinds days lines"
	historyProvenance = "commands"
	lineGlance        = "task proposal events"
	lineDetail        = "after unchanged subject_after effect model check committer"
	lineProvenance    = "commit"
)

// historyAt returns the history at glance, at detail and at provenance, with
// one line and every list of it empty.
func historyAt(level int) History {
	l := Line{Events: []Event{}}
	h := History{Kinds: []EventCount{}, Days: []Day{}}
	if level > 0 {
		l.LineDetail = &LineDetail{}
	}
	if level > 1 {
		l.LineProvenance = &LineProvenance{}
		h.HistoryProvenance = &HistoryProvenance{Commands: []Command{}}
	}
	h.Lines = []Line{l}
	return h
}

// T1: the history is one flat object whose levels nest: each level writes
// its keys after those of the level below, in the order of the declarations;
// LineDetail and LineProvenance are absent below their levels; a list is an
// array and an absent fact null.
func TestHistoryKeysByLevel(t *testing.T) {
	cases := []struct {
		name          string
		level         int
		history, line string
	}{
		{"glance", 0, historyGlance, lineGlance},
		{"detail", 1, historyGlance, lineGlance + " " + lineDetail},
		{"provenance", 2, historyGlance + " " + historyProvenance, lineGlance + " " + lineDetail + " " + lineProvenance},
	}
	for _, c := range cases {
		h := historyAt(c.level)
		keys, values := inOrder(t, h)
		if want := strings.Fields(c.history); !reflect.DeepEqual(keys, want) {
			t.Errorf("the history at %s writes %v, want %v", c.name, keys, want)
		}
		for _, list := range []string{"kinds", "days"} {
			if values[list] != "[]" {
				t.Errorf("the history at %s writes %s as %s", c.name, list, values[list])
			}
		}
		if c.level > 1 && values["commands"] != "[]" {
			t.Errorf("the history at provenance writes the commands as %s", values["commands"])
		}
		keys, values = inOrder(t, h.Lines[0])
		if want := strings.Fields(c.line); !reflect.DeepEqual(keys, want) {
			t.Errorf("a line at %s writes %v, want %v", c.name, keys, want)
		}
		if values["events"] != "[]" || values["task"] != `{"id":"","title":""}` || values["proposal"] != "false" {
			t.Errorf("a line at %s writes %v", c.name, values)
		}
		if c.level > 0 {
			for key, want := range map[string]string{"after": "null", "subject_after": "null", "check": "null", "effect": `""`, "model": `""`, "committer": `""`, "unchanged": "false"} {
				if values[key] != want {
					t.Errorf("a line at %s writes %s as %s, want %s", c.name, key, values[key], want)
				}
			}
		}
		if c.level > 1 && values["commit"] != "null" {
			t.Errorf("a line at provenance writes the commit as %s", values["commit"])
		}
	}
	for _, c := range []struct {
		v    any
		want string
	}{
		{EventCount{}, "kind count"},
		{Day{Kinds: []EventCount{}}, "date events commits kinds"},
	} {
		keys, values := inOrder(t, c.v)
		if want := strings.Fields(c.want); !reflect.DeepEqual(keys, want) {
			t.Errorf("%T writes %v, want %v", c.v, keys, want)
		}
		if kinds, ok := values["kinds"]; ok && kinds != "[]" {
			t.Errorf("a day writes its kinds as %s", kinds)
		}
	}
}

// T1: the tags of the history: one JSON key per field, in snake case, and
// none omitted when empty.
func TestHistoryTags(t *testing.T) {
	for _, v := range []any{History{}, HistoryProvenance{}, EventCount{}, Day{}, Line{}, LineDetail{}, LineProvenance{}} {
		typ := reflect.TypeOf(v)
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			tag := field.Tag.Get("json")
			if field.Anonymous {
				if tag != "" {
					t.Errorf("%s.%s embeds a level with the tag %q", typ.Name(), field.Name, tag)
				}
				continue
			}
			if tag == "" || strings.Contains(tag, ",") || strings.ToLower(tag) != tag {
				t.Errorf("%s.%s has the tag %q", typ.Name(), field.Name, tag)
			}
		}
	}
}
