package views

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/view"
)

// observed returns the task definition at provenance as a consumer reads
// it: the JSON, decoded.
func observed(t testing.TB, f *derive.Facts, id string) map[string]any {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal(encoded(t, Task(f, Query{Task: id, Level: Provenance})), &data); err != nil {
		t.Fatal(err)
	}
	return data
}

// get follows keys through decoded JSON; it returns nil where the path ends.
func get(v any, path string) any {
	for _, key := range strings.Split(path, ".") {
		object, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = object[key]
	}
	return v
}

// str returns a value of decoded JSON as text: "" for null and for a key the
// data lacks.
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// list returns a list of decoded JSON.
func list(v any) []any {
	l, _ := v.([]any)
	return l
}

// compareAuthorisation compares what expected.yaml states of a task's
// authorisation with the task definition.
func compareAuthorisation(c *check, e *expectation, data map[string]any, stated map[string]any) {
	state := "proposed"
	if data["authorised"] == true {
		state = "authorised"
	}
	c.eq("the authorisation", state, text(stated["state"]))
	if commit, ok := stated["commit"]; ok {
		c.eq("the deciding commit", str(get(data, "authorisation.commit.hash")), e.hash(text(commit)))
	}
	if by, ok := stated["by"]; ok {
		c.eq("the authorisation's hand", str(get(data, "authorisation.email")), text(by))
	}
}

