package views

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
)

// envelopeSchema compiles envelope.schema.yaml, the one schema of tablo/1,
// from the module's root. The root package embeds the same file; a test of
// this package cannot import it once snapshot.go joins the views to it.
func envelopeSchema(t testing.TB) *jsonschema.Schema {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "envelope.schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	asJSON, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	res, err := jsonschema.UnmarshalJSON(bytes.NewReader(asJSON))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("tablo/envelope.schema.yaml", res); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("tablo/envelope.schema.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The forms the envelope's schema asks of a task id and of an email.
var (
	idForm    = regexp.MustCompile(`^[0-9a-f]{4}$`)
	emailForm = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)
)

// envelope wraps a view's data as the plumbing command's envelope does, with
// the keys the schema asks of a view: the source, the project, an observer
// and the resolved parameters. data is the view's JSON.
func envelope(t testing.TB, command string, q Query, data []byte) any {
	t.Helper()
	params := map[string]any{"task": nil, "role": "observer", "level": levels[q.Level]}
	if idForm.MatchString(q.Task) {
		params["task"] = q.Task
	}
	if command != "gates" {
		params["person"] = nil
		if emailForm.MatchString(q.Person) {
			params["person"] = q.Person
		}
	}
	if command == "authority" {
		params["proposed"] = q.Proposed
	}
	doc := map[string]any{
		"schema":  "tablo/1",
		"command": command,
		"source": map[string]any{
			"ref": "main", "worktree": false, "commit": nil, "date": nil, "from": nil,
			"trunk": map[string]any{"name": nil, "resolved": "undetermined"}, "on_trunk": false,
		},
		"project":     nil,
		"viewer":      map[string]any{"email": nil, "roles": []string{"observer"}},
		"params":      params,
		"data":        json.RawMessage(data),
		"diagnostics": []any{},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

// queries returns the queries a test runs every view of a project with, at
// one level: each view of the whole project, the gate and task definitions
// of every task, and the tree views of every parent and of every person.
func queries(f *derive.Facts, level Level) map[string][]Query {
	out := map[string][]Query{
		"gates":      {{Level: level}},
		"authority":  {{Task: f.Root(), Level: level}, {Task: f.Root(), Level: level, Proposed: true}},
		"assignment": {{Task: f.Root(), Level: level}},
	}
	people := map[string]bool{}
	for _, id := range f.Order() {
		out["gates"] = append(out["gates"], Query{Task: id, Level: level})
		out["task"] = append(out["task"], Query{Task: id, Level: level})
		if email := f.Project().Tasks[id].Assignee.V; email != "" && !people[email] {
			people[email] = true
			out["task"] = append(out["task"], Query{Task: id, Person: email, Level: level})
			out["authority"] = append(out["authority"], Query{Task: f.Root(), Person: email, Level: level})
			out["assignment"] = append(out["assignment"], Query{Task: f.Root(), Person: email, Level: level})
		}
		if !f.Leaf(id) && id != f.Root() {
			out["authority"] = append(out["authority"], Query{Task: id, Level: level})
			out["assignment"] = append(out["assignment"], Query{Task: id, Level: level})
		}
	}
	return out
}

// T3: the schema holds. Every view of every repository the corpus builds
// that the Derivation does not refuse, at three levels, in an envelope,
// validates against the schema.
func TestSchemaHolds(t *testing.T) {
	s := envelopeSchema(t)
	projects, envelopes := 0, 0
	for _, name := range built(t) {
		f := derived(t, load.Options{}, builtAt(t, name), "HEAD", "")
		if f.Refused() != nil {
			continue
		}
		projects++
		for level := range levels {
			for command, list := range queries(f, level) {
				for _, q := range list {
					envelopes++
					if err := s.Validate(envelope(t, command, q, encoded(t, viewOf(f, command, q)))); err != nil {
						t.Errorf("%s: %s %+v does not validate: %v", name, command, q, err)
					}
				}
			}
		}
	}
	if projects < 90 || envelopes < 3000 {
		t.Errorf("%d envelopes of %d projects validate; the corpus builds more", envelopes, projects)
	}
	t.Logf("%d envelopes of %d projects validate", envelopes, projects)
}

// T3: the schema types the closed sets of words as enums (decision 11): a
// word outside a set fails.
func TestSchemaClosesTheWords(t *testing.T) {
	s := envelopeSchema(t)
	f := derived(t, load.Options{}, builtAt(t, "weather-station"), "main", "")
	q := Query{Task: "c07d", Level: Provenance}
	valid := encoded(t, Task(f, q))
	if err := s.Validate(envelope(t, "task", q, valid)); err != nil {
		t.Fatalf("the task definition of c07d does not validate: %v", err)
	}
	for _, change := range [][2]string{
		{`"kind": "snapshot"`, `"kind": "pinned"`},               // a status's kind, and its origin's
		{`"kind": "not_applicable"`, `"kind": "not-applicable"`}, // a junction's kind
		{`"way": "change"`, `"way": "commit"`},                   // the way a deciding commit accepts
		{`"why": ""`, `"why": "no history"`},                     // the reason a fact is undetermined
	} {
		if !bytes.Contains(valid, []byte(change[0])) {
			t.Fatalf("the task definition of c07d holds no %s", change[0])
		}
		broken := bytes.Replace(valid, []byte(change[0]), []byte(change[1]), 1)
		if err := s.Validate(envelope(t, "task", q, broken)); err == nil {
			t.Errorf("the schema accepts %s", change[1])
		}
	}
	mismatch := derived(t, load.Options{}, builtAt(t, "model-mismatch"), "main", "")
	for _, id := range mismatch.Order() {
		q := Query{Task: id, Level: Provenance}
		data := encoded(t, Task(mismatch, q))
		if !bytes.Contains(data, []byte(`"reading": "differs"`)) {
			continue
		}
		if err := s.Validate(envelope(t, "task", q, bytes.Replace(data, []byte(`"reading": "differs"`), []byte(`"reading": "mismatch"`), 1))); err == nil {
			t.Error("the schema accepts the reading mismatch")
		}
		return
	}
	t.Error("model-mismatch holds no reading that differs")
}

// viewOf returns the data of the view a command names.
func viewOf(f *derive.Facts, command string, q Query) any {
	switch command {
	case "gates":
		return Gates(f, q)
	case "task":
		return Task(f, q)
	case "authority":
		return Authority(f, q)
	}
	return Assignment(f, q)
}
