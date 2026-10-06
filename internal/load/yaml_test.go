package load

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// codes renders diagnostics one a line as broken.expected.txt writes them:
// position, severity, code.
func codes(diagnostics []model.Diagnostic) []string {
	var lines []string
	for _, d := range diagnostics {
		pos := d.Pos.File
		if d.Pos.Line > 0 {
			pos += fmt.Sprintf(":%d", d.Pos.Line)
		}
		if d.Pos.Col > 0 {
			pos += fmt.Sprintf(":%d", d.Pos.Col)
		}
		lines = append(lines, pos+" "+d.Severity.String()+" "+d.Code)
	}
	return lines
}

// scalar reads `v: <written>` and returns the value of v.
func scalar(t *testing.T, written string) *model.Value {
	t.Helper()
	root, parsed, diagnostics := readYAML("x.yaml", []byte("v: "+written+"\n"))
	if !parsed || len(diagnostics) > 0 {
		t.Fatalf("v: %s: parsed %v, diagnostics %v", written, parsed, codes(diagnostics))
	}
	v := root.Get("v")
	if v == nil {
		t.Fatalf("v: %s: no value", written)
	}
	return v
}

// T1: plain scalars take their YAML 1.2 kind, by every row of typing.yaml,
// and the same text in quotes, as a block scalar and under !!str is a string.
func TestPlainScalarKinds(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "typing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	table, parsed, diagnostics := readYAML("typing.yaml", data)
	if !parsed || len(diagnostics) > 0 || table == nil {
		t.Fatalf("typing.yaml: parsed %v, diagnostics %v", parsed, codes(diagnostics))
	}
	if len(table.Items) != 32 {
		t.Fatalf("typing.yaml holds %d rows, want 32", len(table.Items))
	}
	for _, row := range table.Items {
		text, want := row.Get("text").Text, row.Get("kind").Text
		plain := scalar(t, text)
		if plain.Kind.String() != want || plain.Read != plain.Kind || plain.Quoted || plain.Text != text {
			t.Errorf("plain %q: kind %s, read %s, quoted %v, text %q; want %s", text, plain.Kind, plain.Read, plain.Quoted, plain.Text, want)
		}
		quoted := map[string]string{
			"double quotes": `"` + text + `"`,
			"single quotes": `'` + text + `'`,
			"block scalar":  "|-\n  " + text,
			"!!str":         "!!str " + text,
		}
		for form, written := range quoted {
			v := scalar(t, written)
			if v.Kind != model.String || v.Read != model.String || !v.Quoted || v.Text != text {
				t.Errorf("%q in %s: kind %s, read %s, quoted %v, text %q; want the string", text, form, v.Kind, v.Read, v.Quoted, v.Text)
			}
		}
	}
}

// T4, beside the fixture: what else a file may hold that a Tableaux file does not use.
func TestYAMLFeatures(t *testing.T) {
	tests := []struct {
		name, text string
		parsed     bool
		want       []string
		plain      any
	}{
		{"an empty file", "", true, nil, nil},
		{"comments alone", "# nothing\n", true, nil, nil},
		{"a key that is a sequence", "? [a, b]\n: 1\nc: 2\n", true, []string{"x.yaml:1:3 error L3"}, map[string]any{"c": int64(2)}},
		{"a tag on a mapping", "a: !!map { b: 1 }\n", true, []string{"x.yaml:1:4 error L3"}, map[string]any{"a": map[string]any{"b": int64(1)}}},
		{"a tag of the file's own", "a: !thing 5\n", true, []string{"x.yaml:1:4 error L3"}, map[string]any{"a": int64(5)}},
		{"the tag !!str", "a: !!str 5\n", true, nil, map[string]any{"a": "5"}},
		{"a merge key", "<<: { a: 1 }\n", true, nil, map[string]any{"<<": map[string]any{"a": int64(1)}}},
		{"an anchor and an alias on mappings", "a: &x { b: 1 }\nc: *x\n", true, []string{"x.yaml:1:4 error L3", "x.yaml:2:4 error L3"},
			map[string]any{"a": map[string]any{"b": int64(1)}, "c": nil}},
		{"a key twice in two spellings", "a: 1\n\"a\": 2\n", true, []string{"x.yaml:2:1 error L2"}, map[string]any{"a": int64(1)}},
		{"a second document that does not parse", "a: 1\n---\n b: [\n", false, []string{"x.yaml:3 error L1"}, nil},
		{"a tab for indentation", "a:\n\tb: 1\n", false, []string{"x.yaml:2 error L1"}, nil},
	}
	for _, tt := range tests {
		root, parsed, diagnostics := readYAML("x.yaml", []byte(tt.text))
		if parsed != tt.parsed {
			t.Errorf("%s: parsed %v", tt.name, parsed)
		}
		if got := codes(diagnostics); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: diagnostics %v, want %v", tt.name, got, tt.want)
		}
		if got := root.Plain(); !reflect.DeepEqual(got, tt.plain) {
			t.Errorf("%s: Plain gives %#v, want %#v", tt.name, got, tt.plain)
		}
	}
}
