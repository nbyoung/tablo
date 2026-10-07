package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/schemas"
)

// fileKind is which of the four files of the layout a schema judges.
type fileKind int

// The file kinds, each with a schema of its own.
const (
	versionFile fileKind = iota
	gatesFile
	taskFile
	statusFile
)

// schemaNames are the embedded schema files, by file kind.
var schemaNames = [...]string{"version", "gates", "task", "status"}

// entryDefs are the definitions of task.schema.yaml that judge one junction
// entry alone, by the kind of the entry.
var entryDefs = [...]string{model.Plain: "plain", model.Recursive: "recursive", model.NotApplicable: "not_applicable"}

// schemaBase is where the compiler holds the embedded schemas. The address is
// absolute so that the compiler resolves it without the working directory,
// and nothing ever fetches it.
const schemaBase = "https://tableaux.invalid/"

// compiled holds the four schemas and the three definitions of a junction
// entry. Nothing changes them once they compile.
type compiled struct {
	file  [len(schemaNames)]*jsonschema.Schema
	entry [len(entryDefs)]*jsonschema.Schema
}

// schemaSet compiles the embedded schemas once, with the formats asserted, as
// SYNTAX.md intends for email and uri-reference. A schema that does not
// compile is a fault of the module, so it panics.
var schemaSet = sync.OnceValue(func() *compiled {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	for _, name := range schemaNames {
		data, err := schemas.FS.ReadFile(name + ".schema.yaml")
		if err != nil {
			panic("validate: " + err.Error())
		}
		var doc any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			panic("validate: " + name + ".schema.yaml: " + err.Error())
		}
		// The compiler takes the numbers of a schema as JSON gives them.
		asJSON, err := json.Marshal(doc)
		if err != nil {
			panic("validate: " + name + ".schema.yaml: " + err.Error())
		}
		resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(asJSON))
		if err != nil {
			panic("validate: " + name + ".schema.yaml: " + err.Error())
		}
		if err := compiler.AddResource(schemaBase+name+".schema.yaml", resource); err != nil {
			panic("validate: " + name + ".schema.yaml: " + err.Error())
		}
	}
	set := &compiled{}
	for i, name := range schemaNames {
		set.file[i] = compiler.MustCompile(schemaBase + name + ".schema.yaml")
	}
	for i, def := range entryDefs {
		set.entry[i] = compiler.MustCompile(schemaBase + "task.schema.yaml#/$defs/" + def)
	}
	return set
})

// leaf is one failure of a schema, as the diagnostic the table makes of it,
// with the path of the value in the file.
type leaf struct {
	at         []string
	diagnostic model.Diagnostic
}

// checkFile validates a file against the schema of its kind and returns a
// leaf for each failure the table takes. id is the task of a task or status
// file. In a task file it leaves the junction entries to checkEntry.
func checkFile(k fileKind, file *model.File, id string) []leaf {
	return check(schemaSet().file[k], k, file, id, nil)
}

// checkEntry validates one junction entry alone against the schema of its
// kind, which is plain, recursive or not applicable.
func checkEntry(file *model.File, id string, j *model.Junction, k model.JunctionKind) []leaf {
	return check(schemaSet().entry[k], taskFile, file, id, []string{"junctions", j.Gate})
}

// check validates the value at prefix in a file and maps each leaf of the
// error tree to a rule, a position and a message.
func check(schema *jsonschema.Schema, k fileKind, file *model.File, id string, prefix []string) []leaf {
	err := schema.Validate(finite(file.Root.At(prefix...).Plain()))
	if err == nil {
		return nil
	}
	failure, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return nil
	}
	var found []leaf
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		_, names := e.ErrorKind.(*kind.PropertyNames)
		if len(e.Causes) > 0 && !names {
			for _, cause := range e.Causes {
				walk(cause)
			}
			return
		}
		at := append(append([]string{}, prefix...), e.InstanceLocation...)
		keyword := keywordLocation(e)
		// The three leaves the tables never see: a rule in code reports each.
		if names || strings.Contains(keyword, "/contains") {
			return
		}
		if k == taskFile && prefix == nil && len(at) >= 2 && at[0] == "junctions" {
			return
		}
		code, gate, ok := row(k, file.Root, at, keyword, e.ErrorKind)
		if !ok {
			return
		}
		for _, placed := range place(file, at, keyword, e.ErrorKind) {
			placed.Code, placed.Severity, placed.Task, placed.Gate = code, model.Error, id, gate
			found = append(found, leaf{at: at, diagnostic: placed})
		}
	}
	walk(failure)
	return found
}

