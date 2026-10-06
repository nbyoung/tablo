package model

import (
	"math"
	"reflect"
	"testing"
)

// str, num and the rest build values as the Loader does, for tests that need no file.
func scalar(kind Kind, text string) *Value {
	return &Value{Kind: kind, Read: kind, Text: text}
}

func mapping(pairs ...any) *Value {
	v := &Value{Kind: Map, Read: Map}
	for i := 0; i < len(pairs); i += 2 {
		v.Fields = append(v.Fields, Field{Key: pairs[i].(string), Value: pairs[i+1].(*Value)})
	}
	return v
}

func sequence(items ...*Value) *Value {
	return &Value{Kind: Seq, Read: Seq, Items: items}
}

func TestGetAndAt(t *testing.T) {
	leaf := scalar(String, "x")
	root := mapping(
		"a", sequence(scalar(Int, "1"), mapping("b", leaf)),
		"0", scalar(Null, ""),
	)
	if root.Get("a") == nil || root.Get("missing") != nil || leaf.Get("a") != nil || (*Value)(nil).Get("a") != nil {
		t.Error("Get returns the value of a key in a Map and nil elsewhere")
	}
	if got := root.At("a", "1", "b"); got != leaf {
		t.Errorf("At(a, 1, b) = %+v", got)
	}
	if got := root.At(); got != root {
		t.Error("At with no path returns the value")
	}
	if got := root.At("0"); got == nil || got.Kind != Null {
		t.Errorf("At(0) on a Map takes the segment as a key: %+v", got)
	}
	for _, path := range [][]string{{"a", "2"}, {"a", "01"}, {"a", "-1"}, {"a", "x"}, {"a", "0", "b"}, {"b"}, {"a", "1", "b", "c"}} {
		if got := root.At(path...); got != nil {
			t.Errorf("At(%v) = %+v, want nil", path, got)
		}
	}
}

func TestPlain(t *testing.T) {
	tests := []struct {
		v    *Value
		want any
	}{
		{nil, nil},
		{scalar(Null, "~"), nil},
		{scalar(Bool, "True"), true},
		{scalar(Bool, "FALSE"), false},
		{scalar(Int, "0123"), int64(123)},
		{scalar(Int, "-7"), int64(-7)},
		{scalar(Int, "+7"), int64(7)},
		{scalar(Int, "0o17"), int64(15)},
		{scalar(Int, "0x1f"), int64(31)},
		{scalar(Int, "99999999999999999999"), 1e20},
		{scalar(Float, "40e8"), 40e8},
		{scalar(Float, ".5"), 0.5},
		{scalar(Float, "1."), 1.0},
		{scalar(Float, "-.inf"), math.Inf(-1)},
		{scalar(Float, ".Inf"), math.Inf(1)},
		{scalar(String, "07e0"), "07e0"},
		{sequence(), []any{}},
		{mapping("a", sequence(scalar(Int, "1"))), map[string]any{"a": []any{int64(1)}}},
	}
	for _, tt := range tests {
		if got := tt.v.Plain(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Plain(%+v) = %#v, want %#v", tt.v, got, tt.want)
		}
	}
	if x, ok := scalar(Float, ".nan").Plain().(float64); !ok || !math.IsNaN(x) {
		t.Error("Plain(.nan) is no NaN")
	}
}

func TestTypedFields(t *testing.T) {
	if f := StrOf(nil); f.Node != nil || f.V != "" {
		t.Errorf("StrOf(nil) = %+v", f)
	}
	if f := StrOf(scalar(Int, "0010")); f.V != "0010" || f.Node == nil {
		t.Errorf("StrOf keeps the text of any scalar: %+v", f)
	}
	if f := StrOf(sequence(scalar(String, "a"))); f.V != "" || f.Node == nil {
		t.Errorf("StrOf of a collection has a node and no text: %+v", f)
	}
	ints := []struct {
		v    *Value
		want int
		ok   bool
	}{
		{nil, 0, false},
		{scalar(Int, "3"), 3, true},
		{scalar(Int, "-2"), -2, true},
		{scalar(Float, "2.0"), 2, true},
		{scalar(Float, "1e3"), 1000, true},
		{scalar(Float, "2.5"), 0, false},
		{scalar(Float, ".inf"), 0, false},
		{scalar(Float, ".nan"), 0, false},
		{scalar(Int, "99999999999999999999"), 0, false},
		{scalar(String, "3"), 0, false},
		{scalar(Bool, "true"), 0, false},
	}
	for _, tt := range ints {
		if f := IntOf(tt.v); f.V != tt.want || f.OK != tt.ok || f.Node != tt.v {
			t.Errorf("IntOf(%+v) = %+v, want %d, %v", tt.v, f, tt.want, tt.ok)
		}
	}
	if f := BoolOf(scalar(Bool, "true")); !f.V || !f.OK {
		t.Errorf("BoolOf(true) = %+v", f)
	}
	if f := BoolOf(scalar(Bool, "false")); f.V || !f.OK {
		t.Errorf("BoolOf(false) = %+v", f)
	}
	if f := BoolOf(scalar(String, "yes")); f.V || f.OK || f.Node == nil {
		t.Errorf("BoolOf(yes) = %+v", f)
	}
	if f := BoolOf(nil); f.OK || f.Node != nil {
		t.Errorf("BoolOf(nil) = %+v", f)
	}
}

func TestJunctionKind(t *testing.T) {
	present := scalar(String, "x")
	tests := []struct {
		node *Value
		want JunctionKind
	}{
		{mapping(), Plain},
		{scalar(Null, ""), Plain},
		{mapping("contributor", present, "references", sequence()), Plain},
		{mapping("unknown", present), Plain},
		{mapping("subproject", mapping()), Recursive},
		{mapping("applies", scalar(Bool, "false")), NotApplicable},
		{mapping("applies", scalar(Bool, "true")), NotApplicable},
		{mapping("applies", scalar(String, "no")), NotApplicable},
		{mapping("applies", scalar(Bool, "false"), "contributor", present), Mixed},
		{mapping("subproject", mapping(), "reviewer", present), Mixed},
		{mapping("subproject", mapping(), "references", sequence()), Mixed},
		{mapping("subproject", mapping(), "applies", scalar(Bool, "false")), Mixed},
	}
	for _, tt := range tests {
		j := &Junction{
			Contributor: StrOf(tt.node.Get("contributor")),
			Model:       StrOf(tt.node.Get("model")),
			Reviewer:    StrOf(tt.node.Get("reviewer")),
			Applies:     BoolOf(tt.node.Get("applies")),
			Node:        tt.node,
		}
		if got := j.Kind(); got != tt.want {
			t.Errorf("Kind of %v = %s, want %s", tt.node.Plain(), got, tt.want)
		}
	}
}

func TestTaskIDs(t *testing.T) {
	p := &Project{Tasks: map[string]*Task{"b2c9": {}, "0010": {}, "leaf-two": {}, "A000": {}}}
	if got, want := p.TaskIDs(), []string{"0010", "A000", "b2c9", "leaf-two"}; !reflect.DeepEqual(got, want) {
		t.Errorf("TaskIDs() = %v, want %v", got, want)
	}
}
