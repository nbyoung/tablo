package conformance

import (
	"sort"
	"strings"

	"github.com/nbyoung/tablo/internal/corpustest"
	"gopkg.in/yaml.v3"
)

// observe turns the calls of one entry into a map[string]any shaped as
// expected.yaml; compare then walks the expected document and never looks at
// an envelope. The functions below this comment are the only place that knows
// the command's data. Each cell of the table in design 4b4f, "The observed
// document", reads the key the implemented command names:
//
//	findings[]           validate  diagnostics[]: code as rule, severity, path (less its leading
//	                               ".tableaux/") as file, task, gate, commit, trailer
//	valid                validate  the exit code: 0 true, 1 false
//	owner                task      data.assignee of the root
//	root, count, order   task      the walk from the root: data.parent.id up, data.children[].id down
//	parent               task      data.parent.id
//	authorities          task      data.authorities[].email
//	authorisation        task      state from data.authorised; commit from data.authorisation.commit.hash;
//	                               by from data.authorisation.email
//	junctions.default    task      { kind: plain, contributor: data.assignee }
//	junctions.<gate>     task      the item of data.junctions with that gate; source from its from[]
//	applicable           task      undefined, then the gate of each item of data.junctions that is not not_applicable
//	requires[]           task      data.requires[]: id from task.id, or subproject { url, id, pin } from
//	                               subproject, task.id and commit; from, to, met, due, condition
//	reviews[]            task      data.reviews[]: gate, reviewer, commit.hash
//	subproject           task      data.snapshot: url, pin from commit, task from task.id
//	status               task      data.status: gate, state, reason, note, date, recorder, derived;
//	                               commit from data.status_commit.hash; from from status.from.task.id of a roll-up,
//	                               or of data.snapshot.status when a derived status is read through a snapshot
//	events[]             history   the events of the lines of the task, oldest first
//
// The calls are tablo task <id> and tablo history --task <id>, each with
// --level provenance; the envelope has no ref key, and source.commit is the
// commit in view.

// observeFindings reads the diagnostics of a validate envelope.
func observeFindings(env map[string]any) []any {
	var out []any
	for _, d := range asList(env["diagnostics"]) {
		m := asMap(d)
		out = append(out, map[string]any{
			"rule": m["code"], "severity": m["severity"],
			"file": strings.TrimPrefix(text(m["path"]), ".tableaux/"),
			"task": m["task"], "gate": m["gate"], "commit": m["commit"], "trailer": m["trailer"], "message": m["message"],
		})
	}
	return out
}

// observeTask reads the facts of one task from its task view at provenance.
func observeTask(env map[string]any) map[string]any {
	d := asMap(env["data"])
	t := map[string]any{}
	if p := asMap(d["parent"]); p != nil {
		t["parent"] = p["id"]
	}
	var authorities []any
	for _, a := range asList(d["authorities"]) {
		authorities = append(authorities, asMap(a)["email"])
	}
	t["authorities"] = authorities

	auth := asMap(d["authorisation"])
	state := "proposed"
	if b, _ := d["authorised"].(bool); b {
		state = "authorised"
	}
	t["authorisation"] = map[string]any{
		"state": state, "commit": asMap(auth["commit"])["hash"], "by": auth["email"],
	}

	junctions := map[string]any{"default": map[string]any{"kind": "plain", "contributor": d["assignee"]}}
	applicable := []any{"undefined"}
	for _, j := range asList(d["junctions"]) {
		m := asMap(j)
		gate := text(m["gate"])
		item := map[string]any{
			"kind": m["kind"], "contributor": m["contributor"], "model": m["model"], "reviewer": m["reviewer"],
			"source": m["from"],
		}
		var refs []any
		for _, r := range asList(m["references"]) {
			refs = append(refs, map[string]any{"url": asMap(r)["url"], "text": asMap(r)["text"]})
		}
		item["references"] = refs
		if s := asMap(m["subproject"]); s != nil {
			item["subproject"] = map[string]any{"url": s["url"], "id": s["id"], "commit": s["commit"]}
		}
		junctions[gate] = item
		if m["kind"] != "not_applicable" {
			applicable = append(applicable, gate)
		}
	}
	t["junctions"] = junctions
	t["applicable"] = applicable

	var requires []any
	for _, r := range asList(d["requires"]) {
		m := asMap(r)
		item := map[string]any{"from": m["from"], "to": m["to"], "met": m["met"], "due": m["due"], "condition": m["condition"]}
		id := asMap(m["task"])["id"]
		if url := text(m["subproject"]); url != "" {
			item["subproject"] = map[string]any{"url": url, "id": id, "pin": m["commit"]}
		} else {
			item["id"] = id
		}
		requires = append(requires, item)
	}
	t["requires"] = requires

	var reviews []any
	for _, r := range asList(d["reviews"]) {
		m := asMap(r)
		reviews = append(reviews, map[string]any{"gate": m["gate"], "reviewer": m["reviewer"], "commit": asMap(m["commit"])["hash"]})
	}
	t["reviews"] = reviews

	if s := asMap(d["snapshot"]); s != nil {
		t["subproject"] = map[string]any{"url": s["url"], "pin": s["commit"], "task": asMap(s["task"])["id"]}
	}

	st := asMap(d["status"])
	status := map[string]any{
		"gate": st["gate"], "state": st["state"], "reason": st["reason"], "note": st["note"], "date": st["date"],
		"recorder": st["recorder"], "derived": st["derived"], "commit": asMap(d["status_commit"])["hash"],
	}
	// from names the child a roll-up takes its status from: the status's own, or,
	// for a status read through a recursive junction, the subproject task's.
	if from := asMap(st["from"]); from != nil && from["kind"] == "rollup" {
		status["from"] = asMap(from["task"])["id"]
	} else if sub := asMap(asMap(asMap(d["snapshot"])["status"])["from"]); sub != nil && st["derived"] == true && sub["kind"] == "rollup" {
		status["from"] = asMap(sub["task"])["id"]
	}
	t["status"] = status
	return t
}

