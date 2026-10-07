package history

import (
	"container/list"
	"context"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/nbyoung/tablo/internal/git"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// memoSize is how many passes a Reader remembers.
const memoSize = 64

// Options configure a Reader.
type Options struct {
	Git string // the executable; "" is "git"
}

// Reader reads passes and remembers the last 64. It is safe for concurrent use.
type Reader struct {
	run git.Runner

	mu    sync.Mutex
	memo  map[key]*list.Element // of *Log
	order *list.List            // the most recently used first
}

// NewReader returns a Reader.
func NewReader(options Options) *Reader {
	return &Reader{run: git.Runner{Exe: options.Git}, memo: map[key]*list.Element{}, order: list.New()}
}

// key identifies one pass: a pass is a function of the repository, the
// directory, the tips and the paths, and a Log also states the trunk.
type key struct {
	gitDir, dir string
	tips, paths string // each joined by NUL
	trunk       Trunk
	shallow     bool
}

// Set holds the pass of every project of one load.
type Set struct {
	logs map[*model.Project]*Log
}

// Of returns the pass of p, or nil when p has no commit to read history from.
func (s *Set) Of(p *model.Project) *Log {
	if s == nil {
		return nil
	}
	return s.logs[p]
}

// Read makes the passes for p, the project a Load returns, and for every
// project its links reach. trunk is the branch the caller names, read for p
// alone. extra names further commits of p's repository to walk, as a range's
// start. It fails only when git fails.
func (r *Reader) Read(ctx context.Context, p *model.Project, trunk string, extra ...string) (*Set, error) {
	set := &Set{logs: map[*model.Project]*Log{}}
	if p == nil {
		return set, nil
	}
	seen := map[*model.Project]bool{p: true}
	queue := []*model.Project{p}
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		for _, link := range q.Links {
			if link.Project != nil && !seen[link.Project] {
				seen[link.Project] = true
				queue = append(queue, link.Project)
			}
		}
		if q.Where.Commit == "" || q.Where.GitDir == "" {
			continue
		}
		var log *Log
		var err error
		if q == p {
			log, err = r.read(ctx, q, false, trunk, extra)
		} else {
			// A linked repository is another one: a submodule's store or a
			// clone. A directory of the home repository shares its refs.
			log, err = r.read(ctx, q, q.Where.GitDir != p.Where.GitDir, "", nil)
		}
		if err != nil {
			return nil, err
		}
		set.logs[q] = log
	}
	return set, nil
}

