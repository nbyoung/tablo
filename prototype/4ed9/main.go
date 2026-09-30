// Command 4ed9 is a prototype of the tablo plumbing command's shape: a
// subcommand dispatcher that reads a repository at a ref and emits JSON or
// YAML in a stable, versioned envelope. Content subcommands are stubs over
// real data; the trailer subcommand is complete.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nbyoung/tablo"
)

// Schema names the envelope version. A change to the envelope's keys bumps it.
const Schema = "tablo/1"

const usage = `usage: tablo [-C dir] [--ref ref] [--json|--yaml] <command> [args]
commands: validate audit view status history trailer version
  view <name>                         name is one of: tasks
  trailer authorised <id>
  trailer reviewed <id> <gate>
  trailer reaffirmed <id>`

var (
	idRe   = regexp.MustCompile(`^[0-9a-f]{4}$`)
	gateRe = regexp.MustCompile(`^[a-z][a-z_]*$`)
)

// options are the global flags, which precede the subcommand.
type options struct {
	dir, ref string
	yaml     bool
}

// usageError makes the dispatcher exit 2 and print usage.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

// result is what a subcommand returns: its data, its diagnostics, and whether
// they make the run fail (exit 1).
type result struct {
	data  any
	diags []map[string]any
	plain string // when set, printed as is unless the caller asks for a structured format
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	o := options{dir: ".", ref: "HEAD"}
	structured := false
	i := 0
flags:
	for ; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-C" || a == "--ref":
			if i+1 == len(args) {
				return fail(stderr, usageError{a + " needs a value"})
			}
			i++
			if a == "-C" {
				o.dir = args[i]
			} else {
				o.ref = args[i]
			}
		case a == "--json":
			o.yaml, structured = false, true
		case a == "--yaml":
			o.yaml, structured = true, true
		case strings.HasPrefix(a, "-"):
			return fail(stderr, usageError{"unknown flag " + a})
		default:
			break flags
		}
	}
	if i == len(args) {
		return fail(stderr, usageError{"no command"})
	}
	cmd, rest := args[i], args[i+1:]
	var (
		res result
		err error
	)
	switch cmd {
	case "version":
		res = result{data: map[string]any{"version": tablo.Version, "tableaux": fmt.Sprintf("%d.%d", tablo.AcceptedMajor, tablo.AcceptedMinor)}}
	case "validate":
		res, err = cmdValidate(o)
	case "audit":
		res, err = cmdAudit(o)
	case "view":
		res, err = cmdView(o, rest)
	case "status":
		res, err = cmdStatus(o)
	case "history":
		res, err = cmdHistory(o)
	case "trailer":
		res, err = cmdTrailer(o, rest)
	default:
		err = usageError{"unknown command " + cmd}
	}
	if err != nil {
		return fail(stderr, err)
	}
	// The trailer line is the one output a script pastes into a commit, so it
	// is plain unless a structured format is asked for.
	if res.plain != "" && !structured {
		_, _ = fmt.Fprintln(stdout, res.plain)
		return 0
	}
	env := map[string]any{
		"schema":      Schema,
		"command":     cmd,
		"data":        res.data,
		"diagnostics": orEmpty(res.diags),
	}
	if cmd != "version" && cmd != "trailer" {
		hash, herr := git(o, "rev-parse", "--verify", o.ref+"^{commit}")
		if herr != nil {
			return fail(stderr, herr)
		}
		env["ref"] = map[string]any{"name": o.ref, "commit": hash}
	}
	if o.yaml {
		_, _ = fmt.Fprint(stdout, toYAML(env, 0))
	} else {
		b, _ := json.MarshalIndent(env, "", "  ")
		_, _ = fmt.Fprintln(stdout, string(b))
	}
	for _, d := range res.diags {
		if d["severity"] == "error" {
			return 1
		}
	}
	return 0
}

func fail(w io.Writer, err error) int {
	_, _ = fmt.Fprintf(w, "tablo: %v\n", err)
	if _, ok := err.(usageError); ok {
		_, _ = fmt.Fprintln(w, usage)
		return 2
	}
	return 3
}

func orEmpty(d []map[string]any) []map[string]any {
	if d == nil {
		return []map[string]any{}
	}
	return d
}

