// Package load is the Loader: it reads one Tableaux project, from the working
// tree or from any Git revision, and returns it as an immutable model with
// every project its files link to. It reads files and never history. It
// reports what concerns reading, the rules L1 to L4 of RULES.md, and the one
// content rule its task gives it, P4; every other rule is the validator's.
package load

import (
	"container/list"
	"context"
	"errors"
	"sync"

	"github.com/nbyoung/tablo/internal/git"
	"github.com/nbyoung/tablo/internal/model"
)

// Source names what to read.
type Source struct {
	Dir string // a directory in the repository; "" is the current directory
	Ref string // a revision or a tree; "" reads the working tree
}

// Options configure a Loader.
type Options struct {
	Replace  map[string]string // absolute URL to the path of a clone; wins over Git's configuration
	Fetch    bool              // fetch a URL that has no clone into the cache; off by default
	CacheDir string            // "" is os.UserCacheDir()/tableaux
	Git      string            // the executable; "" is "git"
}

// Loader reads projects. It is safe for concurrent use, and the projects it returns are
// immutable and may be shared between results.
type Loader struct {
	options Options
	run     git.Runner

	mu    sync.Mutex
	memo  map[key]*list.Element // of *remembered
	order *list.List            // the most recently used first
}

// New returns a Loader with the options given.
func New(options Options) *Loader {
	replace := make(map[string]string, len(options.Replace))
	for url, clone := range options.Replace {
		replace[url] = clone
	}
	options.Replace = replace
	return &Loader{
		options: options,
		run:     git.Runner{Exe: options.Git},
		memo:    map[key]*list.Element{},
		order:   list.New(),
	}
}

// Error is a failure to find what to read. Err is one of the four below.
type Error struct {
	Err    error
	Dir    string
	Ref    string
	Detail string // what git wrote to standard error
}

// Error returns the failure, the directory and the revision, and what git wrote.
func (e *Error) Error() string {
	text := e.Err.Error()
	if e.Dir != "" {
		text += ": " + e.Dir
	}
	if e.Ref != "" {
		text += " at " + e.Ref
	}
	if e.Detail != "" {
		text += ": " + e.Detail
	}
	return text
}

// Unwrap returns Err, so that errors.Is finds which of the four it is.
func (e *Error) Unwrap() error { return e.Err }

// The four failures to find what to read.
var (
	ErrNoGit        = errors.New("git is not on the path")
	ErrNoRepository = errors.New("not in a Git repository")
	ErrNoRef        = errors.New("the revision names no commit or tree")
	ErrNoWorktree   = errors.New("a bare repository has no working tree")
)

// maxDepth is how many links from the project first asked for the Loader follows.
const maxDepth = 8

// memoSize is how many projects a Loader remembers.
const memoSize = 256

// key identifies one reading of one project.
type key struct {
	gitDir  string // the repository's common Git directory
	treeish string // the commit or the tree read; "" for the working tree
	top     string // the working tree's root, for a reading of the working tree
	dir     string // the directory that holds .tableaux
}

// place is a repository as the Loader reaches it.
type place struct {
	repo   git.Repo // how a command addresses it
	gitDir string   // its Git directory, where the clones of its submodules sit
	common string   // its common Git directory
	top    string   // its working tree's root; "" when the Loader knows none
}

// node is one project of a load and what the Loader needs to resolve its links.
type node struct {
	project *model.Project
	key     key
	place   place
	depth   int                 // links from the project first asked for
	fields  []*model.Subproject // the subproject fields its files hold
}

// session is one call of Load.
type session struct {
	loader *Loader
	run    git.Runner
	home   place                  // the repository of the project first asked for
	root   key                    // the reading first asked for
	seen   map[key]*model.Project // every project of this load
	keys   map[*model.Project]key
	queue  []*node           // read, with links to resolve, nearest first
	fresh  []*node           // read by this load at a commit
	paths  map[string]string // absolute URL to clone path, from Git's configuration; nil until read
}

// Load reads the project at src and every project it links to.
func (l *Loader) Load(ctx context.Context, src Source) (*model.Project, error) {
	dir := src.Dir
	if dir == "" {
		dir = "."
	}
	fail := func(err error, detail string) (*model.Project, error) {
		return nil, &Error{Err: err, Dir: src.Dir, Ref: src.Ref, Detail: detail}
	}
	rev := src.Ref
	if rev == "" {
		rev = "HEAD"
	}
	found, err := l.run.Discover(ctx, git.Repo{Dir: dir}, rev)
	switch {
	case errors.Is(err, git.ErrNotFound):
		return fail(ErrNoGit, err.Error())
	case errors.Is(err, git.ErrNoRepository):
		return fail(ErrNoRepository, err.Error())
	case err != nil:
		return nil, err
	}
	s := &session{
		loader: l,
		run:    l.run,
		home:   place{repo: git.Repo{Dir: dir}, gitDir: found.GitDir, common: found.CommonDir, top: found.Top},
		seen:   map[key]*model.Project{},
		keys:   map[*model.Project]key{},
	}
	if found.Top != "" {
		s.home.repo = git.Repo{Dir: found.Top}
	}
	where := model.Location{GitDir: found.CommonDir, Top: found.Top, Ref: src.Ref, Commit: found.Commit}
	dirs := candidates(found.Prefix)

	var root *node
	if src.Ref == "" {
		if !found.InWorkTree {
			return fail(ErrNoWorktree, "")
		}
		where.Worktree = true
		where.Dir = nearestOnDisk(found.Top, dirs)
		root, err = s.readDisk(s.home, where, 0)
	} else {
		treeish := found.Commit
		if treeish == "" {
			var ok bool
			if treeish, ok, err = l.run.Tree(ctx, s.home.repo, src.Ref); err != nil {
				return nil, err
			} else if !ok {
				return fail(ErrNoRef, found.Detail)
			}
		}
		var files listing
		if where.Dir, files, err = s.listNearest(ctx, s.home.repo, treeish, dirs); err == nil {
			root = s.add(s.home, key{gitDir: where.GitDir, treeish: treeish, dir: where.Dir}, where, files, 0)
		}
	}
	if err != nil {
		return nil, err
	}
	s.root = root.key
	for len(s.queue) > 0 {
		n := s.queue[0]
		s.queue = s.queue[1:]
		if err := s.resolve(ctx, n); err != nil {
			return nil, err
		}
	}
	s.remember()
	return root.project, nil
}

