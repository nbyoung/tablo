package view

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// The keys the audit and each of its findings add at each level, in the
// order the data writes them.
const (
	auditGlance       = "legend errors warnings information most findings"
	auditDetail       = "silent"
	findingGlance     = "key rule severity demoted kind tasks gate act resolver"
	findingDetail     = "files message"
	findingProvenance = "sentence source facts commits commands"
	findingFacts      = "task junction status requirement authorisation linkage model"
)

// inOrder marshals a value and returns the keys of its JSON object as
// written, and each key's value.
func inOrder(t *testing.T, v any) ([]string, map[string]string) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil {
		t.Fatalf("%s is no object: %v", data, err)
	}
	var keys []string
	values := map[string]string{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key.(string))
		values[key.(string)] = string(raw)
	}
	return keys, values
}

// The audit at glance, at detail and at provenance, with every list empty.
func auditAt(level int) Audit {
	f := Finding{Tasks: []TaskRef{}}
	a := Audit{}
	if level > 0 {
		f.FindingDetail = &FindingDetail{Files: []string{}}
		a.AuditDetail = &AuditDetail{Silent: []Silence{}}
	}
	if level > 1 {
		f.FindingProvenance = &FindingProvenance{Facts: []FindingFacts{}, Commits: []Commit{}, Commands: []Command{}}
	}
	a.Findings = []Finding{f}
	return a
}

// T15: the audit is one flat object whose levels nest: each level writes its
// keys after those of the level below, in the order of the declarations, a
// list as an array and an absent fact as null.
func TestAuditKeysByLevel(t *testing.T) {
	cases := []struct {
		name           string
		level          int
		audit, finding string
	}{
		{"glance", 0, auditGlance, findingGlance},
		{"detail", 1, auditGlance + " " + auditDetail, findingGlance + " " + findingDetail},
		{"provenance", 2, auditGlance + " " + auditDetail, findingGlance + " " + findingDetail + " " + findingProvenance},
	}
	lists := map[string]bool{"findings": true, "silent": true, "tasks": true, "files": true, "facts": true, "commits": true, "commands": true}
	for _, c := range cases {
		a := auditAt(c.level)
		for _, part := range []struct {
			v    any
			want string
		}{{a, c.audit}, {a.Findings[0], c.finding}} {
			keys, values := inOrder(t, part.v)
			if want := strings.Fields(part.want); !reflect.DeepEqual(keys, want) {
				t.Errorf("at %s the keys are %v, want %v", c.name, keys, want)
			}
			for key, value := range values {
				if lists[key] && !strings.HasPrefix(value, "[") {
					t.Errorf("at %s the list %s is %s", c.name, key, value)
				}
			}
		}
		if _, values := inOrder(t, a); values["most"] != "null" {
			t.Errorf("at %s an audit that asks nothing of anyone writes most as %s", c.name, values["most"])
		}
	}
	keys, values := inOrder(t, FindingFacts{Task: "9f31"})
	if want := strings.Fields(findingFacts); !reflect.DeepEqual(keys, want) {
		t.Errorf("the facts of a task hold %v, want %v", keys, want)
	}
	for _, key := range keys[1:] {
		if values[key] != "null" {
			t.Errorf("a fact the finding does not carry is %s at %s, want null", values[key], key)
		}
	}
	if keys, _ := inOrder(t, Resolving{}); !reflect.DeepEqual(keys, []string{"email", "count"}) {
		t.Errorf("most holds %v", keys)
	}
	if keys, _ := inOrder(t, Silence{}); !reflect.DeepEqual(keys, []string{"key", "rule", "kind"}) {
		t.Errorf("a silence holds %v", keys)
	}
}

// T15: every field of the audit's types carries a key and no option, and no
// type holds a worded action or a worded fact (decision 8).
func TestAuditTags(t *testing.T) {
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
			case options != "" && (ty != reflect.TypeOf(Junction{}) || field.Name != "From"):
				t.Errorf("%s.%s carries the option %q", ty.Name(), field.Name, options)
			}
		}
	}
	walk(reflect.TypeOf(Audit{}))
	for _, ty := range []reflect.Type{reflect.TypeOf(Finding{}), reflect.TypeOf(FindingDetail{}), reflect.TypeOf(FindingProvenance{})} {
		if !seen[ty] {
			t.Errorf("the walk does not reach %s", ty.Name())
		}
		for i := 0; i < ty.NumField(); i++ {
			if name := ty.Field(i).Name; name == "Action" || name == "Actions" {
				t.Errorf("%s holds a worded %s", ty.Name(), name)
			}
		}
	}
	facts := reflect.TypeOf(FindingFacts{})
	for i := 1; i < facts.NumField(); i++ {
		if field := facts.Field(i); field.Type.Kind() != reflect.Pointer || field.Type.Elem().Kind() != reflect.Struct {
			t.Errorf("the fact %s is no typed fact: %s", field.Name, field.Type)
		}
	}
}