// T7: the task definition agrees with the corpus. For every valid entry and
// every ref it states, each fact of expected.yaml that the table of design
// 493e maps equals the task definition at provenance.
func TestTaskAgreesWithCorpus(t *testing.T) {
	c := &check{t: t}
	for _, e := range expectations(t) {
		f := e.at(t, e.ref)
		if f.Refused() != nil {
			t.Errorf("%s: refused: %+v", e.name, f.Refused())
			continue
		}
		c.where = e.name
		views := map[string]map[string]any{}
		for _, id := range f.Order() {
			views[id] = observed(t, f, id)
		}

		// The owner, the root, the order and the counts, by the walk over
		// parent and children that design 4b4f states.
		root := ""
		for _, id := range f.Order() {
			if views[id]["parent"] == nil {
				root = id
			}
		}
		var order []string
		leaves := 0
		var walk func(id string)
		walk = func(id string) {
			order = append(order, id)
			children := list(views[id]["children"])
			if len(children) == 0 {
				leaves++
			}
			for _, child := range children {
				walk(str(get(child, "id")))
			}
		}
		walk(root)
		if want, ok := e.want["owner"]; ok {
			c.eq("the owner", str(views[root]["assignee"]), text(want))
		}
		if want, ok := e.want["root"]; ok {
			c.eq("the root", root, text(want))
		}
		if want, ok := e.want["order"]; ok {
			c.eq("the order", order, texts(want))
		}
		if want, ok := e.want["count"].(map[string]any); ok {
			c.eq("the count of tasks", len(order), want["tasks"])
			c.eq("the count of leaves", leaves, want["leaves"])
		}

		tasks, _ := e.want["tasks"].(map[string]any)
		for _, id := range sortedKeys(tasks) {
			want, _ := tasks[id].(map[string]any)
			data := views[id]
			c.where = e.name + " " + id
			if data == nil {
				c.eq("the task", nil, want)
				continue
			}
			if parent, ok := want["parent"]; ok {
				c.eq("the parent", str(get(data, "parent.id")), text(parent))
			}
			if authorities, ok := want["authorities"]; ok {
				got := []string{}
				for _, a := range list(data["authorities"]) {
					got = append(got, str(get(a, "email")))
				}
				c.eq("the authorities", got, append([]string{}, texts(authorities)...))
			}
			if stated, ok := want["authorisation"].(map[string]any); ok {
				compareAuthorisation(c, e, data, stated)
			}
			junctions := map[string]any{}
			applicable := []string{"undefined"}
			for _, j := range list(data["junctions"]) {
				junctions[str(get(j, "gate"))] = j
				if str(get(j, "kind")) != "not_applicable" {
					applicable = append(applicable, str(get(j, "gate")))
				}
			}
			if stated, ok := want["applicable"]; ok {
				c.eq("the applicable gates", applicable, texts(stated))
			}
			wanted, _ := want["junctions"].(map[string]any)
			for _, gate := range sortedKeys(wanted) {
				stated, _ := wanted[gate].(map[string]any)
				if gate == "default" {
					c.eq("the default's kind", "plain", text(stated["kind"]))
					c.eq("the default's contributor", str(data["assignee"]), text(stated["contributor"]))
					continue
				}
				j := junctions[gate]
				if j == nil {
					c.eq("the junction at "+gate, nil, stated)
					continue
				}
				at := gate + "'s "
				c.eq(at+"kind", str(get(j, "kind")), text(stated["kind"]))
				c.eq(at+"contributor", str(get(j, "contributor")), text(stated["contributor"]))
				c.eq(at+"model", str(get(j, "model")), text(stated["model"]))
				c.eq(at+"reviewer", str(get(j, "reviewer")), text(stated["reviewer"]))
				c.eq(at+"sources", get(j, "from"), anyList(texts(stated["source"])))
				references := []any{}
				if refs, ok := stated["references"].([]any); ok {
					references = refs
				}
				var got []any
				for _, r := range list(get(j, "references")) {
					got = append(got, map[string]any{"url": get(r, "url"), "text": get(r, "text")})
				}
				c.eq(at+"references", fmt.Sprint(got), fmt.Sprint(references))
				if sub, ok := stated["subproject"].(map[string]any); ok {
					c.eq(at+"url", str(get(j, "subproject.url")), text(sub["url"]))
					if target, ok := sub["id"]; ok {
						c.eq(at+"target", str(get(j, "subproject.id")), text(target))
					}
					if commit, ok := sub["commit"]; ok {
						c.eq(at+"commit", str(get(j, "subproject.commit")), e.hash(text(commit)))
					}
				}
			}
			if stated, ok := want["requires"].([]any); ok {
				got := list(data["requires"])
				if c.eq("the count of requirements", len(got), len(stated)); len(got) == len(stated) {
					for i, item := range stated {
						r, at := item.(map[string]any), fmt.Sprintf("requirement %d's ", i)
						if sub, ok := r["subproject"].(map[string]any); ok {
							c.eq(at+"origin", str(get(got[i], "task.id")), text(sub["id"]))
							c.eq(at+"url", str(get(got[i], "subproject")), text(sub["url"]))
							c.eq(at+"pin", str(get(got[i], "commit")), e.hash(text(sub["pin"])))
						} else {
							c.eq(at+"origin", str(get(got[i], "task.id")), text(r["id"]))
							c.eq(at+"url", str(get(got[i], "subproject")), "")
						}
						c.eq(at+"from", str(get(got[i], "from")), text(r["from"]))
						c.eq(at+"to", str(get(got[i], "to")), text(r["to"]))
						c.eq(at+"met", get(got[i], "met"), r["met"])
						c.eq(at+"due", get(got[i], "due"), r["due"])
						c.eq(at+"condition", str(get(got[i], "condition")), text(r["condition"]))
					}
				}
			}
			if stated, ok := want["reviews"].([]any); ok {
				var got, wantReviews []string
				for _, r := range list(data["reviews"]) {
					got = append(got, str(get(r, "gate"))+" "+str(get(r, "reviewer"))+" "+str(get(r, "commit.hash")))
				}
				for _, item := range stated {
					r := item.(map[string]any)
					wantReviews = append(wantReviews, text(r["gate"])+" "+text(r["reviewer"])+" "+e.hash(text(r["commit"])))
				}
				c.eq("the reviews", got, wantReviews)
			}
			if stated, ok := want["subproject"].(map[string]any); ok {
				read := data["snapshot"]
				c.eq("the subproject's url", str(get(read, "url")), text(stated["url"]))
				c.eq("the subproject's pin", str(get(read, "commit")), e.hash(text(stated["pin"])))
				c.eq("the subproject's task", str(get(read, "task.id")), text(stated["task"]))
			}
			if stated, ok := want["status"].(map[string]any); ok {
				c.eq("the status's gate", str(get(data, "status.gate")), text(stated["gate"]))
				if state, ok := stated["state"]; ok {
					c.eq("the status's state", str(get(data, "status.state")), text(state))
				}
				c.eq("the status's reason", str(get(data, "status.reason")), text(stated["reason"]))
				c.eq("the status's note", str(get(data, "status.note")), text(stated["note"]))
				c.eq("the status's date", str(get(data, "status.date")), text(stated["date"]))
				c.eq("the status's recorder", str(get(data, "status.recorder")), text(stated["recorder"]))
				c.eq("the status's roll-up", get(data, "status.derived"), stated["derived"] == true)
				if stated["derived"] != true {
					c.eq("the status's commit", str(get(data, "status_commit.hash")), e.hash(text(stated["commit"])))
				}
				if from, ok := stated["from"]; ok {
					// A roll-up names its child; a snapshot of a parent
					// names the child of the task read.
					got := str(get(data, "status.from.task.id"))
					if str(get(data, "status.kind")) == "snapshot" {
						got = str(get(data, "snapshot.status.from.task.id"))
					}
					c.eq("the status's child", got, text(from))
				}
			}
		}

		refs, _ := e.want["refs"].(map[string]any)
		for _, ref := range sortedKeys(refs) {
			stated := refs[ref].(map[string]any)
			at := e.at(t, ref)
			tasks, _ := stated["tasks"].(map[string]any)
			for _, id := range sortedKeys(tasks) {
				c.where = e.name + " at " + ref + " " + id
				if a, ok := tasks[id].(map[string]any)["authorisation"].(map[string]any); ok {
					compareAuthorisation(c, e, observed(t, at, id), a)
				}
			}
		}
	}
	if c.n < 1000 {
		t.Errorf("the comparison reads %d facts; the corpus states more", c.n)
	}
	t.Logf("%d facts agree or are reported", c.n)
}

