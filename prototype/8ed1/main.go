// Command 8ed1 demonstrates the temporal views: the history and the audit
// bounded by a ref range, derived from one pass of git log.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Event is one row of the history, in the shape of schemas/history.schema.yaml.
// The pin event is not in that schema's enum (finding F12).
type Event struct {
	Date   string `json:"date"`
	Commit string `json:"commit"`
	By     string `json:"by"`
	Task   string `json:"task"`
	Event  string `json:"event"`
	Gate   string `json:"gate,omitempty"`
	State  string `json:"state,omitempty"`
	Reason string `json:"reason,omitempty"`
	Note   string `json:"note,omitempty"`

	full string // the full hash, for set tests
	at   int64  // the author time, for ordering
}

// Finding is a stand-in for what task dada derives.
type Finding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Task     string `json:"task"`
	Message  string `json:"message"`
	Since    string `json:"since,omitempty"`
	Range    string `json:"range,omitempty"` // introduced, standing or resolved
}

func git(repo string, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Stdin = bytes.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, errb.String())
	}
	return out.Bytes(), nil
}

var (
	taskPath    = regexp.MustCompile(`^\.tableaux/tasks/([0-9a-f]{4})\.yaml$`)
	statusPath  = regexp.MustCompile(`^\.tableaux/status/([0-9a-f]{4})\.yaml$`)
	trailerLine = regexp.MustCompile(`^(Authorised|Reaffirmed|Reviewed): ([0-9a-f]{4})(?: ([a-z][a-z0-9_-]*))?$`)
)

type rawChange struct {
	mode, sha, status, path string
}

type commit struct {
	full, date, by, body string
	at                   int64
	changes              []rawChange
}

// readLog runs one git log over the range and parses commits, oldest first.
// Merges show no diff, so their file changes are not counted twice; a merge
// still yields its trailers.
func readLog(repo, rng string) ([]commit, error) {
	out, err := git(repo, nil, "log", "--root", "--reverse", "--raw", "--no-abbrev", "--no-renames",
		"--format=%x01%H%x00%at%x00%as%x00%ae%x00%B%x02", rng)
	if err != nil {
		return nil, err
	}
	var cs []commit
	for _, chunk := range strings.Split(string(out), "\x01")[1:] {
		head, rest, _ := strings.Cut(chunk, "\x02")
		f := strings.SplitN(head, "\x00", 5)
		if len(f) != 5 {
			return nil, errors.New("malformed log record")
		}
		at, _ := strconv.ParseInt(f[1], 10, 64)
		c := commit{full: f[0], at: at, date: f[2], by: f[3], body: f[4]}
		for _, l := range strings.Split(rest, "\n") {
			if !strings.HasPrefix(l, ":") {
				continue
			}
			meta, path, _ := strings.Cut(l, "\t")
			m := strings.Fields(meta)
			if len(m) < 5 {
				continue
			}
			c.changes = append(c.changes, rawChange{mode: m[1], sha: m[3], status: m[4], path: path})
		}
		cs = append(cs, c)
	}
	return cs, nil
}

// blobs reads many blobs with one git cat-file process.
func blobs(repo string, shas []string) (map[string]string, error) {
	res := map[string]string{}
	if len(shas) == 0 {
		return res, nil
	}
	out, err := git(repo, []byte(strings.Join(shas, "\n")+"\n"), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(bytes.NewReader(out))
	for range shas {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		f := strings.Fields(line)
		if len(f) != 3 {
			continue // missing
		}
		n, _ := strconv.Atoi(f[2])
		buf := make([]byte, n+1)
		if _, err := readFull(r, buf); err != nil {
			return nil, err
		}
		res[f[0]] = string(buf[:n])
	}
	return res, nil
}

func readFull(r *bufio.Reader, b []byte) (int, error) {
	n := 0
	for n < len(b) {
		m, err := r.Read(b[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// parseStatus reads the flat subset of a status file: key: value lines.
func parseStatus(s string) map[string]string {
	m := map[string]string{}
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "#") || !strings.Contains(l, ":") {
			continue
		}
		k, v, _ := strings.Cut(l, ":")
		v = strings.TrimSpace(v)
		if len(v) > 1 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		} else if i := strings.Index(v, " #"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		m[strings.TrimSpace(k)] = v
	}
	return m
}

// History derives the events of the commits in a range.
func History(repo, rng string) ([]Event, error) {
	cs, err := readLog(repo, rng)
	if err != nil {
		return nil, err
	}
	var want []string
	for _, c := range cs {
		for _, ch := range c.changes {
			if statusPath.MatchString(ch.path) && ch.status != "D" {
				want = append(want, ch.sha)
			}
		}
	}
	bl, err := blobs(repo, want)
	if err != nil {
		return nil, err
	}
	evs := []Event{}
	for _, c := range cs {
		mk := func(task, ev string) Event {
			return Event{Date: c.date, Commit: c.full[:7], By: c.by, Task: task, Event: ev, full: c.full, at: c.at}
		}
		for _, ch := range c.changes {
			if m := taskPath.FindStringSubmatch(ch.path); m != nil {
				evs = append(evs, mk(m[1], "task"))
			}
		}
		for _, l := range strings.Split(strings.TrimSpace(c.body), "\n") {
			m := trailerLine.FindStringSubmatch(strings.TrimSpace(l))
			if m == nil {
				continue
			}
			e := mk(m[2], map[string]string{"Authorised": "authorised", "Reaffirmed": "reaffirmed", "Reviewed": "reviewed"}[m[1]])
			if m[1] == "Reviewed" {
				e.Gate = m[3]
			}
			if (m[1] == "Reviewed") == (m[3] != "") {
				evs = append(evs, e)
			}
		}
		for _, ch := range c.changes {
			if m := statusPath.FindStringSubmatch(ch.path); m != nil {
				e := mk(m[1], "status")
				if ch.status != "D" {
					v := parseStatus(bl[ch.sha])
					e.Gate, e.State, e.Reason, e.Note = v["gate"], v["state"], v["reason"], v["note"]
				}
				evs = append(evs, e)
			}
			if ch.mode == "160000" && ch.status != "D" {
				task, err := pinTask(repo, c.full, ch.path)
				if err != nil {
					return nil, err
				}
				e := mk(task, "pin")
				e.Note = "pin " + ch.path + " at " + ch.sha[:7]
				evs = append(evs, e)
			}
		}
	}
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].at < evs[j].at })
	return evs, nil
}