// keywordLocation returns where in its schema file a leaf fails: the part of
// the schema's address after #, then the keyword.
func keywordLocation(e *jsonschema.ValidationError) string {
	_, location, _ := strings.Cut(e.SchemaURL, "#")
	if path := e.ErrorKind.KeywordPath(); len(path) > 0 {
		location += "/" + strings.Join(path, "/")
	}
	return location
}

// finite returns the tree with every float that is no number, .nan and .inf,
// replaced by one half: a number, and no integer, that the schema library
// compares without a fault.
func finite(v any) any {
	switch v := v.(type) {
	case map[string]any:
		for key, item := range v {
			v[key] = finite(item)
		}
	case []any:
		for i, item := range v {
			v[i] = finite(item)
		}
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0.5
		}
	}
	return v
}

// scalarText returns the text of the scalar at a path, and "" when the path
// holds none.
func scalarText(root *model.Value, path ...string) string {
	if node := root.At(path...); node.Scalar() {
		return node.Text
	}
	return ""
}

// row is the table of schema.md: it maps a leaf, by the path of its value and
// the place of its keyword, to a rule and the gate the diagnostic names. ok is
// false for the one leaf a file kind drops.
func row(k fileKind, root *model.Value, at []string, keyword string, failed jsonschema.ErrorKind) (code, gate string, ok bool) {
	holds := func(part string) bool { return strings.Contains(keyword, part) }
	_, unknown := failed.(*kind.AdditionalProperties)
	required, lacks := failed.(*kind.Required)
	switch k {
	case versionFile:
		return "P3", "", true

	case gatesFile:
		if len(at) == 0 {
			switch {
			case unknown:
				return "G11", "", true
			case lacks && len(required.Missing) == 1 && required.Missing[0] == "states":
				return "G7", "", true
			}
			return "G3", "", true
		}
		field := ""
		if len(at) == 3 {
			field = at[2]
		}
		switch at[0] {
		case "gates":
			switch {
			case len(at) == 1:
				return "G3", "", true
			case field == "key" && at[1] == "0" && holds("/prefixItems/"):
				return "G2", "", true
			case field == "key":
				return "G5", scalarText(root, "gates", at[1], "key"), true
			}
			return "G4", scalarText(root, "gates", at[1], "key"), true
		case "states":
			switch field {
			case "key":
				return "G5", "", true
			case "severity":
				return "G8", "", true
			}
			return "G7", "", true
		case "reasons":
			if field == "key" {
				return "G5", "", true
			}
			return "G10", "", true
		}
		return "G11", "", true

	case taskFile:
		if len(at) == 0 {
			if unknown {
				return "T3", "", true
			}
			return "T2", "", true
		}
		switch at[0] {
		case "title", "description":
			return "T2", "", true
		case "assignee":
			if holds("/format") {
				return "T4", "", true
			}
			return "T2", "", true
		case "references":
			return "T5", "", true
		case "parent":
			if len(at) == 2 && at[1] == "id" {
				return "T6", "", true
			}
			return "T11", "", true
		case "requires":
			switch {
			case len(at) == 1:
				return "R8", "", true
			case len(at) == 2:
				if holds("/items/oneOf") && root.At("requires", at[1], "subproject") != nil {
					return "R11", "", true
				}
				return "R8", "", true
			}
			switch at[2] {
			case "id":
				return "T6", "", true
			case "from":
				return "R6", scalarText(root, at...), true
			case "to":
				return "R7", scalarText(root, at...), true
			case "subproject":
				if len(at) >= 4 && at[3] == "id" {
					return "T6", "", true
				}
				if len(at) >= 4 && at[3] == "commit" {
					return "J14", "", true
				}
				return "R11", "", true
			}
			return "R8", "", true
		case "junctions":
			if len(at) == 1 {
				return "J4", "", true
			}
			gate = at[1]
			if len(at) == 2 {
				switch {
				case holds("/dependentRequired"):
					return "J5", gate, true
				case holds("/$defs/plain/"):
					return "J10", gate, true
				case holds("/$defs/recursive/") && lacks:
					return "J7", gate, true
				case holds("/$defs/recursive/"):
					return "J4", gate, true
				}
				return "J6", gate, true
			}
			switch at[2] {
			case "contributor", "reviewer":
				return "T4", gate, true
			case "references":
				return "T5", gate, true
			case "model":
				return "J10", gate, true
			case "applies":
				return "J6", gate, true
			case "subproject":
				if len(at) >= 4 && at[3] == "id" {
					return "T6", gate, true
				}
				if len(at) >= 4 && at[3] == "commit" {
					return "J14", gate, true
				}
				return "J7", gate, true
			}
		}
		return "T3", "", true
	}

	// A status file. The undefined rule of the schema cannot tell which side
	// is wrong without a gate, and S3 or S5 then reports the gate.
	if holds("/then/") {
		if !root.Get("gate").Scalar() {
			return "", "", false
		}
		return "S4", "", true
	}
	if len(at) > 0 {
		switch at[0] {
		case "gate":
			return "S5", scalarText(root, "gate"), true
		case "state", "reason":
			return "S6", "", true
		}
	}
	return "S3", "", true
}