// anyList returns a list of strings as decoded JSON holds it: never nil.
func anyList(texts []string) any {
	out := []any{}
	for _, s := range texts {
		out = append(out, s)
	}
	return out
}

// T8: the kinds of status.
func TestStatusKinds(t *testing.T) {
	f, hashes := station(t, "main")
	firmware := labels(t, "weather-station.firmware")
	at := func(id string) *view.Task { return Task(f, Query{Task: id, Level: Provenance}) }

	board := at("9f31")
	if s := board.Status; s.Kind != "recorded" || s.Gate != "function" || s.State != "stalled" || s.Reason != "blocked" || s.Date != "2026-09-28" ||
		s.Recorder != "ada@example.org" || s.Derived || s.From != nil || s.Why != "" {
		t.Errorf("the status of 9f31 is %+v", s)
	}
	if board.StatusCommit == nil || board.StatusCommit.Hash != hashes["W13"] || board.OwnCommit != nil || len(board.Chain) != 1 || len(board.Considered) != 0 || board.Snapshot != nil {
		t.Errorf("9f31: the deciding commit is %+v, with the chain %+v", board.StatusCommit, board.Chain)
	}

	gateway := at("7b2e")
	if s := gateway.Status; s.Kind != "absent" || s.Gate != "undefined" || s.State != "undefined" || s.Recorder != "" || s.From != nil {
		t.Errorf("the status of 7b2e is %+v", s)
	}
	if gateway.StatusCommit == nil || gateway.StatusCommit.Hash != hashes["W1"] {
		t.Errorf("7b2e: the deciding commit is %+v; want W1, the task file's", gateway.StatusCommit)
	}

	node := at("4e2b")
	if s := node.Status; s.Kind != "rollup" || !s.Derived || s.Gate != "function" || s.State != "stalled" || s.Recorder != "" || s.From == nil ||
		*s.From != (view.Origin{Kind: "rollup", Task: view.TaskRef{ID: "9f31", Title: "Sensor board"}}) {
		t.Errorf("the status of 4e2b is %+v from %+v", s, s.From)
	}
	if !reflect.DeepEqual(node.Considered, []string{"9f31", "c07d"}) || len(node.Chain) != 2 || node.Chain[1].Task.ID != "9f31" || node.Chain[1].Status.Kind != "recorded" ||
		node.StatusCommit == nil || node.StatusCommit.Hash != hashes["W13"] || node.OwnCommit != nil {
		t.Errorf("4e2b considers %v through %+v, decided by %+v", node.Considered, node.Chain, node.StatusCommit)
	}

	image := at("c07d")
	if s := image.Status; s.Kind != "snapshot" || s.Derived || s.Gate != "design" || s.Note != "Sleep scheduler in progress" || s.From == nil ||
		*s.From != (view.Origin{Kind: "snapshot", Task: view.TaskRef{ID: "f1a0", Title: "Weather node image"}, URL: "firmware", Pin: firmware["F2"], Gate: "design"}) {
		t.Errorf("the status of c07d is %+v from %+v", s, s.From)
	}
	if image.OwnCommit == nil || image.OwnCommit.Hash != hashes["W11"] || image.StatusCommit == nil || image.StatusCommit.Hash != firmware["F2"] {
		t.Errorf("c07d: its own commit is %+v and the deciding commit %+v; want W11 and F2", image.OwnCommit, image.StatusCommit)
	}
	if len(image.Chain) != 2 || image.Chain[0].URL != "" || image.Chain[1].URL != "firmware" || image.Chain[1].Task.ID != "f1a0" || image.Chain[1].Status.Kind != "recorded" {
		t.Errorf("the chain of c07d is %+v; want it to end in the firmware", image.Chain)
	}
	if n := image.Snapshot; n == nil || n.Task.ID != "f1a0" || n.Assignee != "ben@example.org" || n.Status == nil || n.Status.Gate != "design" || n.Children == nil || len(n.Children) != 0 || n.Commit != firmware["F2"] {
		t.Errorf("the snapshot of c07d is %+v", n)
	}

	// A snapshot of a parent: a roll-up in the subproject gives the state.
	kinds := Task(entry(t, "junction-kinds"), Query{Task: "a500", Level: Provenance})
	if s := kinds.Status; s.Kind != "snapshot" || !s.Derived || s.Recorder != "" || s.From == nil || s.From.Task.ID != "5a00" || s.From.Gate != "defined" {
		t.Errorf("the status of a500 is %+v from %+v; want a derived snapshot of 5a00", s, s.From)
	}
	if len(kinds.Chain) != 3 || kinds.Chain[1].Status.Kind != "rollup" || kinds.Chain[1].URL != "sub" || kinds.Chain[2].Task.ID != "5c00" || kinds.Chain[2].URL != "sub" ||
		kinds.Snapshot == nil || len(kinds.Snapshot.Children) != 2 || kinds.Snapshot.Children[0].ID != "5b00" || kinds.Snapshot.Children[0].Order == nil || *kinds.Snapshot.Children[0].Order != 1 {
		t.Errorf("a500 reads %+v through %+v", kinds.Snapshot, kinds.Chain)
	}
}