// observeEvents reads the events of one task from a history view at provenance.
func observeEvents(env map[string]any, id string) []any {
	var out []any
	for _, l := range asList(asMap(env["data"])["lines"]) {
		line := asMap(l)
		if text(asMap(line["task"])["id"]) != id {
			continue
		}
		for _, e := range asList(line["events"]) {
			m := asMap(e)
			item := map[string]any{
				"date": m["date"], "commit": m["commit"], "by": m["by"], "task": id, "event": m["kind"], "gate": m["gate"],
			}
			if r := asMap(m["recorded"]); r != nil {
				item["gate"], item["state"], item["reason"], item["note"] = r["gate"], r["state"], r["reason"], r["note"]
			}
			if p := asMap(m["pin"]); p != nil {
				item["url"], item["old"], item["new"] = p["url"], p["old"], p["new"]
			}
			if s := asMap(m["subproject"]); s != nil {
				item["subproject"] = s["task"]
			}
			out = append(out, item)
		}
	}
	return out
}

// An observer calls the command for one entry and builds its observed document.
type observer struct {
	r      *runner
	e      *corpustest.Entry
	cache  map[string]*reply
	errs   []string
	lines  map[string]string // the short form of a call to its command line
	rootID string
}

func newObserver(r *runner, e *corpustest.Entry) *observer {
	return &observer{r: r, e: e, cache: map[string]*reply{}, lines: map[string]string{}}
}

// call runs one command at a ref, once. A failure is recorded and returns nil.
func (o *observer) call(ref, short string, args ...string) *reply {
	name := short
	if ref != o.e.Ref {
		name += " --ref " + ref
	}
	if rep, ok := o.cache[name]; ok {
		return rep
	}
	full := []string{"-C", o.e.Repo, "--ref", ref}
	if o.e.Trunk != "" {
		full = append(full, "--trunk", o.e.Trunk)
	}
	urls := make([]string, 0, len(o.e.Replace))
	for url := range o.e.Replace {
		urls = append(urls, url)
	}
	sort.Strings(urls)
	for _, url := range urls {
		full = append(full, "--replace", url+"="+o.e.Replace[url])
	}
	full = append(full, "--json")
	full = append(full, args...)
	rep, err := o.r.run(full...)
	if err != nil {
		o.errs = append(o.errs, err.Error())
		rep = nil
	}
	if rep != nil {
		o.lines[name] = rep.line
	}
	o.cache[name] = rep
	return rep
}

func (o *observer) taskView(ref, id string) map[string]any {
	rep := o.call(ref, "task "+id, "task", id, "--level", "provenance")
	if rep == nil {
		return nil
	}
	return rep.env
}

