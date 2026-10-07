package validate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// mutations calls emit with each mutation of a YAML file: at every node, the
// value emptied, retyped, nulled and made no number; at every mapping, a key
// added and each key dropped; at every sequence, each item dropped; and the
// file itself emptied.
func mutations(t *testing.T, data []byte, emit func([]byte)) {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) != 1 {
		t.Fatalf("a file to mutate does not parse: %v", err)
	}
	out := func() {
		text, err := yaml.Marshal(&doc)
		if err != nil {
			t.Fatal(err)
		}
		emit(text)
	}
	scalar := func(tag, value string) yaml.Node {
		return yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		saved := *n
		try := func(changed yaml.Node) {
			*n = changed
			out()
			*n = saved
		}
		// Emptied.
		if saved.Kind == yaml.ScalarNode {
			try(yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle})
		} else {
			try(yaml.Node{Kind: saved.Kind, Tag: saved.Tag, Style: yaml.FlowStyle})
		}
		// Retyped: a scalar becomes a list of itself, an integer and a
		// boolean; a collection becomes a string.
		if saved.Kind == yaml.ScalarNode {
			item := saved
			try(yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle, Content: []*yaml.Node{&item}})
			try(scalar("!!int", "7"))
			try(scalar("!!bool", "true"))
		} else {
			try(scalar("!!str", "words"))
		}
		// Nulled, and made what is no number.
		try(scalar("!!null", "null"))
		try(scalar("!!float", ".nan"))
		switch saved.Kind {
		case yaml.MappingNode:
			key, value := scalar("!!str", "zzz"), scalar("!!str", "added")
			added := saved
			added.Content = append(append([]*yaml.Node{}, saved.Content...), &key, &value)
			try(added)
			for i := 0; i+1 < len(saved.Content); i += 2 {
				dropped := saved
				dropped.Content = append(append([]*yaml.Node{}, saved.Content[:i]...), saved.Content[i+2:]...)
				try(dropped)
			}
			for i := 1; i < len(saved.Content); i += 2 {
				walk(saved.Content[i])
			}
		case yaml.SequenceNode:
			for i := range saved.Content {
				dropped := saved
				dropped.Content = append(append([]*yaml.Node{}, saved.Content[:i]...), saved.Content[i+1:]...)
				try(dropped)
			}
			for _, item := range saved.Content {
				walk(item)
			}
		}
	}
	walk(doc.Content[0])
	emit(nil)
}

// kindOf returns the file kind of a path below .tableaux.
func kindOf(path string) fileKind {
	switch {
	case path == "version.yaml":
		return versionFile
	case path == "gates.yaml":
		return gatesFile
	case strings.HasPrefix(path, "tasks/"):
		return taskFile
	}
	return statusFile
}

// TestSchemaStaysTheJudge is T5: over the mutations of each file of
// weather-station and junction-kinds, whenever the schema rejects the file,
// Files gives an error in it, and no run panics.
func TestSchemaStaysTheJudge(t *testing.T) {
	if testing.Short() {
		t.Skip("the mutations take a load each")
	}
	total, invalid := 0, 0
	for _, name := range []string{"weather-station", "junction-kinds"} {
		repo := copyOf(t, name)
		loader := load.New(load.Options{CacheDir: t.TempDir()})
		reload := func() *model.Project {
			p, err := loader.Load(t.Context(), load.Source{Dir: repo})
			if err != nil {
				t.Fatal(err)
			}
			return p
		}
		start := reload()
		for _, d := range Files(start) {
			if d.Severity == model.Error {
				t.Fatalf("%s: the copy starts with %s", name, where(d))
			}
		}
		var paths []string
		for _, file := range start.Files {
			paths = append(paths, file.Path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			full := filepath.Join(repo, ".tableaux", filepath.FromSlash(path))
			original, err := os.ReadFile(full)
			if err != nil {
				t.Fatal(err)
			}
			mutations(t, original, func(text []byte) {
				if err := os.WriteFile(full, text, 0o644); err != nil {
					t.Fatal(err)
				}
				p := reload()
				found := Files(p)
				total++
				var file *model.File
				for _, f := range p.Files {
					if f.Path == path {
						file = f
					}
				}
				if file == nil {
					t.Fatalf("%s %s: the Loader lists no such file", name, path)
				}
				if schemaSet().file[kindOf(path)].Validate(finite(file.Root.Plain())) == nil {
					return
				}
				invalid++
				for _, d := range found {
					if d.Severity == model.Error && d.Pos.File == path {
						return
					}
				}
				t.Errorf("%s %s: the schema rejects the file and Files gives no error in it:\n%s\ngot %v", name, path, text, places(found))
			})
			if err := os.WriteFile(full, original, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("%d mutations, %d of them invalid by the schema", total, invalid)
	if total < 1000 || invalid < total/2 {
		t.Errorf("%d mutations, %d invalid: too few to judge", total, invalid)
	}
}