// read makes the pass of one project: the ref listing, then the log and the
// blob batch unless the Reader remembers the pass.
func (r *Reader) read(ctx context.Context, p *model.Project, linked bool, caller string, extra []string) (*Log, error) {
	repo := git.Repo{GitDir: p.Where.GitDir}
	refs, err := r.run.Refs(ctx, repo, "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	trunk := resolve(p, refs, linked, caller)
	tips := []string{p.Where.Commit}
	if trunk.Tip != "" && trunk.Tip != p.Where.Commit {
		tips = append(tips, trunk.Tip)
	}
	tips = append(tips, extra...)
	plan := path.Join(p.Where.Dir, planDir)
	paths := []string{plan}
	for _, link := range p.Links {
		if link.Form == model.Submodule {
			paths = append(paths, link.URL)
		}
	}
	sort.Strings(paths)
	paths = compact(paths)
	_, statErr := os.Stat(filepath.Join(p.Where.GitDir, "shallow"))
	k := key{
		gitDir: p.Where.GitDir, dir: p.Where.Dir,
		tips: strings.Join(tips, "\x00"), paths: strings.Join(paths, "\x00"),
		trunk: trunk, shallow: statErr == nil,
	}
	if log := r.recall(k); log != nil {
		return log, nil
	}
	commits, err := r.run.Log(ctx, repo, tips, paths)
	if err != nil {
		return nil, err
	}
	log := &Log{
		GitDir: k.gitDir, Dir: k.dir, Source: p.Where.Commit, Trunk: trunk, Shallow: k.shallow,
		byID:     make(map[string]*Commit, len(commits)),
		tables:   make(map[string]table, len(commits)),
		blobs:    map[string][]byte{},
		pieces:   map[string]*load.Piece{},
		projects: map[string]*model.Project{},
	}
	var ids []string
	for i, raw := range commits {
		c := &Commit{
			ID: raw.ID, Parents: raw.Parents,
			Author:    Person{Name: raw.AuthorName, Email: raw.AuthorEmail},
			Committer: Person{Name: raw.CommitterName, Email: raw.CommitterEmail},
			Time:      raw.AuthorTime, Zone: raw.AuthorZone, Subject: raw.Subject,
			Seq: i, Trunk: -1,
		}
		for _, line := range raw.Trailers {
			c.Trailers = append(c.Trailers, trailer(line))
		}
		for _, change := range raw.Changes {
			c.Changes = append(c.Changes, Change{
				Path: change.Path, Old: present(change.Old), New: present(change.New),
				Gitlink: change.OldMode == git.ModeGitlink || change.NewMode == git.ModeGitlink,
			})
			below, inPlan := strings.CutPrefix(change.Path, plan+"/")
			if _, read := log.blobs[change.New]; inPlan && !read && present(change.New) != "" &&
				regular(change.NewMode) && load.Parse(below, nil) != nil {
				log.blobs[change.New] = nil
				ids = append(ids, change.New)
			}
		}
		sort.Slice(c.Changes, func(i, j int) bool { return c.Changes[i].Path < c.Changes[j].Path })
		log.Commits = append(log.Commits, c)
		log.byID[c.ID] = c
	}
	blobs, err := r.run.Blobs(ctx, repo, ids)
	if err != nil {
		return nil, err
	}
	for i, id := range ids {
		log.blobs[id] = blobs[i]
	}
	// No parent comes before its child, so the oldest commit comes first here
	// and each table builds on its first parent's.
	for i := len(commits) - 1; i >= 0; i-- {
		raw := commits[i]
		var held table
		if len(raw.Parents) > 0 {
			held = log.tables[raw.Parents[0]]
		}
		if len(raw.Changes) > 0 {
			next := make(table, len(held)+len(raw.Changes))
			for p, o := range held {
				next[p] = o
			}
			for _, change := range raw.Changes {
				if present(change.New) == "" {
					delete(next, change.Path)
				} else {
					next[change.Path] = object{id: change.New, regular: regular(change.NewMode)}
				}
			}
			held = next
		}
		if held == nil {
			held = table{}
		}
		log.tables[raw.ID] = held
	}
	// The source reaches a commit through any parent; the trunk's line runs
	// through first parents alone.
	if source := log.byID[log.Source]; source != nil {
		source.InSource = true
		for _, c := range log.Commits[source.Seq:] {
			if !c.InSource {
				continue
			}
			for _, id := range c.Parents {
				if parent := log.byID[id]; parent != nil {
					parent.InSource = true
				}
			}
		}
	}
	for c, place := log.byID[trunk.Tip], 0; c != nil; place++ {
		c.Trunk = place
		if len(c.Parents) == 0 {
			break
		}
		c = log.byID[c.Parents[0]]
	}
	r.remember(k, log)
	return log, nil
}

// recall takes a pass from the memo.
func (r *Reader) recall(k key) *Log {
	r.mu.Lock()
	defer r.mu.Unlock()
	element, ok := r.memo[k]
	if !ok {
		return nil
	}
	r.order.MoveToFront(element)
	return element.Value.(*remembered).log
}

// remembered is one pass the memo holds.
type remembered struct {
	key key
	log *Log
}

// remember puts a pass in the memo and drops the one used longest ago.
func (r *Reader) remember(k key, log *Log) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.memo[k]; ok {
		return
	}
	r.memo[k] = r.order.PushFront(&remembered{key: k, log: log})
	if r.order.Len() > memoSize {
		last := r.order.Back()
		delete(r.memo, last.Value.(*remembered).key)
		r.order.Remove(last)
	}
}

// resolve names the trunk as README.md#project orders it, version.yaml, then
// refs/remotes/origin/HEAD, then the caller's branch, and finds the ref that
// carries the name: the local branch, then origin's, then the other remotes'
// in byte order. In a linked repository the remote-tracking branches come
// first, since a fetch moves them and leaves a local branch behind.
func resolve(p *model.Project, refs []git.Ref, linked bool, caller string) Trunk {
	const heads, remotes, origin = "refs/heads/", "refs/remotes/", "refs/remotes/origin/"
	var t Trunk
	byName := make(map[string]git.Ref, len(refs))
	for _, ref := range refs {
		byName[ref.Name] = ref
	}
	inferred, _ := strings.CutPrefix(byName[origin+"HEAD"].Target, origin)
	switch {
	case p.Version != nil && p.Version.Trunk.Node.Scalar() && p.Version.Trunk.V != "":
		t.Name, t.From = p.Version.Trunk.V, Stated
	case inferred != "":
		t.Name, t.From = inferred, Inferred
	case caller != "":
		t.Name, t.From = caller, Caller
	default:
		return t
	}
	var others []string // refs is by name, so these are in byte order
	for _, ref := range refs {
		if strings.HasPrefix(ref.Name, remotes) && !strings.HasPrefix(ref.Name, origin) &&
			strings.HasSuffix(ref.Name, "/"+t.Name) && ref.Target == "" {
			others = append(others, ref.Name)
		}
	}
	candidates := append([]string{heads + t.Name, origin + t.Name}, others...)
	if linked {
		candidates = append(append([]string{origin + t.Name}, others...), heads+t.Name)
	}
	for _, name := range candidates {
		if ref, ok := byName[name]; ok && ref.Commit != "" {
			t.Ref, t.Tip = ref.Name, ref.Commit
			break
		}
	}
	return t
}

// present returns an object id, or "" for the zeros that mark an absent path.
func present(id string) string {
	if strings.Trim(id, "0") == "" {
		return ""
	}
	return id
}

// regular reports whether a mode is a regular file's.
func regular(mode string) bool { return strings.HasPrefix(mode, "100") }

// compact removes the repeats of a sorted list.
func compact(sorted []string) []string {
	out := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}