// pinTask finds the task whose recursive junction names the submodule path.
func pinTask(repo, commit, path string) (string, error) {
	out, err := git(repo, nil, "grep", "-l", "-e", "url: "+path+",", commit, "--", ".tableaux/tasks")
	if err != nil {
		return "", nil // grep found nothing
	}
	for _, l := range strings.Split(string(out), "\n") {
		if m := regexp.MustCompile(`([0-9a-f]{4})\.yaml$`).FindStringSubmatch(l); m != nil {
			return m[1], nil
		}
	}
	return "", nil
}

func filter(evs []Event, task, person string) []Event {
	out := []Event{}
	for _, e := range evs {
		if (task == "" || e.Task == task) && (person == "" || e.By == person) {
			out = append(out, e)
		}
	}
	return out
}

// Audit lists the stand-in findings that a range introduces, leaves standing or
// resolves. One log pass to the end ref serves both ends: the state at the start
// is the same events cut to the commits reachable from it.
func Audit(repo, from, to string, stale int) ([]Finding, error) {
	all, err := History(repo, to)
	if err != nil {
		return nil, err
	}
	trunk := set(repo, "rev-list", "--first-parent", to)
	var inFrom map[string]bool
	fromDate := ""
	if from != "" {
		inFrom = set(repo, "rev-list", from)
		out, err := git(repo, nil, "log", "-1", "--format=%as", from)
		if err != nil {
			return nil, err
		}
		fromDate = strings.TrimSpace(string(out))
	}
	toOut, err := git(repo, nil, "log", "-1", "--format=%as", to)
	if err != nil {
		return nil, err
	}
	var before []Event
	for _, e := range all {
		if inFrom[e.full] {
			before = append(before, e)
		}
	}
	auth := authorities(repo, to, all)
	endF := standIn(all, trunk, auth, strings.TrimSpace(string(toOut)), stale)
	startF := map[string]Finding{}
	if from != "" {
		for _, f := range standIn(before, trunk, auth, fromDate, stale) {
			startF[key(f)] = f
		}
	}
	out := []Finding{}
	seen := map[string]bool{}
	for _, f := range endF {
		seen[key(f)] = true
		f.Range = "introduced"
		if _, ok := startF[key(f)]; ok {
			f.Range = "standing"
		}
		out = append(out, f)
	}
	for k, f := range startF {
		if !seen[k] {
			f.Range = "resolved"
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return key(out[i]) < key(out[j]) })
	return out, nil
}

func key(f Finding) string { return f.Rule + "|" + f.Task }

func set(repo string, args ...string) map[string]bool {
	m := map[string]bool{}
	out, err := git(repo, nil, args...)
	if err != nil {
		return m
	}
	for _, l := range strings.Fields(string(out)) {
		m[l] = true
	}
	return m
}

