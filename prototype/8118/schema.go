package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Violation is one place where an instance fails a schema.
type Violation struct {
	Pos     Pos
	Path    string // JSON pointer-like path of the instance, "" for the root
	Keyword string // the failing schema keyword
	Msg     string
	Then    bool // the failure sits under an if/then
}

// Schema validates instance nodes against one parsed schema file. It covers
// the keywords the five tableaux schemas use and no others.
type Schema struct{ root *Node }

var (
	emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	reCache = map[string]*regexp.Regexp{}
)

// Validate returns every violation of the schema by inst.
func (s Schema) Validate(inst *Node) []Violation {
	var out []Violation
	s.check(inst, s.root, "", false, &out)
	return out
}

func (s Schema) valid(inst, sch *Node, path string) bool {
	var out []Violation
	s.check(inst, sch, path, false, &out)
	return len(out) == 0
}

func (s Schema) resolve(ref string) *Node {
	n := s.root
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		if n = n.Get(part); n == nil {
			panic("unresolved $ref " + ref)
		}
	}
	return n
}

func (s Schema) check(inst, sch *Node, path string, then bool, out *[]Violation) {
	add := func(at Pos, kw, format string, args ...any) {
		*out = append(*out, Violation{at, path, kw, fmt.Sprintf(format, args...), then})
	}
	if sch.Kind == Bool {
		if !sch.Bool {
			add(inst.Pos, "false", "no value is allowed here")
		}
		return
	}
	for _, e := range sch.Entries {
		v := e.Val
		switch e.Key {
		case "$ref":
			s.check(inst, s.resolve(v.Str), path, then, out)
		case "type":
			ok := map[string]Kind{"object": Map, "array": Seq, "string": Str, "integer": Int}[v.Str] == inst.Kind
			if !ok {
				add(inst.Pos, "type", "want %s", v.Str)
				return
			}
		case "const":
			if inst.Kind != v.Kind || inst.Str != v.Str || inst.Int != v.Int || inst.Bool != v.Bool {
				add(inst.Pos, "const", "want %s", scalar(v))
			}
		case "pattern":
			if inst.Kind == Str && !regex(v.Str).MatchString(inst.Str) {
				add(inst.Pos, "pattern", "%q does not match %s", inst.Str, v.Str)
			}
		case "minLength":
			if inst.Kind == Str && utf8.RuneCountInString(inst.Str) < v.Int {
				add(inst.Pos, "minLength", "shorter than %d", v.Int)
			}
		case "minimum":
			if inst.Kind == Int && inst.Int < v.Int {
				add(inst.Pos, "minimum", "below %d", v.Int)
			}
		case "format":
			bad := inst.Kind == Str && ((v.Str == "email" && !emailRe.MatchString(inst.Str)) ||
				(v.Str == "uri-reference" && strings.ContainsAny(inst.Str, " \t\n")))
			if bad {
				add(inst.Pos, "format", "%q is not a valid %s", inst.Str, v.Str)
			}
		case "required":
			if inst.Kind == Map {
				for _, k := range v.Items {
					if inst.Get(k.Str) == nil {
						add(inst.Pos, "required", "missing %q", k.Str)
					}
				}
			}
		case "dependentRequired":
			for _, d := range v.Entries {
				if inst.Get(d.Key) == nil {
					continue
				}
				for _, k := range d.Val.Items {
					if inst.Get(k.Str) == nil {
						add(inst.Pos, "dependentRequired", "%q needs %q", d.Key, k.Str)
					}
				}
			}
		case "properties":
			for _, ent := range inst.Entries {
				if sub := v.Get(ent.Key); sub != nil {
					s.check(ent.Val, sub, path+"/"+ent.Key, then, out)
				}
			}
		case "additionalProperties":
			for _, ent := range inst.Entries {
				if sch.Get("properties").Get(ent.Key) != nil {
					continue
				}
				if v.Kind == Bool && !v.Bool {
					add(ent.KeyPos, "additionalProperties", "unexpected field %q", ent.Key)
				} else if v.Kind == Map {
					s.check(ent.Val, v, path+"/"+ent.Key, then, out)
				}
			}
		case "propertyNames":
			for _, ent := range inst.Entries {
				s.check(&Node{Kind: Str, Pos: ent.KeyPos, Str: ent.Key}, v, path+"/"+ent.Key, then, out)
			}
		case "prefixItems":
			for i, sub := range v.Items {
				if i < len(inst.Items) {
					s.check(inst.Items[i], sub, fmt.Sprintf("%s/%d", path, i), then, out)
				}
			}
		case "items":
			for i := len(sch.Get("prefixItems").itemsOrNil()); i < len(inst.Items); i++ {
				s.check(inst.Items[i], v, fmt.Sprintf("%s/%d", path, i), then, out)
			}
		case "minItems":
			if inst.Kind == Seq && len(inst.Items) < v.Int {
				add(inst.Pos, "minItems", "fewer than %d items", v.Int)
			}
		case "contains":
			found := false
			for _, it := range inst.Items {
				found = found || s.valid(it, v, path)
			}
			if inst.Kind == Seq && !found {
				add(inst.Pos, "contains", "no item matches")
			}
		case "allOf":
			for _, sub := range v.Items {
				s.check(inst, sub, path, then, out)
			}
		case "oneOf":
			n := 0
			for _, sub := range v.Items {
				if s.valid(inst, sub, path) {
					n++
				}
			}
			if n != 1 {
				add(inst.Pos, "oneOf", "%d of %d alternatives match", n, len(v.Items))
			}
		case "not":
			if s.valid(inst, v, path) {
				add(inst.Pos, "not", "value is forbidden")
			}
		case "if":
			if s.valid(inst, v, path) {
				if t := sch.Get("then"); t != nil {
					s.check(inst, t, path, true, out)
				}
			}
		}
	}
}

func (n *Node) itemsOrNil() []*Node {
	if n == nil {
		return nil
	}
	return n.Items
}

func scalar(n *Node) string {
	switch n.Kind {
	case Bool:
		return fmt.Sprint(n.Bool)
	case Int:
		return fmt.Sprint(n.Int)
	}
	return n.Str
}

func regex(p string) *regexp.Regexp {
	if r, ok := reCache[p]; ok {
		return r
	}
	r := regexp.MustCompile(p)
	reCache[p] = r
	return r
}
