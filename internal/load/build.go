package load

import (
	"strconv"

	"github.com/nbyoung/tablo/internal/model"
)

// codeVersion is the one content rule of RULES.md the Loader raises: the
// project's major version equals the module's and its minor does not exceed it.
const codeVersion = "P4"

// buildVersion types version.yaml and raises P4 when the version is well
// formed and the module does not accept it.
func buildVersion(file *model.File) (*model.Version, []model.Diagnostic) {
	v := &model.Version{
		File:     file,
		Tableaux: model.StrOf(file.Root.Get("tableaux")),
		Trunk:    model.StrOf(file.Root.Get("trunk")),
	}
	if !v.Tableaux.Node.Scalar() {
		return v, nil
	}
	v.Major, v.Minor, v.Patch, v.WellFormed = model.ParseVersion(v.Tableaux.V)
	v.Accepted = v.WellFormed && model.Accepts(v.Tableaux.V)
	if !v.WellFormed || v.Accepted {
		return v, nil
	}
	return v, []model.Diagnostic{{
		Code:     codeVersion,
		Severity: model.Error,
		Pos:      v.Tableaux.Node.Pos,
		Message: "the project states tableaux " + v.Tableaux.V + "; the module accepts major " +
			strconv.Itoa(model.AcceptedMajor) + " up to minor " + strconv.Itoa(model.AcceptedMinor),
	}}
}

// buildGating types gates.yaml, with an entry for every item written.
func buildGating(file *model.File) *model.Gating {
	g := &model.Gating{File: file}
	for _, node := range items(file.Root.Get("gates")) {
		g.Gates = append(g.Gates, &model.Gate{
			Key:      model.StrOf(node.Get("key")),
			Symbol:   model.StrOf(node.Get("symbol")),
			Name:     model.StrOf(node.Get("name")),
			Criteria: model.StrOf(node.Get("criteria")),
			Node:     node,
		})
	}
	for _, node := range items(file.Root.Get("states")) {
		g.States = append(g.States, &model.State{
			Key:      model.StrOf(node.Get("key")),
			Symbol:   model.StrOf(node.Get("symbol")),
			Synopsis: model.StrOf(node.Get("synopsis")),
			Severity: model.IntOf(node.Get("severity")),
			Node:     node,
		})
	}
	for _, node := range items(file.Root.Get("reasons")) {
		g.Reasons = append(g.Reasons, &model.Reason{
			Key:      model.StrOf(node.Get("key")),
			Symbol:   model.StrOf(node.Get("symbol")),
			Synopsis: model.StrOf(node.Get("synopsis")),
			Node:     node,
		})
	}
	return g
}

// buildTask types tasks/<id>.yaml as written. It first takes the scalar at
// each of the four places a task file names an id as a string, so that 07e0
// stays 07e0 in the typed model and in what Plain hands a schema.
func buildTask(id string, file *model.File) *model.Task {
	root := file.Root
	t := &model.Task{
		ID:          id,
		File:        file,
		Title:       model.StrOf(root.Get("title")),
		Description: model.StrOf(root.Get("description")),
		Assignee:    model.StrOf(root.Get("assignee")),
		References:  buildReferences(root.Get("references")),
	}
	for _, node := range items(root.Get("requires")) {
		asID(node.Get("id"))
		t.Requires = append(t.Requires, &model.Requirement{
			ID:         model.StrOf(node.Get("id")),
			Subproject: buildSubproject(node.Get("subproject")),
			From:       model.StrOf(node.Get("from")),
			To:         model.StrOf(node.Get("to")),
			Text:       model.StrOf(node.Get("text")),
			Node:       node,
		})
	}
	if junctions := root.Get("junctions"); junctions != nil {
		for _, field := range junctions.Fields {
			node := field.Value
			t.Junctions = append(t.Junctions, &model.Junction{
				Gate:        field.Key,
				KeyPos:      field.KeyPos,
				Contributor: model.StrOf(node.Get("contributor")),
				Model:       model.StrOf(node.Get("model")),
				Reviewer:    model.StrOf(node.Get("reviewer")),
				References:  buildReferences(node.Get("references")),
				Subproject:  buildSubproject(node.Get("subproject")),
				Applies:     model.BoolOf(node.Get("applies")),
				Node:        node,
			})
		}
	}
	if node := root.Get("parent"); node != nil {
		asID(node.Get("id"))
		t.Parent = &model.Parent{
			ID:    model.StrOf(node.Get("id")),
			Order: model.IntOf(node.Get("order")),
			Node:  node,
		}
	}
	return t
}

// buildReferences types a references list.
func buildReferences(list *model.Value) []*model.Reference {
	var references []*model.Reference
	for _, node := range items(list) {
		references = append(references, &model.Reference{
			URL:  model.StrOf(node.Get("url")),
			Text: model.StrOf(node.Get("text")),
			Node: node,
		})
	}
	return references
}

// buildSubproject types a subproject field; nil when the file states none.
// The Loader sets Link once it resolves the project's links.
func buildSubproject(node *model.Value) *model.Subproject {
	if node == nil {
		return nil
	}
	asID(node.Get("id"))
	return &model.Subproject{
		URL:    model.StrOf(node.Get("url")),
		ID:     model.StrOf(node.Get("id")),
		Commit: model.StrOf(node.Get("commit")),
		Node:   node,
	}
}

// buildStatus types status/<id>.yaml as written.
func buildStatus(id string, file *model.File) *model.Status {
	root := file.Root
	return &model.Status{
		ID:     id,
		File:   file,
		Gate:   model.StrOf(root.Get("gate")),
		State:  model.StrOf(root.Get("state")),
		Reason: model.StrOf(root.Get("reason")),
		Note:   model.StrOf(root.Get("note")),
	}
}

// asID takes a scalar at an id position as a string whose text is the source
// text. Read keeps the kind YAML gave it.
func asID(node *model.Value) {
	if node.Scalar() {
		node.Kind = model.String
	}
}

// items returns the entries of a sequence, and none for any other node.
func items(list *model.Value) []*model.Value {
	if list == nil || list.Kind != model.Seq {
		return nil
	}
	return list.Items
}

// subprojects returns every subproject field of a task, a requirement's
// before a junction's, each in file order.
func subprojects(t *model.Task) []*model.Subproject {
	var all []*model.Subproject
	for _, r := range t.Requires {
		if r.Subproject != nil {
			all = append(all, r.Subproject)
		}
	}
	for _, j := range t.Junctions {
		if j.Subproject != nil {
			all = append(all, j.Subproject)
		}
	}
	return all
}