// The glance and the detail of the task definition: the parent, the path,
// the children, the authorities and what each level leaves out.
func TestTaskLevels(t *testing.T) {
	f, _ := station(t, "main")
	glance := Task(f, Query{Task: "3c5d"})
	if glance.TaskDetail != nil || glance.TaskProvenance != nil || glance.Assignee != "dan@example.org" || glance.Authorised ||
		glance.Parent == nil || glance.Parent.ID != "a1c0" || glance.Parent.Title != "Weather station" || glance.Parent.Order != nil {
		t.Errorf("the glance of 3c5d is %+v under %+v; want it proposed, with no order", glance, glance.Parent)
	}
	root := Task(f, Query{Task: "a1c0", Level: Detail})
	if root.Parent != nil || root.Siblings != 0 || len(root.Path) != 1 || len(root.Authorities) != 0 || !root.Authorised || root.TaskProvenance != nil {
		t.Errorf("the root is %+v", root)
	}
	var children []string
	for _, c := range root.Children {
		order := "none"
		if c.Order != nil {
			order = fmt.Sprint(*c.Order)
		}
		children = append(children, c.ID+" "+order+" "+c.Status.Kind+" "+c.Status.Gate)
	}
	if want := []string{"4e2b 1 rollup function", "7b2e 2 absent undefined", "3c5d none recorded defined"}; !reflect.DeepEqual(children, want) {
		t.Errorf("the children of the root are %v; want %v", children, want)
	}
	leaf := Task(f, Query{Task: "c07d", Level: Detail})
	if leaf.Siblings != 2 || leaf.Parent == nil || leaf.Parent.Order == nil || *leaf.Parent.Order != 2 ||
		!reflect.DeepEqual(leaf.Path, []view.TaskRef{{ID: "a1c0", Title: "Weather station"}, {ID: "4e2b", Title: "Sensor node"}, {ID: "c07d", Title: "Node firmware"}}) ||
		!reflect.DeepEqual(leaf.Authorities, []view.Authority{{Email: "ben@example.org", By: "4e2b"}, {Email: "ada@example.org", By: "a1c0"}}) {
		t.Errorf("c07d stands at %+v under %+v with %+v", leaf.Path, leaf.Parent, leaf.Authorities)
	}
	if len(leaf.Junctions) != 11 || leaf.Junctions[0].Gate != "defined" || leaf.Junctions[0].From != nil {
		t.Errorf("c07d holds %d junctions at detail, the first %+v; want eleven after undefined, without from", len(leaf.Junctions), leaf.Junctions[0])
	}
	if with := Task(f, Query{Task: "c07d", Level: Provenance}); with.Junctions[0].From == nil || len(with.Junctions[0].From) != 0 || !reflect.DeepEqual(with.Junctions[1].From, []string{"4e2b"}) {
		t.Errorf("at provenance the junctions of c07d come from %#v and %#v", with.Junctions[0].From, with.Junctions[1].From)
	}
	if board := Task(f, Query{Task: "9f31", Level: Detail}); len(board.References) != 2 || board.References[0] != (view.Reference{URL: "docs/sensor-board.md"}) || board.Description == "" {
		t.Errorf("9f31 holds the references %+v", board.References)
	}
}