// place gives a leaf its position and its message, by the kind of the
// failure. The message is the Validator's own and holds no word of the schema
// library. An unknown field gives a diagnostic for each name, at its key.
func place(file *model.File, at []string, keyword string, failed jsonschema.ErrorKind) []model.Diagnostic {
	// node is the deepest value that exists along the path.
	node := file.Root
	for _, segment := range at {
		next := node.At(segment)
		if next == nil {
			break
		}
		node = next
	}
	pos := model.Pos{File: file.Path}
	if node != nil {
		pos = node.Pos
	}
	where := "the file"
	if len(at) > 0 {
		where = strings.Join(at, ".")
	}
	one := func(pos model.Pos, format string, args ...any) []model.Diagnostic {
		return []model.Diagnostic{{Pos: pos, Message: fmt.Sprintf(format, args...)}}
	}
	switch failed := failed.(type) {
	case *kind.AdditionalProperties:
		names := append([]string{}, failed.Properties...)
		sort.Strings(names)
		var all []model.Diagnostic
		for _, name := range names {
			at := pos
			if node != nil {
				for i := range node.Fields {
					if node.Fields[i].Key == name {
						at = node.Fields[i].KeyPos
					}
				}
			}
			all = append(all, one(at, "%s is no field of %s", name, where)...)
		}
		return all
	case *kind.Required:
		if strings.Contains(keyword, "/items/oneOf") {
			break
		}
		missing := append([]string{}, failed.Missing...)
		sort.Strings(missing)
		return one(pos, "%s lacks %s", where, strings.Join(missing, ", "))
	case *kind.DependentRequired:
		if stated := node.Get("model"); stated != nil {
			pos = stated.Pos
		}
		return one(pos, "%s states model and no contributor", where)
	case *kind.Type:
		want := make([]string, len(failed.Want))
		for i, name := range failed.Want {
			want[i] = typeName(name)
		}
		got := "null"
		if node != nil && node.Kind != model.Null {
			got = article(kindName(node.Kind))
		}
		return one(pos, "%s is %s, not %s", where, got, article(strings.Join(want, " or ")))
	case *kind.Pattern:
		return one(pos, "%s %s does not match %s", where, show(node), failed.Want)
	case *kind.Format:
		what := "is no " + failed.Want
		switch failed.Want {
		case "email":
			what = "is no email address"
		case "uri-reference":
			what = "is no URI reference"
		}
		return one(pos, "%s %s %s", where, show(node), what)
	case *kind.MinLength:
		return one(pos, "%s is empty", where)
	case *kind.Minimum:
		if node != nil && node.Kind == model.Float {
			if f, ok := node.Plain().(float64); ok && (math.IsNaN(f) || math.IsInf(f, 0)) {
				return nil // the type leaf reports what is no number
			}
		}
		return one(pos, "%s is %s, below %s", where, show(node), failed.Want.RatString())
	case *kind.MinItems:
		return one(pos, "%s holds %d, fewer than %d", where, failed.Got, failed.Want)
	case *kind.Const:
		return one(pos, "%s is %s, not %v", where, show(node), failed.Want)
	}
	if strings.Contains(keyword, "/items/oneOf") {
		if node.Get("id") != nil && node.Get("subproject") != nil {
			return one(pos, "%s states both id and subproject", where)
		}
		return one(pos, "%s states neither id nor subproject", where)
	}
	return one(pos, "%s does not match its schema", where)
}

// show returns a value as a message names it: the text of a scalar, or the
// kind of a collection.
func show(node *model.Value) string {
	switch {
	case node == nil:
		return "nothing"
	case !node.Scalar():
		return article(kindName(node.Kind))
	case node.Text == "":
		return strconv.Quote(node.Text)
	}
	return node.Text
}

// kindName names the kind of a node as a message writes it.
func kindName(k model.Kind) string {
	switch k {
	case model.Null:
		return "null"
	case model.Bool:
		return "boolean"
	case model.Int:
		return "integer"
	case model.Float:
		return "number"
	case model.String:
		return "string"
	case model.Seq:
		return "sequence"
	}
	return "mapping"
}

// typeName names a JSON Schema type as a message writes it.
func typeName(name string) string {
	switch name {
	case "object":
		return "mapping"
	case "array":
		return "sequence"
	}
	return name
}

// article puts a or an before a noun.
func article(noun string) string {
	if noun != "" && strings.ContainsRune("aeiou", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}