// git runs git in the repository and returns trimmed standard output.
func git(o options, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", o.dir}, args...)...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// show returns a file of the project at the ref, through git cat-file.
func show(o options, path string) (string, error) {
	return git(o, "cat-file", "blob", o.ref+":.tableaux/"+path)
}

// files lists the file names in a .tableaux subdirectory at the ref.
func files(o options, dir string) ([]string, error) {
	out, err := git(o, "ls-tree", "--name-only", o.ref+":.tableaux/"+dir)
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

// scalars reads the top-level "key: value" lines of a YAML file and one level
// of inline-map "key: { id: "x" }" as key.id. It stands in for a YAML parser.
func scalars(src string) map[string]string {
	m := map[string]string{}
	inline := regexp.MustCompile(`^\{\s*id:\s*"?([0-9a-f]{4})"?`)
	for _, l := range strings.Split(src, "\n") {
		k, v, ok := strings.Cut(l, ": ")
		if !ok || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "-") {
			continue
		}
		v = strings.TrimSpace(v)
		if sub := inline.FindStringSubmatch(v); sub != nil {
			m[k+".id"] = sub[1]
			continue
		}
		m[k] = strings.Trim(v, `"`)
	}
	return m
}

func taskIDs(o options) ([]string, error) {
	fs, err := files(o, "tasks")
	var ids []string
	for _, f := range fs {
		ids = append(ids, strings.TrimSuffix(f, ".yaml"))
	}
	return ids, err
}

// gateKeys returns the gate keys of gates.yaml in order.
func gateKeys(o options) ([]string, error) {
	src, err := show(o, "gates.yaml")
	if err != nil {
		return nil, err
	}
	var keys []string
	re := regexp.MustCompile(`^\s*-\s*\{\s*key:\s*(\w+),\s*symbol`)
	for _, l := range strings.Split(src, "\n") {
		if m := re.FindStringSubmatch(l); m != nil {
			keys = append(keys, m[1])
		}
	}
	return keys, nil
}

func cmdView(o options, args []string) (result, error) {
	if len(args) != 1 || args[0] != "tasks" {
		return result{}, usageError{"view needs a name: tasks"}
	}
	ids, err := taskIDs(o)
	if err != nil {
		return result{}, err
	}
	var tasks []map[string]any
	for _, id := range ids {
		src, err := show(o, "tasks/"+id+".yaml")
		if err != nil {
			return result{}, err
		}
		s := scalars(src)
		t := map[string]any{"id": id, "title": s["title"], "assignee": s["assignee"], "parent": nil}
		if p := s["parent.id"]; p != "" {
			t["parent"] = p
		}
		tasks = append(tasks, t)
	}
	return result{data: map[string]any{"view": "tasks", "tasks": tasks}}, nil
}

func cmdStatus(o options) (result, error) {
	fs, err := files(o, "status")
	if err != nil {
		return result{}, err
	}
	var out []map[string]any
	for _, f := range fs {
		src, err := show(o, "status/"+f)
		if err != nil {
			return result{}, err
		}
		s := scalars(src)
		out = append(out, map[string]any{"id": strings.TrimSuffix(f, ".yaml"), "gate": s["gate"], "state": s["state"], "reason": s["reason"], "note": s["note"]})
	}
	return result{data: map[string]any{"statuses": out}}, nil
}

// cmdValidate stubs the validator: the version rule, and that every task and
// status file names a known id. Each diagnostic has a stable code.
func cmdValidate(o options) (result, error) {
	var r result
	add := func(sev, code, path, msg string) {
		r.diags = append(r.diags, map[string]any{"severity": sev, "code": code, "path": path, "message": msg})
	}
	src, err := show(o, "version.yaml")
	if err != nil {
		return r, err
	}
	if v := scalars(src)["tableaux"]; !tablo.Accepts(v) {
		add("error", "version.unsupported", ".tableaux/version.yaml", "tableaux "+v+" is not accepted")
	}
	ids, err := taskIDs(o)
	if err != nil {
		return r, err
	}
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
		if !idRe.MatchString(id) {
			add("error", "task.id", ".tableaux/tasks/"+id+".yaml", "the file name is not a task id")
		}
	}
	fs, _ := files(o, "status")
	for _, f := range fs {
		if id := strings.TrimSuffix(f, ".yaml"); !known[id] {
			add("error", "status.orphan", ".tableaux/status/"+f, "no task "+id)
		}
	}
	r.data = map[string]any{"checked": len(ids) + len(fs)}
	return r, nil
}

