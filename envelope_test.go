package tablo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// golden reads a draft the design lands in internal/cli/testdata.
func golden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("internal", "cli", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func ptr[T any](v T) *T { return &v }

// weatherSource is the source of the goldens: main at its commit, on the trunk.
func weatherSource(commit, date string) *Source {
	return &Source{
		Ref:     "main",
		Commit:  &commit,
		Date:    &date,
		Trunk:   SourceTrunk{Name: ptr("main"), Resolved: "stated"},
		OnTrunk: true,
	}
}

func weatherProject() *ProjectInfo {
	return &ProjectInfo{Tableaux: "0.3.1", Root: "a1c0", Title: "Weather station", Owner: "ada@example.org"}
}

// tableauEnvelope builds the envelope of tableau.json: a view whose data is
// abridged from prototype 886d and fixes the encoding only.
func tableauEnvelope() *Envelope {
	type column struct {
		Gate   string `json:"gate"`
		Symbol string `json:"symbol"`
	}
	type folded struct {
		Side  string `json:"side"`
		From  string `json:"from"`
		To    string `json:"to"`
		Count int    `json:"count"`
	}
	type status struct {
		Gate     string `json:"gate"`
		State    string `json:"state"`
		Reason   string `json:"reason"`
		Note     string `json:"note"`
		Date     string `json:"date"`
		Recorder string `json:"recorder"`
		Commit   string `json:"commit"`
	}
	type cell struct {
		Gate    string `json:"gate"`
		Kind    string `json:"kind"`
		Symbols string `json:"symbols"`
	}
	type row struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Depth  int    `json:"depth"`
		Status status `json:"status"`
		Cells  []cell `json:"cells"`
	}
	commit := "edb30d2baa5ea3c7e3fb8eef8ac6cce6e145221a"
	return &Envelope{
		Schema:  SchemaVersion,
		Command: "tableau",
		Source:  weatherSource(commit, "2026-09-28"),
		Project: weatherProject(),
		Viewer:  &Viewer{Email: ptr("ada@example.org"), Roles: []Role{RoleOwner, RoleAuthority, RoleAssignee, RoleContributor}},
		Params:  Resolved{View: Tableau, Params: Params{Window: ptr(1), Level: LevelDetail}},
		Data: struct {
			Columns []column `json:"columns"`
			Folded  []folded `json:"folded"`
			Rows    []row    `json:"rows"`
		}{
			Columns: []column{{"mockup", "📌"}, {"function", "⚙️"}},
			Folded:  []folded{{"after", "integrate", "release", 0}},
			Rows: []row{{
				ID: "9f31", Title: "Sensor board", Depth: 2,
				Status: status{"function", "stalled", "blocked", "Barometer ICs on 14-week backorder", "2026-09-28", "ada@example.org", commit},
				Cells:  []cell{{"mockup", "blank", ""}, {"function", "status", "🔴⛔"}},
			}},
		},
	}
}

func versionEnvelope() *Envelope {
	return &Envelope{
		Schema:  SchemaVersion,
		Command: "version",
		Data: struct {
			Version  string `json:"version"`
			Tableaux string `json:"tableaux"`
			Schema   string `json:"schema"`
		}{Version, fmt.Sprintf("%d.%d", AcceptedMajor, AcceptedMinor), SchemaVersion},
	}
}

func trailerEnvelope() *Envelope {
	line, _ := Trailer{Kind: "reviewed", Task: "9f31", Gate: "validate"}.Line()
	return &Envelope{
		Schema:  SchemaVersion,
		Command: "trailer",
		Source:  weatherSource("edb30d2baa5ea3c7e3fb8eef8ac6cce6e145221a", "2026-09-28"),
		Project: weatherProject(),
		Data: struct {
			Trailer string  `json:"trailer"`
			Kind    string  `json:"kind"`
			Task    *string `json:"task"`
			Gate    *string `json:"gate"`
			Model   *string `json:"model"`
		}{line, "reviewed", ptr("9f31"), ptr("validate"), nil},
	}
}

