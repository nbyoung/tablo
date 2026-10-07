package history

import (
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// planDir is the directory that holds a project's files.
const planDir = ".tableaux"

// object is what a tree holds at one path.
type object struct {
	id      string
	regular bool // a regular file: a symbolic link and a gitlink are none
}

// table is every watched path of one commit's tree. A commit that changes
// none shares its first parent's table.
type table map[string]object

// Log is one pass: the history of one project's source and of its trunk. It
// is immutable and safe for concurrent use.
type Log struct {
	GitDir  string // the repository's common Git directory
	Dir     string // the directory that holds .tableaux
	Source  string // the commit the view's history ends at, in full
	Trunk   Trunk
	Shallow bool      // the repository is a shallow clone: the pass may end early
	Commits []*Commit // the newest first, in Git's date order

	byID   map[string]*Commit
	tables map[string]table  // by commit
	blobs  map[string][]byte // by object id: every blob the pass names under .tableaux

	mu       sync.Mutex
	pieces   map[string]*load.Piece    // by path below .tableaux and object id
	projects map[string]*model.Project // by commit
}

// Commit returns the commit of the pass with the id, or nil.
func (l *Log) Commit(id string) *Commit {
	return l.byID[id]
}

// Object returns the id of the object at path as the tree of a commit of the
// pass holds it, or "".
func (l *Log) Object(commit, path string) string {
	return l.tables[commit][path].id
}

// Changed reports whether c changes path: against its parent or, for a
// merge, against every parent. A root commit changes each path it holds.
func (l *Log) Changed(c *Commit, path string) bool {
	own := l.Object(c.ID, path)
	known := false
	for _, parent := range c.Parents {
		if _, ok := l.byID[parent]; !ok {
			continue // a shallow clone ends here
		}
		known = true
		if l.Object(parent, path) == own {
			return false
		}
	}
	if known {
		return true
	}
	if len(c.Parents) == 0 {
		return own != ""
	}
	for _, change := range c.Changes {
		if change.Path == path {
			return true
		}
	}
	return false
}

// Project returns the project in Dir as its files stand at a commit of the
// pass, without links, or nil when the pass holds no such commit.
func (l *Log) Project(commit string) *model.Project {
	held, ok := l.tables[commit]
	if !ok {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if p, ok := l.projects[commit]; ok {
		return p
	}
	prefix := path.Join(l.Dir, planDir) + "/"
	var paths []string
	for p := range held {
		if strings.HasPrefix(p, prefix) {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var pieces []*load.Piece
	var stray []string
	for _, p := range paths {
		below := strings.TrimPrefix(p, prefix)
		at := held[p]
		data, read := l.blobs[at.id]
		if !at.regular || !read {
			stray = append(stray, below)
			continue
		}
		key := below + "\x00" + at.id
		piece, parsed := l.pieces[key]
		if !parsed {
			piece = load.Parse(below, data)
			l.pieces[key] = piece
		}
		if piece == nil {
			stray = append(stray, below)
			continue
		}
		pieces = append(pieces, piece)
	}
	where := model.Location{GitDir: l.GitDir, Dir: l.Dir, Ref: commit, Commit: commit}
	project := load.Compose(where, pieces, stray)
	l.projects[commit] = project
	return project
}

// Reaches reports whether to is from or one of its ancestors in the pass.
func (l *Log) Reaches(from, to string) bool {
	start, end := l.byID[from], l.byID[to]
	if start == nil || end == nil {
		return false
	}
	// No parent comes before its child, so no commit after end leads to it.
	seen := map[*Commit]bool{start: true}
	queue := []*Commit{start}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == end {
			return true
		}
		for _, id := range c.Parents {
			if parent := l.byID[id]; parent != nil && !seen[parent] && parent.Seq <= end.Seq {
				seen[parent] = true
				queue = append(queue, parent)
			}
		}
	}
	return false
}
