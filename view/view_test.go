package view

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// keysOf marshals a value and returns the keys of its JSON object, sorted.
func keysOf(t *testing.T, v any) []string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("%s is no object: %v", data, err)
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sorted returns the words of a text, sorted.
func sorted(words ...string) []string {
	var keys []string
	for _, w := range words {
		keys = append(keys, strings.Fields(w)...)
	}
	sort.Strings(keys)
	return keys
}

// The keys each view adds at each level, as envelope.schema.yaml names them.
const (
	gatesGlance       = "legend task"
	gatesDetail       = "for_task"
	gatesProvenance   = "files commands"
	taskGlance        = "legend task assignee parent status authorised"
	taskDetail        = "description references path siblings children authorities requires dependents junctions snapshot"
	taskProvenance    = "status_commit own_commit authorisation considered chain linkages sources reviews models events event_count commands"
	delegGlance       = "legend rows"
	delegDetail       = "parents"
	delegProvenance   = "deciding undecided commands"
	assignGlance      = "legend people recursive"
	assignDetail      = "sections"
	sectionDetail     = "email assigned recursive contributes reviews models counts authority"
	sectionProvenance = "positions"
)

// T1: the encoding by level. Each view is one flat object; a level the data
// does not reach leaves its keys out, and a level it reaches writes every
// one of its keys.
func TestKeysByLevel(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want []string
	}{
		{"gates at glance", Gates{}, sorted(gatesGlance)},
		{"gates at detail", Gates{GatesDetail: &GatesDetail{}}, sorted(gatesGlance, gatesDetail)},
		{"gates at provenance", Gates{GatesDetail: &GatesDetail{}, GatesProvenance: &GatesProvenance{}}, sorted(gatesGlance, gatesDetail, gatesProvenance)},
		{"task at glance", Task{}, sorted(taskGlance)},
		{"task at detail", Task{TaskDetail: &TaskDetail{}}, sorted(taskGlance, taskDetail)},
		{"task at provenance", Task{TaskDetail: &TaskDetail{}, TaskProvenance: &TaskProvenance{}}, sorted(taskGlance, taskDetail, taskProvenance)},
		{"authority at glance", Delegation{}, sorted(delegGlance)},
		{"authority at detail", Delegation{DelegationDetail: &DelegationDetail{}}, sorted(delegGlance, delegDetail)},
		{"authority at provenance", Delegation{DelegationDetail: &DelegationDetail{}, DelegationProvenance: &DelegationProvenance{}}, sorted(delegGlance, delegDetail, delegProvenance)},
		{"assignment at glance", Assignment{}, sorted(assignGlance)},
		{"assignment at detail", Assignment{AssignmentDetail: &AssignmentDetail{}}, sorted(assignGlance, assignDetail)},
		{"a section at detail", PersonSection{}, sorted(sectionDetail)},
		{"a section at provenance", PersonSection{SectionProvenance: &SectionProvenance{}}, sorted(sectionDetail, sectionProvenance)},
	}
	for _, c := range cases {
		if got := keysOf(t, c.v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: the keys are %v, want %v", c.name, got, c.want)
		}
	}
}

// T1: a list a level reaches is an array, [] when empty, an absent object is
// null and an absent text is "".
func TestEmptyValues(t *testing.T) {
	data, err := json.Marshal(Gates{GatesDetail: &GatesDetail{ForTask: []GateFor{}}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `"task":null,"for_task":[]}`; !strings.HasSuffix(string(data), want) {
		t.Errorf("an empty gate definition ends %s, want %s", data, want)
	}
	data, err = json.Marshal(GateFor{References: []Reference{}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"gate":"","applies":false,"references":[],"stated_by":""}`; string(data) != want {
		t.Errorf("an empty row is %s, want %s", data, want)
	}
}

// T1: from is absent where the view shows no suppliers and [] where it shows
// them and the junction has none.
func TestJunctionFrom(t *testing.T) {
	has := func(j Junction) (string, bool) {
		data, err := json.Marshal(j)
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil {
			t.Fatal(err)
		}
		from, ok := object["from"]
		return string(from), ok
	}
	if from, ok := has(Junction{}); ok {
		t.Errorf("a nil from is written as %s", from)
	}
	if from, ok := has(Junction{From: []string{}}); !ok || from != "[]" {
		t.Errorf("an empty from is %q, %v; want []", from, ok)
	}
	if from, ok := has(Junction{From: []string{"4e2b"}}); !ok || from != `["4e2b"]` {
		t.Errorf("a from of one is %q, %v", from, ok)
	}
	if keys := keysOf(t, JunctionRow{}); !reflect.DeepEqual(keys, sorted("task gate kind marks acts contributor model reviewer reviewer_by_default subproject references stands reviewed")) {
		t.Errorf("a junction row holds %v", keys)
	}
}

// T1: every field of every type a view reaches carries a JSON tag, and no
// tag but a junction's from lets a value go unwritten.
func TestTags(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(ty reflect.Type)
	walk = func(ty reflect.Type) {
		for ty.Kind() == reflect.Pointer || ty.Kind() == reflect.Slice {
			ty = ty.Elem()
		}
		if ty.Kind() != reflect.Struct || seen[ty] {
			return
		}
		seen[ty] = true
		for i := 0; i < ty.NumField(); i++ {
			field := ty.Field(i)
			walk(field.Type)
			tag, ok := field.Tag.Lookup("json")
			if field.Anonymous {
				if ok {
					t.Errorf("%s.%s is embedded and carries the tag %q", ty.Name(), field.Name, tag)
				}
				continue
			}
			name, options, _ := strings.Cut(tag, ",")
			switch {
			case !ok || name == "" || name == "-":
				t.Errorf("%s.%s carries no key", ty.Name(), field.Name)
			case options != "" && (ty != reflect.TypeOf(Junction{}) || field.Name != "From" || options != "omitzero"):
				t.Errorf("%s.%s carries the option %q", ty.Name(), field.Name, options)
			}
		}
	}
	for _, v := range []any{Gates{}, Task{}, Delegation{}, Assignment{}, Tableau{}, Queue{}, Blockage{}} {
		walk(reflect.TypeOf(v))
	}
	if len(seen) < 40 {
		t.Errorf("the walk reaches %d types", len(seen))
	}
}
