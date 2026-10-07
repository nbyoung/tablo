package validate

import (
	"sort"
	"strconv"
	"strings"

	"github.com/nbyoung/tablo/internal/model"
)

// tree applies the rules of the task tree: one root, then for each task its
// name, its parent and its chain, then the orders the children of each task
// share.
func (r *run) tree() {
	r.t8()
	for _, id := range r.project.TaskIDs() {
		task := r.project.Tasks[id]
		r.t1(task)
		r.t9(task)
		r.t10(task)
	}
	for _, id := range r.project.TaskIDs() {
		r.t12(r.project.Tasks[id])
	}
}

// task applies the rules of one task file: its schema, the unquoted ids, the
// junction entries, the requirements, and the gates that apply.
func (r *run) task(t *model.Task) {
	// The codes the schema raises under each requirement, by index.
	under := map[string]map[string]bool{}
	for _, l := range checkFile(taskFile, t.File, t.ID) {
		r.found = append(r.found, l.diagnostic)
		if len(l.at) >= 2 && l.at[0] == "requires" {
			if under[l.at[1]] == nil {
				under[l.at[1]] = map[string]bool{}
			}
			under[l.at[1]][l.diagnostic.Code] = true
		}
	}
	r.t7(t)
	for _, j := range t.Junctions {
		r.junction(t, j)
	}
	for i, requirement := range t.Requires {
		r.requirement(t, requirement, under[strconv.Itoa(i)])
	}
	r.j12(t)
}

// t1 reports a task whose file name is no id. The task stays in the tree.
func (r *run) t1(t *model.Task) {
	if !idPattern.MatchString(t.ID) {
		r.add("T1", model.Pos{File: t.File.Path}, t.ID, "", "the file name %s is not four lowercase hexadecimal digits", t.ID)
	}
}

// t7 warns of an id written without quotes, at each of the four places a task
// file names one.
func (r *run) t7(t *model.Task) {
	warn := func(node *model.Value, path string) {
		if !node.Scalar() || node.Quoted {
			return
		}
		switch node.Read {
		case model.String:
			r.add("T7", node.Pos, t.ID, "", "%s is written without quotes: %s", path, node.Text)
		default:
			read := kindName(node.Read)
			if node.Read == model.Float {
				read = "float"
			}
			r.add("T7", node.Pos, t.ID, "", "%s is written as the %s %s, not the string %q", path, read, node.Text, node.Text)
		}
	}
	if t.Parent != nil {
		warn(t.Parent.ID.Node, "parent.id")
	}
	for i, requirement := range t.Requires {
		at := "requires." + strconv.Itoa(i)
		warn(requirement.ID.Node, at+".id")
		if requirement.Subproject != nil {
			warn(requirement.Subproject.ID.Node, at+".subproject.id")
		}
	}
	for _, j := range t.Junctions {
		if j.Subproject != nil {
			warn(j.Subproject.ID.Node, "junctions."+j.Gate+".subproject.id")
		}
	}
}

// t8 reports a project whose tasks with no parent are not one. It counts no
// root while a task file is unread, since that file may hold the root.
func (r *run) t8() {
	roots := r.shape.roots
	if len(r.shape.unread) > 0 || len(roots) == 1 {
		return
	}
	switch {
	case len(r.project.Tasks) == 0:
		r.add("T8", model.Pos{}, "", "", "no task is the root: the project holds no task file")
	case len(roots) == 0:
		r.add("T8", model.Pos{}, "", "", "no task is the root: every task file states a parent")
	default:
		r.add("T8", model.Pos{}, "", "", "%d tasks state no parent: %s", len(roots), strings.Join(roots, ", "))
	}
}

// t9 reports a parent that names no task. It stays silent about an unread id.
func (r *run) t9(t *model.Task) {
	if t.Parent == nil || !t.Parent.ID.Node.Scalar() {
		return
	}
	if id := t.Parent.ID.V; r.project.Tasks[id] == nil && !r.shape.unread[id] {
		r.add("T9", t.Parent.ID.Node.Pos, t.ID, "", "parent %s names no task", id)
	}
}

// t10 reports a task whose walk up its parents meets a task twice.
func (r *run) t10(t *model.Task) {
	if t.Parent == nil || !t.Parent.ID.Node.Scalar() || r.project.Tasks[t.Parent.ID.V] == nil {
		return
	}
	met := map[string]bool{t.ID: true}
	for at := t; at.Parent != nil && at.Parent.ID.Node.Scalar(); {
		above := at.Parent.ID.V
		if met[above] {
			r.add("T10", t.Parent.ID.Node.Pos, t.ID, "", "the parent chain of %s loops and never reaches the root", t.ID)
			return
		}
		met[above] = true
		if at = r.project.Tasks[above]; at == nil {
			return
		}
	}
}

// t12 warns of each order that two or more children of a task share, orders
// ascending. The finding sits in the parent's file and names the parent.
func (r *run) t12(parent *model.Task) {
	sharing := map[int][]string{}
	for _, id := range r.shape.children[parent.ID] {
		if order := r.project.Tasks[id].Parent.Order; order.OK {
			sharing[order.V] = append(sharing[order.V], id)
		}
	}
	orders := make([]int, 0, len(sharing))
	for order, ids := range sharing {
		if len(ids) > 1 {
			orders = append(orders, order)
		}
	}
	sort.Ints(orders)
	for _, order := range orders {
		r.add("T12", model.Pos{File: parent.File.Path}, parent.ID, "",
			"%s share order %d under %s", strings.Join(sharing[order], " and "), order, parent.ID)
	}
}