func validateEnvelope() *Envelope {
	return &Envelope{
		Schema:  SchemaVersion,
		Command: "validate",
		Source:  weatherSource("dc2f478ae7f54f4a4652a4b81c4ff2da6a0437ee", "2026-09-01"),
		Project: &ProjectInfo{Tableaux: "0.3.1", Root: "e4a1", Title: "Base", Owner: "olive@example.org"},
		Data: struct {
			Valid       bool `json:"valid"`
			Errors      int  `json:"errors"`
			Warnings    int  `json:"warnings"`
			Information int  `json:"information"`
		}{false, 1, 0, 0},
		Diagnostics: []Diagnostic{{
			Severity: "error", Code: "T4", Path: ".tableaux/tasks/b2c9.yaml", Line: 4, Col: 11,
			Task: "b2c9", Message: `assignee "not-an-email" is not an email address`,
		}},
	}
}

// TestEncodingsAreExact is the byte-for-byte half of T5.
func TestEncodingsAreExact(t *testing.T) {
	tests := []struct {
		name string
		env  *Envelope
		json string
		yaml string
	}{
		{"tableau", tableauEnvelope(), "tableau.json", "tableau.yaml"},
		{"version", versionEnvelope(), "version.json", ""},
		{"trailer", trailerEnvelope(), "trailer.json", ""},
		{"validate", validateEnvelope(), "validate.json", ""},
	}
	for _, tc := range tests {
		want := golden(t, tc.json)
		if got := tc.env.JSON(); !bytes.Equal(got, want) {
			t.Errorf("%s: JSON differs from %s:\n%s", tc.name, tc.json, got)
		}
		if tc.yaml != "" {
			if got, want := tc.env.YAML(), golden(t, tc.yaml); !bytes.Equal(got, want) {
				t.Errorf("%s: YAML differs from %s:\n%s", tc.name, tc.yaml, got)
			}
			if got, want := yamlOfJSON(golden(t, tc.json)), golden(t, tc.yaml); !bytes.Equal(got, want) {
				t.Errorf("%s: YAML from %s differs from %s:\n%s", tc.name, tc.json, tc.yaml, got)
			}
		}
	}
}

// plain converts a decoded value so that JSON and YAML readings compare equal:
// every number is a float64 and every mapping a map[string]any.
func plain(t *testing.T, v any) any {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		m := map[string]any{}
		for k, e := range x {
			m[k] = plain(t, e)
		}
		return m
	case []any:
		s := []any{}
		for _, e := range x {
			s = append(s, plain(t, e))
		}
		return s
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case uint64:
		return float64(x)
	}
	return v
}