// add builds the project a listing states, records it as a project of this
// load, and queues it for the resolution of its links.
func (s *session) add(at place, k key, where model.Location, l listing, depth int) *node {
	project, fields := assemble(where, l)
	n := &node{project: project, key: k, place: at, depth: depth, fields: fields}
	s.seen[k] = project
	s.keys[project] = k
	s.queue = append(s.queue, n)
	if k.treeish != "" {
		s.fresh = append(s.fresh, n)
	}
	return n
}

// readDisk reads the project where names from the working tree.
func (s *session) readDisk(at place, where model.Location, depth int) (*node, error) {
	l, err := listDisk(where.Top, where.Dir)
	if err != nil {
		return nil, err
	}
	k := key{gitDir: where.GitDir, top: where.Top, dir: where.Dir}
	return s.add(at, k, where, l, depth), nil
}

// project returns the project in dir of treeish in the repository at, for a
// link at depth links from the project first asked for. It takes it from this
// load, else from the memo, else from the repository in two processes. commit
// is the commit treeish is, or "" when it is a bare tree.
func (s *session) project(ctx context.Context, at place, treeish, commit, dir string, depth int) (*model.Project, error) {
	k := key{gitDir: at.common, treeish: treeish, dir: dir}
	if p, ok := s.seen[k]; ok {
		return p, nil
	}
	if p := s.recall(k, depth); p != nil {
		return p, nil
	}
	l, err := s.listTree(ctx, at.repo, treeish, dir)
	if err != nil {
		return nil, err
	}
	where := model.Location{GitDir: at.common, Dir: dir, Ref: treeish, Commit: commit}
	return s.add(at, k, where, l, depth).project, nil
}

// remembered is one project the memo holds, with what decides whether a load
// may take it.
type remembered struct {
	key     key
	project *model.Project
	graph   map[key]*model.Project // every project its links reach, itself included
	height  int                    // the most links from it to any of them
}

// recall takes a project from the memo for a link at depth, and records every
// project it reaches as a project of this load. It takes none whose links
// would run deeper than a fresh read follows them, and none that reaches the
// reading first asked for, which this load holds under its own Location.
func (s *session) recall(k key, depth int) *model.Project {
	l := s.loader
	l.mu.Lock()
	defer l.mu.Unlock()
	element, ok := l.memo[k]
	if !ok {
		return nil
	}
	r := element.Value.(*remembered)
	if _, reaches := r.graph[s.root]; reaches || depth+r.height > maxDepth {
		return nil
	}
	l.order.MoveToFront(element)
	for k, p := range r.graph {
		if _, ok := s.seen[k]; !ok {
			s.seen[k] = p
		}
		s.keys[p] = k
	}
	return r.project
}

// remember puts in the memo each project this load read at a commit whose
// links all resolved, through every project they reach. A project read at a
// commit never changes, and a link that did not resolve may resolve later, so
// the memo never serves a stale link. The project first asked for stays out:
// its Location names the revision as the caller wrote it.
func (s *session) remember() {
	l := s.loader
	for _, n := range s.fresh {
		if n.key == s.root || n.project.Where.Commit == "" {
			continue
		}
		graph, height, whole := s.reach(n.project)
		if !whole {
			continue
		}
		l.mu.Lock()
		if _, ok := l.memo[n.key]; !ok {
			l.memo[n.key] = l.order.PushFront(&remembered{key: n.key, project: n.project, graph: graph, height: height})
			if l.order.Len() > memoSize {
				last := l.order.Back()
				delete(l.memo, last.Value.(*remembered).key)
				l.order.Remove(last)
			}
		}
		l.mu.Unlock()
	}
}

// reach walks the links of p breadth first. It returns every project they
// reach and the most links to any of them. whole is false when a link has no
// project or leads to a reading the memo does not keep: the working tree, a
// bare tree, or the reading first asked for.
func (s *session) reach(p *model.Project) (graph map[key]*model.Project, height int, whole bool) {
	graph = map[key]*model.Project{s.keys[p]: p}
	level := []*model.Project{p}
	for ; len(level) > 0; height++ {
		var next []*model.Project
		for _, q := range level {
			for _, link := range q.Links {
				if link.Problem != model.Resolved {
					return nil, 0, false
				}
				k := s.keys[link.Project]
				if k == s.root || link.Project.Where.Commit == "" || link.Project.Where.Worktree {
					return nil, 0, false
				}
				if _, ok := graph[k]; !ok {
					graph[k] = link.Project
					next = append(next, link.Project)
				}
			}
		}
		level = next
	}
	return graph, height - 1, true
}
