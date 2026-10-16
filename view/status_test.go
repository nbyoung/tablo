package view

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
)

// The keys each status view adds at each level, and those a row, a queue
// item and a cause add, as envelope.schema.yaml names them.
const (
	tableauGlance      = "legend subject columns rows"
	tableauProvenance  = "commands"
	rowGlance          = "id title under depth parent hidden label status leaf cells"
	rowProvenance      = "status_commit own_commit chain junctions sources linkages"
	queueGlance        = "legend items"
	queueProvenance    = "brief"
	itemGlance         = "kind task gate model since age causes why"
	itemDetail         = "junction sources task_references requires unblocks also_required_by parent_unblocks status status_commit authorisation"
	blockageGlance     = "legend causes kinds not_due_count not_due_unmet"
	blockageDetail     = "not_due"
	blockageProvenance = "commands"
	causeGlance        = "kind task url gate status pin tip resolver mark act advance holds"
	causeDetail        = "held"
	causeProvenance    = "status_commit junction authorisation linkage resolve"
	briefKeys          = "kind causes task gate junction supplier requires dependents parent status"
)

// defs reads the $defs of envelope.schema.yaml, from the module's root.
func defs(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "envelope.schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc["$defs"].(map[string]any)
}

// named returns the keys a definition of the schema requires, and every
// key it names, each sorted.
func named(t *testing.T, defs map[string]any, name string) (required, all []string) {
	t.Helper()
	def, ok := defs[name].(map[string]any)
	if !ok {
		t.Fatalf("the schema holds no %s", name)
	}
	for _, key := range def["required"].([]any) {
		required = append(required, key.(string))
	}
	for key := range def["properties"].(map[string]any) {
		all = append(all, key)
	}
	sort.Strings(required)
	sort.Strings(all)
	return required, all
}

// T1: the encoding by level. The keys of the three types at each level are
// the sets the schema names: what the glance writes is what the schema
// requires, and what provenance writes is every key it names. A row, a
// queue item and a cause add their keys at the level stated.
func TestStatusKeysByLevel(t *testing.T) {
	defs := defs(t)
	cases := []struct {
		name   string
		def    string // the definition of the schema; "" for a level between
		lowest any
		whole  any
		levels []string
	}{
		{"a tableau", "view_tableau", Tableau{}, Tableau{TableauProvenance: &TableauProvenance{}}, []string{tableauGlance, tableauProvenance}},
		{"a row", "view_grid_row", GridRow{}, GridRow{RowProvenance: &RowProvenance{}}, []string{rowGlance, rowProvenance}},
		{"a queue", "view_queue", Queue{}, Queue{QueueProvenance: &QueueProvenance{}}, []string{queueGlance, queueProvenance}},
		{"a queue item", "view_queue_item", QueueItem{}, QueueItem{ItemDetail: &ItemDetail{}}, []string{itemGlance, itemDetail}},
		{"a tree", "view_blockage", Blockage{}, Blockage{BlockageDetail: &BlockageDetail{}, BlockageProvenance: &BlockageProvenance{}}, []string{blockageGlance, blockageDetail, blockageProvenance}},
		{"a tree at detail", "", Blockage{BlockageDetail: &BlockageDetail{}}, nil, []string{blockageGlance, blockageDetail}},
		{"a cause", "view_cause", Cause{}, Cause{CauseDetail: &CauseDetail{}, CauseProvenance: &CauseProvenance{}}, []string{causeGlance, causeDetail, causeProvenance}},
		{"a cause at detail", "", Cause{CauseDetail: &CauseDetail{}}, nil, []string{causeGlance, causeDetail}},
		{"a brief", "view_brief", Brief{}, Brief{}, []string{briefKeys}},
	}
	for _, c := range cases {
		if c.def == "" {
			if got, want := keysOf(t, c.lowest), sorted(c.levels...); !reflect.DeepEqual(got, want) {
				t.Errorf("%s: the keys are %v, want %v", c.name, got, want)
			}
			continue
		}
		required, all := named(t, defs, c.def)
		if got := keysOf(t, c.lowest); !reflect.DeepEqual(got, sorted(c.levels[0])) || !reflect.DeepEqual(got, required) {
			t.Errorf("%s at its lowest level: the keys are %v, want %v, and the schema requires %v", c.name, got, sorted(c.levels[0]), required)
		}
		if got := keysOf(t, c.whole); !reflect.DeepEqual(got, sorted(c.levels...)) || !reflect.DeepEqual(got, all) {
			t.Errorf("%s at provenance: the keys are %v, want %v, and the schema names %v", c.name, got, sorted(c.levels...), all)
		}
	}
	// Every other type of the three views writes each key the schema requires of it.
	for def, v := range map[string]any{
		"view_cell": Cell{}, "view_leaf": Leaf{}, "view_wait": Wait{}, "view_parent_edge": ParentEdge{}, "view_kind_count": KindCount{},
		"view_held": Held{}, "view_waiting": Waiting{}, "view_brief_task": BriefTask{}, "view_brief_gate": BriefGate{},
		"view_brief_junction": BriefJunction{}, "view_brief_field": BriefField{}, "view_brief_source": BriefSource{},
		"view_brief_reference": BriefReference{}, "view_brief_supplier": BriefSupplier{}, "view_brief_edge": BriefEdge{},
		"view_brief_parent": BriefParent{}, "view_brief_status": BriefStatus{},
	} {
		required, all := named(t, defs, def)
		if got := keysOf(t, v); !reflect.DeepEqual(got, required) || !reflect.DeepEqual(got, all) {
			t.Errorf("%s: the keys are %v; the schema requires %v and names %v", def, got, required, all)
		}
	}
}

// T1: an empty list is [], an absent object is null and a level the data
// does not reach leaves its keys out.
func TestStatusEmptyValues(t *testing.T) {
	for want, v := range map[string]any{
		`{"kind":"","historical":false,"state":"","reason":"","marks":[],"acts":false}`:                                                   Cell{Marks: []string{}},
		`{"kind":"","requires":null,"reason":"","note":"","by":""}`:                                                                       Wait{},
		`{"task":{"id":"","title":""},"gate":"","parent":false,"via":"","child":"","requires":null,"contributor":"","also":[],"held":[]}`: Held{Also: []int{}, Held: []Held{}},
		`{"kind":"","task":{"id":"","title":""},"gate":"","model":"","since":"","age":0,"causes":[],"why":""}`:                            QueueItem{Causes: []Wait{}},
		`{"gates":[],"folded":false,"count":0,"by_gate":[]}`:                                                                              Column{Gates: []string{}, ByGate: []int{}},
	} {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Errorf("%T is %s, want %s", v, data, want)
		}
	}
	data, err := json.Marshal(Queue{Items: []QueueItem{}, QueueProvenance: &QueueProvenance{}})
	if err != nil {
		t.Fatal(err)
	}
	var queue map[string]json.RawMessage
	if err := json.Unmarshal(data, &queue); err != nil {
		t.Fatal(err)
	}
	if string(queue["items"]) != "[]" || string(queue["brief"]) != "null" {
		t.Errorf("a queue with no item and no brief is %s", data)
	}
	if keys := keysOf(t, Tableau{}); !reflect.DeepEqual(keys, sorted(tableauGlance)) {
		t.Errorf("a tableau at glance holds %v", keys)
	}
}