// TestYAMLReadsBack is the read-back half of T5: yaml.v3 reads every YAML
// equal to the JSON, with the strings that YAML would otherwise retype.
func TestYAMLReadsBack(t *testing.T) {
	hard := &Envelope{
		Schema:  SchemaVersion,
		Command: "version",
		Data: map[string]any{
			"id":         "07e0",
			"thousand":   "1000",
			"word":       "yes",
			"null":       "null",
			"tilde":      "~",
			"empty":      "",
			"colon":      "key: value",
			"hash":       "# not a comment",
			"dash":       "- item",
			"quote":      `say "hi" \ done`,
			"html":       "<a href=\"x\">&amp;</a>",
			"newline":    "line\nbreak",
			"tab":        "a\tb",
			"control":    "bell\x07 and nul\x00",
			"joiner":     "👩‍💻 and \u200d and \u2028",
			"symbols":    "⚙️📌🔴⛔",
			"yes":        1,
			"y":          "y",
			"On":         true,
			"with space": "k",
			"1st":        "digit first",
			"lower_case": []string{},
			"empties":    map[string]any{"list": []int{}, "map": map[string]int{}},
			"nested":     [][]any{{1, 2.5, nil}, {}, {"x", true}},
			"records":    []map[string]any{{"a": 1, "b": []string{"p", "q"}}, {"c": map[string]any{"d": "e"}}},
			"number":     1e21,
			"negative":   -7,
		},
	}
	docs := map[string]*Envelope{"hard": hard, "tableau": tableauEnvelope(), "validate": validateEnvelope(), "trailer": trailerEnvelope()}
	for name, env := range docs {
		var fromJSON, fromYAML any
		if err := json.Unmarshal(env.JSON(), &fromJSON); err != nil {
			t.Fatalf("%s: JSON: %v", name, err)
		}
		y := env.YAML()
		if err := yaml.Unmarshal(y, &fromYAML); err != nil {
			t.Fatalf("%s: yaml.v3: %v\n%s", name, err, y)
		}
		if got, want := plain(t, fromYAML), plain(t, fromJSON); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: YAML reads back as %v, want %v\n%s", name, got, want, y)
		}
	}
	out := string(hard.YAML())
	for _, want := range []string{`  id: "07e0"`, `  thousand: "1000"`, `  word: "yes"`, `  "yes": 1`, `  "y": "y"`, `  "On": true`, `  "with space": "k"`, `  "1st": "digit first"`, `  lower_case: []`, `  empties:` + "\n" + `    list: []` + "\n" + `    map: {}`, `\u200d`} {
		if !strings.Contains(out, want) {
			t.Errorf("YAML lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `\U0001f469`) {
		t.Errorf("YAML escapes a character beyond the Basic Multilingual Plane:\n%s", out)
	}
	if !strings.Contains(out, "<a href=") {
		t.Errorf("YAML escapes HTML:\n%s", out)
	}
}

// TestJSONShape covers the nil lists, the empty parameters and the exit code.
func TestJSONShape(t *testing.T) {
	e := &Envelope{Schema: SchemaVersion, Command: "version", Viewer: &Viewer{}}
	got := string(e.JSON())
	for _, want := range []string{`"params": {}`, `"diagnostics": []`, `"roles": []`, `"data": null`, `"email": null`} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON lacks %s:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, "}\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("JSON does not end in one newline: %q", got[len(got)-4:])
	}
	if code := e.ExitCode(); code != 0 {
		t.Errorf("ExitCode = %d, want 0", code)
	}
	e.Diagnostics = []Diagnostic{{Severity: "warning", Code: "H1", Message: "m"}}
	if code := e.ExitCode(); code != 0 {
		t.Errorf("a warning gives ExitCode %d, want 0", code)
	}
	e.Diagnostics = append(e.Diagnostics, Diagnostic{Severity: "error", Code: "T4", Message: "m"})
	if code := e.ExitCode(); code != 1 {
		t.Errorf("an error gives ExitCode %d, want 1", code)
	}
}

// TestResolvedKeys checks the parameters each view marshals, in order.
func TestResolvedKeys(t *testing.T) {
	keys := map[View]string{
		Gates:      "task role level",
		Task:       "task person role level",
		Authority:  "task person proposed role level",
		Assignment: "task person role level",
		Queue:      "task person brief role level",
		Blockage:   "task person role level",
		Tableau:    "person window columns historical role level",
		Context:    "task person window columns historical role level",
		History:    "task person role level",
		Audit:      "task person stale now role level",
	}
	for v, want := range keys {
		raw, err := json.Marshal(Resolved{View: v})
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		if _, err := dec.Token(); err != nil {
			t.Fatal(err)
		}
		var got []string
		for dec.More() {
			k, _ := dec.Token()
			got = append(got, k.(string))
			var skip any
			if err := dec.Decode(&skip); err != nil {
				t.Fatal(err)
			}
			if skip != nil && got[len(got)-1] != "historical" && got[len(got)-1] != "proposed" {
				t.Errorf("%s: unset %s marshals as %v, want null", v, k, skip)
			}
		}
		if strings.Join(got, " ") != want {
			t.Errorf("%s marshals %v, want %s", v, got, want)
		}
	}
	for _, v := range []View{"", "validate", "trailer", "version"} {
		raw, _ := json.Marshal(Resolved{View: v, Params: Params{Task: "e9c6"}})
		if string(raw) != "{}" {
			t.Errorf("command %q marshals %s, want {}", v, raw)
		}
	}
	full := Resolved{View: Audit, Params: Params{Task: "e9c6", Stale: 7, Now: "2026-10-06", Role: RoleOwner, Level: LevelGlance}}
	raw, _ := json.Marshal(full)
	if want := `{"task":"e9c6","person":null,"stale":7,"now":"2026-10-06","role":"owner","level":"glance"}`; string(raw) != want {
		t.Errorf("audit marshals %s, want %s", raw, want)
	}
	q := Resolved{View: Queue, Params: Params{Brief: &Brief{Task: "e9c6", Gate: "mockup"}}}
	raw, _ = json.Marshal(q)
	if want := `{"task":null,"person":null,"brief":{"task":"e9c6","gate":"mockup"},"role":null,"level":null}`; string(raw) != want {
		t.Errorf("queue marshals %s, want %s", raw, want)
	}
}