// T11: requirements, both ways.
func TestRequirements(t *testing.T) {
	f, _ := station(t, "main")
	image := Task(f, Query{Task: "c07d", Level: Detail})
	if len(image.Requires) != 1 {
		t.Fatalf("c07d has the requirements %+v", image.Requires)
	}
	if r := image.Requires[0]; r.Task != (view.TaskRef{ID: "9f31", Title: "Sensor board"}) || r.From != "design" || r.To != "implementation" || r.Met || !r.Due ||
		r.Condition != "unmet" || r.Why != "" || r.Status == nil || r.Status.Gate != "function" || r.Status.State != "stalled" || r.Subproject != "" || r.Commit != "" || r.AtTrunk != "unknown" {
		t.Errorf("c07d requires %+v with %+v; want it unmet and due", r, r.Status)
	}
	dash := Task(f, Query{Task: "3c5d", Level: Detail})
	if r := dash.Requires[0]; r.Task.ID != "7b2e" || r.Condition != "pending" || r.Met || r.Due || r.Status == nil || r.Status.Kind != "absent" {
		t.Errorf("3c5d requires %+v; want it pending", r)
	}
	board := Task(f, Query{Task: "9f31", Level: Detail})
	if len(board.Requires) != 0 || len(board.Dependents) != 1 {
		t.Fatalf("9f31 has %d requirements and %d dependents", len(board.Requires), len(board.Dependents))
	}
	if d := board.Dependents[0]; d.Task != (view.TaskRef{ID: "c07d", Title: "Node firmware"}) || d.From != "design" || d.To != "implementation" || d.Condition != "unmet" || !d.Due || d.Status != nil {
		t.Errorf("9f31 has the dependent %+v", d)
	}

	// Across projects: the url, the commit read, and the other project's status.
	across := entry(t, "requires-across-projects")
	sub, umbrella := labels(t, "requires-across-projects.sub"), labels(t, "requires-across-projects.umbrella")
	pinned := func(commit string, of ...map[string]string) bool {
		for _, hashes := range of {
			for _, hash := range hashes {
				if hash == commit {
					return true
				}
			}
		}
		return false
	}
	byURL := Task(across, Query{Task: "b2c9", Level: Detail}).Requires[0]
	if byURL.Subproject != "https://example.org/umbrella.git" || !pinned(byURL.Commit, umbrella) || byURL.Task.ID != "9e10" || byURL.Task.Title == "" ||
		byURL.Condition != "met" || byURL.Status == nil || byURL.Status.Gate != "design" || byURL.AtTrunk != "yes" {
		t.Errorf("b2c9 requires %+v with %+v across projects", byURL, byURL.Status)
	}
	byPath := Task(across, Query{Task: "c3d7", Level: Detail}).Requires[0]
	if byPath.Subproject != "sub" || !pinned(byPath.Commit, sub) || byPath.Task.ID != "5a00" || byPath.Condition != "pending" || byPath.Status == nil ||
		byPath.Status.Gate != "defined" || byPath.AtTrunk != "no" {
		t.Errorf("c3d7 requires %+v with %+v across projects", byPath, byPath.Status)
	}

	// A task that is not there.
	missing := Task(entry(t, "requires-missing-task"), Query{Task: "b2c9", Level: Detail}).Requires
	if len(missing) != 1 || missing[0].Why != "no_task" || missing[0].Status != nil || missing[0].Condition != "" || missing[0].Met || missing[0].Due || missing[0].Task.Title != "" {
		t.Errorf("a requirement on no task is %+v", missing)
	}
}