// standIn holds three rules computed from the events alone. Task dada replaces
// it with the real audit.
func standIn(evs []Event, trunk map[string]bool, auth map[string]map[string]bool, today string, stale int) []Finding {
	if len(evs) == 0 {
		return nil
	}
	created := map[string]Event{}
	authorised := map[string]bool{}
	last := map[string]Event{}
	var out []Finding
	for _, e := range evs {
		switch e.Event {
		case "task":
			if _, ok := created[e.Task]; !ok {
				created[e.Task] = e
			}
			if trunk[e.full] && auth[e.Task][e.By] {
				authorised[e.Task] = true
			}
		case "authorised":
			authorised[e.Task] = true
		}
		if e.Event == "status" || e.Event == "reaffirmed" {
			last[e.Task] = e
		}
	}
	for _, e := range evs {
		if _, ok := created[e.Task]; !ok && e.Task != "" && e.Event != "pin" && e.Event != "task" {
			out = append(out, Finding{Rule: "H1", Severity: "warning", Task: e.Task, Since: e.Commit,
				Message: "The " + e.Event + " trailer names a task with no file"})
		}
	}
	td, _ := time.Parse(time.DateOnly, today)
	for id, e := range created {
		if !authorised[id] {
			out = append(out, Finding{Rule: "PROPOSED", Severity: "warning", Task: id, Since: e.Commit,
				Message: "The task stands proposed: no trailer or owner commit on the trunk authorises it"})
		}
	}
	for id, e := range last {
		d, err := time.Parse(time.DateOnly, e.Date)
		if err == nil && td.Sub(d) > time.Duration(stale)*24*time.Hour {
			out = append(out, Finding{Rule: "STALE", Severity: "warning", Task: id, Since: e.Commit,
				Message: fmt.Sprintf("The status dates from %s, more than %d days before %s, with no later reaffirmation", e.Date, stale, today)})
		}
	}
	return out
}

// resolve turns a label such as W3 into a hash, using <repo>.labels.txt when present.
func resolve(repo, ref string) string {
	if ref == "" {
		return ""
	}
	b, err := os.ReadFile(strings.TrimRight(repo, "/") + ".labels.txt")
	if err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if f := strings.Fields(l); len(f) == 2 && f[0] == ref {
				return f[1]
			}
		}
	}
	return ref
}

func main() {
	repo := flag.String("repo", "", "the Git repository")
	view := flag.String("view", "history", "history or audit")
	rng := flag.String("range", "", "FROM..TO, each a ref or a corpus label; FROM may be empty")
	task := flag.String("task", "", "history: one task")
	person := flag.String("person", "", "history: one actor")
	stale := flag.Int("stale", 14, "audit: the stale age in days")
	flag.Parse()
	from, to, ok := strings.Cut(*rng, "..")
	if !ok || *repo == "" || to == "" {
		fmt.Fprintln(os.Stderr, "usage: 8ed1 -repo DIR -view history|audit -range FROM..TO")
		os.Exit(2)
	}
	from, to = resolve(*repo, from), resolve(*repo, to)
	var v any
	var err error
	switch *view {
	case "history":
		var evs []Event
		spec := to
		if from != "" {
			spec = from + ".." + to
		}
		evs, err = History(*repo, spec)
		v = filter(evs, *task, *person)
	case "audit":
		v, err = Audit(*repo, from, to, *stale)
	default:
		err = errors.New("unknown view " + *view)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var (
	assigneeRe = regexp.MustCompile(`(?m)^assignee:\s*(\S+)`)
	parentRe   = regexp.MustCompile(`(?m)^parent:\s*\{\s*id:\s*"([0-9a-f]{4})"`)
)

// authorities is a stand-in for task 27a3: for each task, the assignees of its
// ancestors, read from the task files at the end ref (the assignee of a root is
// its own authority, standing for the owner).
func authorities(repo, ref string, evs []Event) map[string]map[string]bool {
	ids := map[string]bool{}
	for _, e := range evs {
		if e.Event == "task" {
			ids[e.Task] = true
		}
	}
	var specs []string
	for id := range ids {
		specs = append(specs, ref+":.tableaux/tasks/"+id+".yaml")
	}
	sort.Strings(specs)
	assignee, parent := map[string]string{}, map[string]string{}
	if len(specs) > 0 {
		out, err := git(repo, []byte(strings.Join(specs, "\n")+"\n"), "cat-file", "--batch")
		if err == nil {
			r := bufio.NewReader(bytes.NewReader(out))
			for _, sp := range specs {
				line, err := r.ReadString('\n')
				if err != nil {
					break
				}
				f := strings.Fields(line)
				if len(f) != 3 {
					continue
				}
				n, _ := strconv.Atoi(f[2])
				buf := make([]byte, n+1)
				if _, err := readFull(r, buf); err != nil {
					break
				}
				id := sp[strings.LastIndex(sp, "/")+1 : strings.LastIndex(sp, ".")]
				if m := assigneeRe.FindSubmatch(buf); m != nil {
					assignee[id] = string(m[1])
				}
				if m := parentRe.FindSubmatch(buf); m != nil {
					parent[id] = string(m[1])
				}
			}
		}
	}
	res := map[string]map[string]bool{}
	for id := range ids {
		res[id] = map[string]bool{}
		if parent[id] == "" {
			res[id][assignee[id]] = true
		}
		for p := parent[id]; p != ""; p = parent[p] {
			res[id][assignee[p]] = true
			if parent[p] == "" {
				break
			}
		}
	}
	return res
}