// auditSchema compiles the definition of a name in envelope.schema.yaml, at
// the module's root.
func auditSchema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "envelope.schema.yaml"))
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
	s, err := c.Compile("tablo/envelope.schema.yaml#/$defs/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// instance reads JSON as the schema's validator takes it.
func instance(t *testing.T, data []byte) any {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

// T15: the data of weather-station at provenance decodes into Audit with no
// unknown key, encodes to an equal JSON value and holds to the schema.
func TestAuditGolden(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "audit.provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var a Audit
	dec := json.NewDecoder(bytes.NewReader(golden))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		t.Fatalf("the golden data does not decode: %v", err)
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(golden, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the golden data encodes to another value:\n%s", encoded)
	}
	if err := auditSchema(t, "view_audit").Validate(instance(t, encoded)); err != nil {
		t.Errorf("the golden data does not hold to the schema: %v", err)
	}
	if a.Errors != 0 || a.Warnings != 2 || a.Information != 32 || len(a.Findings) != 2 || a.Most == nil || a.Most.Count != 2 {
		t.Errorf("the golden data counts %d, %d and %d over %d rows, most %+v", a.Errors, a.Warnings, a.Information, len(a.Findings), a.Most)
	}
	for _, f := range a.Findings {
		if f.FindingDetail == nil || f.FindingProvenance == nil || len(f.Facts) != len(f.Tasks) {
			t.Errorf("the finding %s does not reach provenance with a fact per task", f.Key)
		}
	}
	for level := 0; level < 3; level++ {
		data, err := json.Marshal(auditAt(level))
		if err != nil {
			t.Fatal(err)
		}
		// An empty finding has no key, act or severity: the schema refuses it.
		if err := auditSchema(t, "view_audit").Validate(instance(t, data)); err == nil {
			t.Errorf("the schema accepts a finding with no key at level %d", level)
		}
	}
}

// T15: the schema closes the three sets of words (decision 2, 493e decision
// 11): it accepts each severity, each of the twelve acts and each key, and
// refuses advance, a fourth kind and a fourth severity.
func TestAuditSchemaClosesTheWords(t *testing.T) {
	s := auditSchema(t, "view_finding")
	finding := func(key, severity, act string) []byte {
		f := Finding{Key: key, Severity: severity, Act: act, Tasks: []TaskRef{}}
		data, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	accepts := func(key, severity, act string) bool {
		return s.Validate(instance(t, finding(key, severity, act))) == nil
	}
	for _, severity := range []string{"error", "warning", "information"} {
		if !accepts("R9", severity, "await") {
			t.Errorf("the schema refuses the severity %s", severity)
		}
	}
	acts := strings.Fields("authorise review reaffirm record_handoff clear_handoff revise move_pin checkout await state_commit raise_language none")
	if len(acts) != 12 {
		t.Fatalf("%d acts", len(acts))
	}
	for _, act := range acts {
		if !accepts("H2", "warning", act) {
			t.Errorf("the schema refuses the act %s", act)
		}
	}
	for _, key := range []string{"P5", "R13", "J17", "S11", "H6", "proposed", "stale", "sole_review"} {
		if !accepts(key, "warning", "none") {
			t.Errorf("the schema refuses the key %s", key)
		}
	}
	for _, c := range [][3]string{
		{"R9", "warning", "advance"},
		{"R14", "information", "move_on"},
		{"R9", "warning", ""},
		{"self_review", "information", "none"},
		{"", "warning", "revise"},
		{"r9", "warning", "revise"},
		{"R9", "fatal", "revise"},
	} {
		if accepts(c[0], c[1], c[2]) {
			t.Errorf("the schema accepts the key %q, the severity %q and the act %q", c[0], c[1], c[2])
		}
	}
	silence := auditSchema(t, "view_silence")
	if err := silence.Validate(instance(t, []byte(`{"key":"stale","rule":"","kind":"A stale status"}`))); err != nil {
		t.Errorf("the schema refuses a silent kind: %v", err)
	}
	if err := silence.Validate(instance(t, []byte(`{"key":"orphan","rule":"","kind":"An orphan"}`))); err == nil {
		t.Error("the schema accepts a fourth kind among the silent")
	}
}