// T12: linkages.
func TestLinkages(t *testing.T) {
	f, hashes := station(t, "main")
	firmware := labels(t, "weather-station.firmware")
	links := Task(f, Query{Task: "c07d", Level: Provenance}).Linkages
	if len(links) != 1 {
		t.Fatalf("c07d has the linkages %+v", links)
	}
	l := links[0]
	if !reflect.DeepEqual(l.Gates, []string{"implementation"}) || l.Form != "submodule" || l.URL != "firmware" || l.Commit != firmware["F2"] || l.Recorded != firmware["F2"] || l.Moved ||
		l.Trunk != "main" || l.Tip != firmware["F3"] || l.OnTrunk != "yes" || l.Behind != "yes" || l.StatedCommit != "" ||
		l.Pinned == nil || l.Pinned.Hash != firmware["F2"] || l.SetBy == nil || l.SetBy.Hash != hashes["W11"] {
		t.Errorf("the linkage of c07d is %+v; want the submodule at F2, on the trunk and behind, set by W11", l)
	}
	if l.Stated != (view.Stated{By: "c07d", Key: "junctions.implementation.subproject.url", File: ".tableaux/tasks/c07d.yaml", Line: 9, Col: 40}) {
		t.Errorf("the linkage of c07d states its url at %+v", l.Stated)
	}
	if none := Task(f, Query{Task: "9f31", Level: Provenance}).Linkages; none == nil || len(none) != 0 {
		t.Errorf("9f31 has the linkages %#v; want []", none)
	}

	one := func(name string) view.Linkage {
		t.Helper()
		links := Task(entry(t, name), Query{Task: "b2c9", Level: Provenance}).Linkages
		if len(links) != 1 {
			t.Fatalf("%s: b2c9 has the linkages %+v", name, links)
		}
		return links[0]
	}
	if l := one("subproject-by-url"); l.Form != "url" || l.URL != "https://example.org/lib.git" || l.StatedCommit == "" || l.StatedCommit != l.Commit || l.Recorded != "" ||
		l.Pinned == nil || l.Pinned.Hash != l.Commit || l.SetBy == nil || l.Task.ID != "5a00" {
		t.Errorf("subproject-by-url: the linkage is %+v; want the url and the stated commit", l)
	}
	if l := one("subproject-same-repository"); l.Form != "directory" || l.URL != "lib" || l.Commit != "" || l.StatedCommit != "" || l.OnTrunk != "unknown" || l.Behind != "unknown" ||
		l.Pinned != nil || l.SetBy != nil || l.Why != "" || l.Task.ID != "5a00" {
		t.Errorf("subproject-same-repository: the linkage is %+v; want a directory with no commit", l)
	}
	if l := one("subproject-pin-off-trunk"); l.OnTrunk != "no" || l.Behind != "unknown" || l.Form != "submodule" || l.Pinned == nil || l.Tip == "" || l.Tip == l.Commit {
		t.Errorf("subproject-pin-off-trunk: the linkage is %+v; want the pin off the trunk", l)
	}
	if l := one("subproject-path-missing"); l.Why != "no_link" || l.Form != "none" || l.URL != "elsewhere" || l.Task != (view.TaskRef{}) || l.Commit != "" || l.Pinned != nil ||
		l.OnTrunk != "unknown" || l.Trunk != "" || l.By != "b2c9" || l.Line == 0 {
		t.Errorf("subproject-path-missing: the linkage is %+v; want no link", l)
	}
}