// callFor names the call that supplies the value at a path of the observed document.
func (o *observer) callFor(path string) string {
	suffix := ""
	if rest, ok := strings.CutPrefix(path, "refs."); ok {
		branch, tail, _ := strings.Cut(rest, ".")
		suffix = " --ref " + branch
		path = tail
		if path == "commit" || path == "" {
			return "validate" + suffix
		}
	}
	parts := strings.Split(path, ".")
	switch {
	case parts[0] == "valid" || parts[0] == "findings":
		return "validate"
	case parts[0] == "tasks" && len(parts) > 2 && strings.HasPrefix(parts[2], "events"):
		return "history --task " + parts[1] + suffix
	case parts[0] == "tasks" && len(parts) > 1:
		return "task " + parts[1] + suffix
	case parts[0] == "tasks" && o.rootID != "":
		return "task " + o.rootID
	}
	if o.rootID != "" {
		return "task " + o.rootID
	}
	return "task"
}

// observe makes the observed document for the facts expected.yaml states. The
// calls it makes follow design 4b4f, "One entry, one run": steps 1 to 4, each
// only where the file states a fact there.
func (o *observer) observe(top *yaml.Node) map[string]any {
	doc := map[string]any{}
	ref := o.e.Ref
	rep := o.call(ref, "validate", "validate")
	if rep == nil {
		return doc
	}
	doc["valid"] = rep.code == 0
	doc["findings"] = observeFindings(rep.env)
	project := asMap(rep.env["project"])

	tasks := mapKey(top, "tasks")
	if tasks != nil {
		doc["tasks"] = o.tasksAt(ref, tasks)
	}
	if mapKey(top, "owner") != nil || mapKey(top, "root") != nil || mapKey(top, "count") != nil || mapKey(top, "order") != nil {
		start := textOf(mapKey(top, "root"))
		if start == "" && tasks != nil && len(tasks.Content) > 0 {
			start = tasks.Content[0].Value
		}
		if start == "" {
			start = text(project["root"])
		}
		o.project(doc, start)
	}
	if refs := mapKey(top, "refs"); refs != nil {
		out := map[string]any{}
		for i := 0; i+1 < len(refs.Content); i += 2 {
			branch, spec := refs.Content[i].Value, refs.Content[i+1]
			entry := map[string]any{}
			if t := mapKey(spec, "tasks"); t != nil {
				entry["tasks"] = o.tasksAt(branch, t)
			}
			if r := o.call(branch, "validate", "validate"); r != nil {
				entry["commit"] = asMap(r.env["source"])["commit"]
			}
			out[branch] = entry
		}
		doc["refs"] = out
	}
	return doc
}

// tasksAt observes each task a mapping names, at one ref, with the events of
// the tasks that state them.
func (o *observer) tasksAt(ref string, tasks *yaml.Node) map[string]any {
	out := map[string]any{}
	for i := 0; i+1 < len(tasks.Content); i += 2 {
		id, spec := tasks.Content[i].Value, tasks.Content[i+1]
		env := o.taskView(ref, id)
		if env == nil {
			continue
		}
		t := observeTask(env)
		if mapKey(spec, "events") != nil {
			if h := o.call(ref, "history --task "+id, "history", "--task", id, "--level", "provenance"); h != nil {
				t["events"] = observeEvents(h.env, id)
			}
		}
		out[id] = t
	}
	return out
}

// project fills owner, root, count and order by the walk the design describes:
// up the parents from the start to the task with none, then down the children
// in the order given.
func (o *observer) project(doc map[string]any, start string) {
	root, seen := start, map[string]bool{}
	for {
		env := o.taskView(o.e.Ref, root)
		if env == nil {
			return
		}
		seen[root] = true
		p := asMap(asMap(env["data"])["parent"])
		if p == nil || seen[text(p["id"])] {
			break
		}
		root = text(p["id"])
	}
	o.rootID = root
	var order []any
	leaves := 0
	visited := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		order = append(order, id)
		env := o.taskView(o.e.Ref, id)
		if env == nil {
			return
		}
		children := asList(asMap(env["data"])["children"])
		if len(children) == 0 {
			leaves++
		}
		for _, c := range children {
			walk(text(asMap(c)["id"]))
		}
	}
	walk(root)
	if env := o.taskView(o.e.Ref, root); env != nil {
		doc["owner"] = asMap(env["data"])["assignee"]
	}
	doc["root"] = root
	doc["order"] = order
	doc["count"] = map[string]any{"tasks": len(order), "leaves": leaves}
}
