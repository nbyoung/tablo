package validate

import (
	"reflect"
	"testing"
)

// TestApplicableAgreesWithTheCorpus is T10, first part: the Validator's own
// resolution gives every applicable list that junction-kinds and
// weather-station state.
func TestApplicableAgreesWithTheCorpus(t *testing.T) {
	lists := 0
	for _, name := range []string{"junction-kinds", "weather-station"} {
		s := newShape(entry(t, name))
		for id, task := range expected(t, name).Tasks {
			if task.Applicable == nil {
				continue
			}
			lists++
			if got := s.applicable(id); !reflect.DeepEqual(got, task.Applicable) {
				t.Errorf("%s %s: applicable %v; want %v", name, id, got, task.Applicable)
			}
		}
	}
	if lists < 5 {
		t.Errorf("the two entries state %d applicable lists; want at least 5", lists)
	}
}

// TestKindAt is T10, second part: the nearest entry that decides gives the
// kind. An applies: true entry, a mixed one and an ancestor's recursive one
// decide nothing, and an entry of a descendant's own stands under an
// exemption.
func TestKindAt(t *testing.T) {
	const leaf = "title: Task\ndescription: A task.\nassignee: pat@example.org\n"
	p := project(t, with(base(),
		"tasks/e4a1.yaml", baseRoot+`junctions:
  design: { applies: false }
  release: { subproject: { url: lib } }
`,
		"tasks/b2c9.yaml", leaf+"junctions: { design: { applies: true } }\nparent: { id: \"e4a1\" }\n",
		"tasks/c3d7.yaml", leaf+"junctions: { design: { applies: false, reviewer: olive@example.org } }\nparent: { id: \"e4a1\" }\n",
		"tasks/d4e8.yaml", leaf+"junctions: { design: { reviewer: olive@example.org } }\nparent: { id: \"e4a1\" }\n",
		"tasks/f6a0.yaml", leaf+"junctions: { design: { colour: red }, defined: { subproject: { url: lib } } }\nparent: { id: \"d4e8\" }\n",
		"tasks/0b1c.yaml", leaf+"junctions: { design: [] }\nparent: { id: \"e4a1\" }\n",
	))
	s := newShape(p)
	cases := []struct {
		id, gate string
		want     resolved
	}{
		{"e4a1", "undefined", plain},
		{"e4a1", "design", notApplicable},
		{"e4a1", "release", recursive},
		{"b2c9", "design", notApplicable}, // applies: true decides nothing
		{"c3d7", "design", notApplicable}, // a mixed entry decides nothing
		{"0b1c", "design", notApplicable}, // an entry that is no mapping decides nothing
		{"d4e8", "design", plain},         // its own entry stands under the exemption
		{"f6a0", "design", plain},         // an unknown field decides nothing; d4e8's entry does
		{"f6a0", "defined", recursive},
		{"b2c9", "release", plain}, // an ancestor's recursive entry never decides
		{"b2c9", "nonesuch", plain},
	}
	for _, c := range cases {
		if got := s.kindAt(c.id, c.gate); got != c.want {
			t.Errorf("kindAt(%s, %s) = %d; want %d", c.id, c.gate, got, c.want)
		}
	}
	if got, want := s.applicable("b2c9"), []string{"undefined", "defined", "release"}; !reflect.DeepEqual(got, want) {
		t.Errorf("applicable(b2c9) = %v; want %v", got, want)
	}
	if chain, whole := s.ancestors("f6a0"); !whole || !reflect.DeepEqual(chain, []string{"d4e8", "e4a1"}) {
		t.Errorf("ancestors(f6a0) = %v, %v", chain, whole)
	}
}