// T13: reviews, models and events.
func TestReviewsModelsEvents(t *testing.T) {
	at := func(name string) *view.Task { return Task(entry(t, name), Query{Task: "b2c9", Level: Provenance}) }

	byAuthorisation := at("status-defined-by-authorisation").Reviews
	if len(byAuthorisation) != 1 || byAuthorisation[0].Gate != "defined" || byAuthorisation[0].Effect != "authorisation" || byAuthorisation[0].Commit == nil || byAuthorisation[0].By != "olive@example.org" {
		t.Errorf("status-defined-by-authorisation: the reviews are %+v", byAuthorisation)
	}
	// The wrong hand accepts nothing: the status passes no reviewed junction,
	// and the trailer stays an event.
	wrong := at("review-by-non-reviewer")
	if wrong.Reviews == nil || len(wrong.Reviews) != 0 || wrong.EventCount != 3 || wrong.Events[0].Kind != "reviewed" || wrong.Events[0].Gate != "design" || wrong.Events[0].By != "pat@example.org" {
		t.Errorf("review-by-non-reviewer: the reviews are %+v and the newest event %+v", wrong.Reviews, wrong.Events[0])
	}
	// A status that passes a reviewed junction nothing accepts.
	unreviewed := at("status-unreviewed-gate").Reviews
	if len(unreviewed) != 1 || unreviewed[0] != (view.Review{Gate: "design", Reviewer: "olive@example.org", Effect: "none"}) {
		t.Errorf("status-unreviewed-gate: the reviews are %+v; want design with the effect none", unreviewed)
	}
	accepted := at("model-mismatch")
	if len(accepted.Reviews) != 1 || accepted.Reviews[0].Effect != "accepts" || accepted.Reviews[0].Commit == nil || accepted.Reviews[0].By != "pat@example.org" {
		t.Errorf("model-mismatch: the reviews are %+v", accepted.Reviews)
	}

	var readings []string
	for _, m := range accepted.Models {
		readings = append(readings, m.Gate+" "+m.Stated+" "+m.Trailer+" "+m.Reading)
		if len(m.Commit) != 40 {
			t.Errorf("model-mismatch: a model check names the commit %q", m.Commit)
		}
	}
	if want := []string{"design claude-fable claude-sonnet-5 differs", "release  claude-fable-5-1 unstated"}; !reflect.DeepEqual(readings, want) {
		t.Errorf("model-mismatch: the models read %v; want %v", readings, want)
	}
	readings = nil
	for _, m := range at("model-trailer-missing").Models {
		readings = append(readings, m.Gate+" "+m.Stated+" "+m.Trailer+" "+m.Reading)
	}
	if want := []string{"design claude-fable  exempt", "design claude-fable  missing"}; !reflect.DeepEqual(readings, want) {
		t.Errorf("model-trailer-missing: the models read %v; want %v", readings, want)
	}

	// c07d: five events of seven, newest first, one from the subproject.
	f, hashes := station(t, "main")
	firmware := labels(t, "weather-station.firmware")
	image := Task(f, Query{Task: "c07d", Level: Provenance})
	if image.EventCount != 7 || len(image.Events) != 5 || len(image.Models) != 0 {
		t.Fatalf("c07d holds %d events of %d and the models %+v", len(image.Events), image.EventCount, image.Models)
	}
	var rows []string
	for _, e := range image.Events {
		row := e.Date + " " + e.Kind + " " + e.By
		if e.Subproject != nil {
			row += " from " + e.Subproject.URL + " " + e.Subproject.Task
		}
		rows = append(rows, row)
	}
	want := []string{
		"2026-09-26 pin ben@example.org", "2026-09-26 reviewed ben@example.org", "2026-09-26 status ben@example.org",
		"2026-09-17 status ben@example.org from firmware f1a0", "2026-09-16 task ben@example.org",
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("the events of c07d are %v; want %v", rows, want)
	}
	if pin := image.Events[0]; pin.Commit != hashes["W11"] || pin.Pin == nil || *pin.Pin != (view.PinMove{URL: "firmware", New: firmware["F2"]}) || pin.Recorded != nil || pin.Gate != "" {
		t.Errorf("the pin event of c07d is %+v with %+v", pin, pin.Pin)
	}
	if reviewed := image.Events[1]; reviewed.Gate != "mockup" || reviewed.Pin != nil || reviewed.Recorded != nil {
		t.Errorf("the reviewed event of c07d is %+v", reviewed)
	}
	if sub := image.Events[3]; sub.Commit != firmware["F2"] || sub.Recorded == nil || *sub.Recorded != (view.EventStatus{Gate: "design", State: "nominal", Note: "Sleep scheduler in progress"}) {
		t.Errorf("the subproject's event is %+v with %+v", sub, sub.Recorded)
	}
	if text := image.Commands; len(text) != 1 || text[0].Text != "git log --format='%H %as %ae %s' "+hashes["W13"]+" -- .tableaux/tasks/c07d.yaml .tableaux/status/c07d.yaml firmware" {
		t.Errorf("the command of c07d is %+v", text)
	}
	// Fewer than five events stand whole, and a reaffirmation is one of them.
	board := Task(f, Query{Task: "9f31", Level: Provenance})
	if board.EventCount != 6 || len(board.Events) != 5 || board.Events[0].Kind != "reaffirmed" || board.Events[0].Commit != hashes["W13"] {
		t.Errorf("9f31 holds %d events of %d, the newest %+v", len(board.Events), board.EventCount, board.Events[0])
	}
	if gateway := Task(f, Query{Task: "7b2e", Level: Provenance}); gateway.EventCount != 1 || len(gateway.Events) != 1 || gateway.Events[0].Kind != "task" {
		t.Errorf("7b2e holds the events %+v", gateway.Events)
	}
}

// encoded returns a view's data as an envelope's JSON writes it: a
// two-space indent, no HTML escaping and one final newline.
func encoded(t testing.TB, data any) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