// schema compiles EnvelopeSchema.
func schema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	var doc any
	if err := yaml.Unmarshal(EnvelopeSchema, &doc); err != nil {
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

func validates(t *testing.T, s *jsonschema.Schema, doc any) error {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return s.Validate(inst)
}

func decodeMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestSchemaHolds is T6: every envelope the tests build validates against
// EnvelopeSchema, and seven broken ones fail.
func TestSchemaHolds(t *testing.T) {
	s := schema(t)
	audit := tableauEnvelope()
	audit.Command = "audit"
	audit.Params = Resolved{View: Audit, Params: Params{Stale: 7, Now: "2026-10-06", Level: LevelGlance}}
	queue := tableauEnvelope()
	queue.Command = "queue"
	queue.Params = Resolved{View: Queue, Params: Params{Person: "ada@example.org", Brief: &Brief{Task: "9f31", Gate: "mockup"}, Level: LevelProvenance}}
	history := tableauEnvelope()
	history.Command = "history"
	history.Source.From = &SourceFrom{Ref: "0704a09", Commit: strings.Repeat("a", 40)}
	history.Params = Resolved{View: History, Params: Params{Task: "9f31", Level: LevelDetail}}
	tree := validateEnvelope()
	tree.Source.Commit, tree.Source.Date = nil, nil
	undetermined := validateEnvelope()
	undetermined.Source.Trunk = SourceTrunk{Resolved: "undetermined"}
	undetermined.Source.OnTrunk = false
	noProject := validateEnvelope()
	noProject.Project = nil
	noProject.Diagnostics = []Diagnostic{{Severity: "error", Code: "P1", Message: "no .tableaux"}}
	noData := tableauEnvelope()
	noData.Data = nil
	noData.Diagnostics = validateEnvelope().Diagnostics
	good := map[string]*Envelope{
		"tableau": tableauEnvelope(), "version": versionEnvelope(), "trailer": trailerEnvelope(),
		"validate": validateEnvelope(), "audit": audit, "queue": queue, "history": history,
		"tree": tree, "undetermined": undetermined, "no project": noProject, "no data": noData,
	}
	for name, e := range good {
		if err := validates(t, s, decodeMap(t, e.JSON())); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, f := range []string{"tableau.json", "version.json", "trailer.json", "validate.json"} {
		if err := validates(t, s, decodeMap(t, golden(t, f))); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}

	broken := []struct {
		name   string
		base   func() *Envelope
		break_ func(m map[string]any)
	}{
		{"another schema", tableauEnvelope, func(m map[string]any) { m["schema"] = "tablo/2" }},
		{"a missing level", tableauEnvelope, func(m map[string]any) { delete(m["params"].(map[string]any), "level") }},
		{"a missing window", tableauEnvelope, func(m map[string]any) { delete(m["params"].(map[string]any), "window") }},
		{"a null viewer on a view", tableauEnvelope, func(m map[string]any) { m["viewer"] = nil }},
		{"an abbreviated commit", tableauEnvelope, func(m map[string]any) { m["source"].(map[string]any)["commit"] = "edb30d2" }},
		{"a column without a line", validateEnvelope, func(m map[string]any) {
			delete(m["diagnostics"].([]any)[0].(map[string]any), "line")
		}},
		{"an absolute path", validateEnvelope, func(m map[string]any) {
			m["diagnostics"].([]any)[0].(map[string]any)["path"] = "/home/ada/plan/.tableaux/tasks/b2c9.yaml"
		}},
	}
	for _, tc := range broken {
		m := decodeMap(t, tc.base().JSON())
		tc.break_(m)
		if err := validates(t, s, m); err == nil {
			t.Errorf("%s: the schema accepts it", tc.name)
		}
	}
}

// TestDiagnosticNamesWhatIsMalformed holds the schema to the Validator's
// design (8118, decision 12): a diagnostic's task and gate are strings of any
// form, since T1 reports the task leaf-two and G5 the gate Design, as the
// corpus states them. A task or a gate that is no string still fails.
func TestDiagnosticNamesWhatIsMalformed(t *testing.T) {
	s := schema(t)
	e := validateEnvelope()
	e.Diagnostics = []Diagnostic{
		{Severity: "error", Code: "T1", Path: ".tableaux/tasks/leaf-two.yaml", Task: "leaf-two", Message: "the file name leaf-two is not four lowercase hexadecimal digits"},
		{Severity: "error", Code: "G5", Path: ".tableaux/gates.yaml", Line: 5, Col: 12, Gate: "Design", Message: "gates.2.key Design does not match ^[a-z][a-z0-9_-]*$"},
		{Severity: "warning", Code: "H1", Commit: strings.Repeat("a", 40), Trailer: "Reviewed: 9f31 nowhere", Message: "9f31 names no task and nowhere names no gate"},
	}
	if err := validates(t, s, decodeMap(t, e.JSON())); err != nil {
		t.Errorf("leaf-two and Design: %v", err)
	}
	for _, field := range []string{"task", "gate"} {
		m := decodeMap(t, e.JSON())
		m["diagnostics"].([]any)[0].(map[string]any)[field] = 7
		if err := validates(t, s, m); err == nil {
			t.Errorf("the schema accepts a %s that is no string", field)
		}
	}
}

// TestVersionIsOneFact is T13 as far as it reads no command: SchemaVersion, the
// const in the schema, version.json and the version in code agree. The plain
// line joins them when the command lands.
func TestVersionIsOneFact(t *testing.T) {
	var doc struct {
		Properties struct {
			Schema struct {
				Const string `yaml:"const"`
			} `yaml:"schema"`
		} `yaml:"properties"`
	}
	if err := yaml.Unmarshal(EnvelopeSchema, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Properties.Schema.Const != SchemaVersion {
		t.Errorf("schema const = %q, SchemaVersion = %q", doc.Properties.Schema.Const, SchemaVersion)
	}
	var v struct {
		Schema string `json:"schema"`
		Data   struct {
			Version  string `json:"version"`
			Tableaux string `json:"tableaux"`
			Schema   string `json:"schema"`
		} `json:"data"`
	}
	if err := json.Unmarshal(golden(t, "version.json"), &v); err != nil {
		t.Fatal(err)
	}
	tableaux := fmt.Sprintf("%d.%d", AcceptedMajor, AcceptedMinor)
	if v.Schema != SchemaVersion || v.Data.Schema != SchemaVersion || v.Data.Version != Version || v.Data.Tableaux != tableaux {
		t.Errorf("version.json = %+v, want %s, %s, %s", v, SchemaVersion, Version, tableaux)
	}
	if !strings.Contains(string(EnvelopeSchema), "const: tablo/1") {
		t.Errorf("envelope.schema.yaml names no tablo/1")
	}
}