// cmdAudit stubs the audit: a finding for a trailer that names an unknown task.
func cmdAudit(o options) (result, error) {
	h, err := history(o)
	if err != nil {
		return result{}, err
	}
	ids, err := taskIDs(o)
	if err != nil {
		return result{}, err
	}
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	var r result
	for _, c := range h {
		for _, t := range c["trailers"].([]map[string]any) {
			if id := t["task"].(string); !known[id] {
				r.diags = append(r.diags, map[string]any{"severity": "warning", "code": "trailer.unknown-task", "path": c["commit"], "message": "trailer names no task: " + id})
			}
		}
	}
	r.data = map[string]any{"commits": len(h)}
	return r, nil
}

var trailerRe = regexp.MustCompile(`^(Authorised|Reviewed|Reaffirmed): ([0-9a-f]{4})(?: (\w+))?$`)

// history lists the commits reachable from the ref, newest first, with their
// method trailers.
func history(o options) ([]map[string]any, error) {
	out, err := git(o, "log", "--format=%x1e%H%x1f%an%x1f%s%x1f%b", o.ref)
	if err != nil {
		return nil, err
	}
	var cs []map[string]any
	for _, rec := range strings.Split(out, "\x1e")[1:] {
		f := strings.SplitN(rec, "\x1f", 4)
		ts := []map[string]any{}
		for _, l := range strings.Split(f[3], "\n") {
			if m := trailerRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
				t := map[string]any{"kind": strings.ToLower(m[1]), "task": m[2]}
				if m[3] != "" {
					t["gate"] = m[3]
				}
				ts = append(ts, t)
			}
		}
		cs = append(cs, map[string]any{"commit": f[0], "author": f[1], "subject": f[2], "trailers": ts})
	}
	return cs, nil
}

func cmdHistory(o options) (result, error) {
	h, err := history(o)
	return result{data: map[string]any{"commits": h}}, err
}

// cmdTrailer composes one trailer line per SYNTAX.md#commit-trailers. It reads
// nothing when the gate needs no check; with -C pointing at a project it checks
// the task and the gate against the ref.
func cmdTrailer(o options, args []string) (result, error) {
	if len(args) < 2 {
		return result{}, usageError{"trailer needs a kind and a task id"}
	}
	kind, id, rest := args[0], args[1], args[2:]
	if !idRe.MatchString(id) {
		return result{}, usageError{"task id " + strconv.Quote(id) + " is not four lowercase hexadecimal digits"}
	}
	var line string
	switch kind {
	case "authorised", "reaffirmed":
		if len(rest) != 0 {
			return result{}, usageError{kind + " takes no gate"}
		}
		line = map[string]string{"authorised": "Authorised", "reaffirmed": "Reaffirmed"}[kind] + ": " + id
	case "reviewed":
		if len(rest) != 1 || !gateRe.MatchString(rest[0]) {
			return result{}, usageError{"reviewed needs a gate key"}
		}
		if keys, err := gateKeys(o); err == nil {
			if !contains(keys, rest[0]) {
				return result{}, fmt.Errorf("gate %q is not in gates.yaml", rest[0])
			}
		}
		line = "Reviewed: " + id + " " + rest[0]
	default:
		return result{}, usageError{"unknown trailer " + kind}
	}
	return result{plain: line, data: map[string]any{"line": line}}, nil
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// toYAML renders the generic value the envelope holds, keys sorted, strings
// always quoted so no scalar changes type.
func toYAML(v any, ind int) string {
	pad := strings.Repeat("  ", ind)
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			return "{}\n"
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for _, k := range keys {
			b.WriteString(pad + k + ":" + child(x[k], ind))
		}
		return b.String()
	case []map[string]any:
		items := make([]any, len(x))
		for i, m := range x {
			items[i] = m
		}
		return toYAML(items, ind)
	case []any:
		if len(x) == 0 {
			return "[]\n"
		}
		var b strings.Builder
		for _, e := range x {
			s := toYAML(e, ind+1)
			b.WriteString(pad + "- " + strings.TrimPrefix(s, strings.Repeat("  ", ind+1)))
		}
		return b.String()
	}
	return scalar(v) + "\n"
}

func child(v any, ind int) string {
	switch x := v.(type) {
	case map[string]any:
		if len(x) > 0 {
			return "\n" + toYAML(v, ind+1)
		}
	case []map[string]any:
		if len(x) > 0 {
			return "\n" + toYAML(v, ind)
		}
	case []any:
		if len(x) > 0 {
			return "\n" + toYAML(v, ind)
		}
	}
	return " " + toYAML(v, ind)
}

func scalar(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case int, bool:
		return fmt.Sprint(x)
	case string:
		return strconv.Quote(x)
	}
	return strconv.Quote(fmt.Sprint(v))
}
