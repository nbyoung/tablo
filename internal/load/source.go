package load

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nbyoung/tablo/internal/git"
	"github.com/nbyoung/tablo/internal/model"
)

// planDir is the directory that holds a project's files.
const planDir = ".tableaux"

// codeStrayPath is the rule of RULES.md for a path under .tableaux that is
// none the layout names; it is a warning, and the path stays unread.
const codeStrayPath = "L4"

// content is one file of the layout as a source gives it.
type content struct {
	path string // below .tableaux
	data []byte
	err  error // the file is there and the source cannot read it
}

// listing is what a source holds under one .tableaux directory.
type listing struct {
	exists bool      // the directory is there
	files  []content // the files the layout names, by path
	stray  []string  // every other path, by path
}

// The files of the layout, by what the Loader makes of each.
const (
	strayFile = iota
	versionFile
	gatesFile
	taskFile
	statusFile
)

// layout says which file of the layout a path below .tableaux is, and the id
// a task or status file carries: its name without .yaml, whatever it is.
func layout(p string) (kind int, id string) {
	switch p {
	case "version.yaml":
		return versionFile, ""
	case "gates.yaml":
		return gatesFile, ""
	}
	dir, name := path.Split(p)
	id, ok := strings.CutSuffix(name, ".yaml")
	switch {
	case !ok:
		return strayFile, ""
	case dir == "tasks/":
		return taskFile, id
	case dir == "status/":
		return statusFile, id
	}
	return strayFile, ""
}

// planPath returns the path of the .tableaux directory of the project in dir,
// relative to the repository root.
func planPath(dir string) string {
	return path.Join(dir, planDir)
}

// listTree lists the project in dir of treeish and reads its files, in two
// processes: one ls-tree and one blob batch.
func (s *session) listTree(ctx context.Context, repo git.Repo, treeish, dir string) (listing, error) {
	_, l, err := s.listNearest(ctx, repo, treeish, []string{dir})
	return l, err
}

// listNearest lists the project in the first of dirs that holds a .tableaux
// directory in treeish, and says which it is. dirs runs from a directory up
// to the repository root, so the first is the nearest. One ls-tree covers
// every candidate, since their .tableaux directories never nest.
func (s *session) listNearest(ctx context.Context, repo git.Repo, treeish string, dirs []string) (string, listing, error) {
	plans := make([]string, len(dirs))
	for i, dir := range dirs {
		plans[i] = planPath(dir)
	}
	entries, err := s.run.LsTree(ctx, repo, treeish, true, plans...)
	if err != nil {
		return "", listing{}, err
	}
	for i, plan := range plans {
		var l listing
		var ids []string
		for _, e := range entries {
			below, ok := strings.CutPrefix(e.Path, plan+"/")
			if !ok {
				continue
			}
			l.exists = true
			if kind, _ := layout(below); kind == strayFile || !e.Regular() {
				l.stray = append(l.stray, below)
				continue
			}
			l.files = append(l.files, content{path: below})
			ids = append(ids, e.ID)
		}
		if !l.exists {
			continue
		}
		blobs, err := s.run.Blobs(ctx, repo, ids)
		if err != nil {
			return "", listing{}, err
		}
		for j := range l.files {
			l.files[j].data = blobs[j]
		}
		l.sort()
		return dirs[i], l, nil
	}
	return dirs[0], listing{}, nil
}

// nearestOnDisk returns the first of dirs whose .tableaux is a directory
// under top, or the first of dirs when none is.
func nearestOnDisk(top string, dirs []string) string {
	for _, dir := range dirs {
		if info, err := os.Lstat(filepath.Join(top, filepath.FromSlash(planPath(dir)))); err == nil && info.IsDir() {
			return dir
		}
	}
	return dirs[0]
}

// listDisk lists the project in dir of the working tree at top and reads its
// files, untracked ones included. A symbolic link and any other path that is
// no regular file is stray, as it is in a tree.
func listDisk(top, dir string) (listing, error) {
	root := filepath.Join(top, filepath.FromSlash(planPath(dir)))
	if info, err := os.Lstat(root); err != nil || !info.IsDir() {
		return listing{}, nil
	}
	l := listing{exists: true}
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			if file == root {
				return err
			}
			// A directory the walk cannot enter is a path the Loader leaves unread.
			if rel, relErr := filepath.Rel(root, file); relErr == nil {
				l.stray = append(l.stray, filepath.ToSlash(rel))
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		below := filepath.ToSlash(rel)
		if kind, _ := layout(below); kind == strayFile || !entry.Type().IsRegular() {
			l.stray = append(l.stray, below)
			return nil
		}
		data, err := os.ReadFile(file)
		l.files = append(l.files, content{path: below, data: data, err: err})
		return nil
	})
	if err != nil {
		return listing{}, err
	}
	l.sort()
	return l, nil
}

// drop removes the files and the stray paths that paths names. Each of paths
// is relative to the working tree's root and lies under plan, the project's
// .tableaux directory there.
func (l *listing) drop(plan string, paths []string) {
	if len(paths) == 0 {
		return
	}
	gone := make(map[string]bool, len(paths))
	for _, p := range paths {
		gone[strings.TrimPrefix(p, plan+"/")] = true
	}
	files := l.files[:0]
	for _, f := range l.files {
		if !gone[f.path] {
			files = append(files, f)
		}
	}
	l.files = files
	stray := l.stray[:0]
	for _, p := range l.stray {
		if !gone[p] {
			stray = append(stray, p)
		}
	}
	l.stray = stray
}

// sort puts the files and the stray paths in byte order of their paths.
func (l *listing) sort() {
	sort.Slice(l.files, func(i, j int) bool { return l.files[i].path < l.files[j].path })
	sort.Strings(l.stray)
}

// candidates returns dir and each directory above it, up to the repository
// root, which is "".
func candidates(dir string) []string {
	dirs := []string{dir}
	for dir != "" {
		dir = path.Dir(dir)
		if dir == "." {
			dir = ""
		}
		dirs = append(dirs, dir)
	}
	return dirs
}

// Piece is one file of the layout, parsed: the file, what it builds and the
// diagnostics it earns. A Piece is immutable.
type Piece struct {
	kind        int    // which file of the layout it is
	id          string // the id a task or status file carries
	file        *model.File
	version     *model.Version
	gating      *model.Gating
	task        *model.Task
	status      *model.Status
	diagnostics []model.Diagnostic
}

// Parse reads data as the file at path, a path below .tableaux. It returns
// nil for a path the layout does not name, whatever the data: such a path is
// stray, and a tool leaves it unread.
func Parse(path string, data []byte) *Piece {
	return parse(content{path: path, data: data})
}

// parse reads one file as a source gives it. A file the source cannot read
// earns the diagnostic of a file that is no YAML.
func parse(c content) *Piece {
	kind, id := layout(c.path)
	if kind == strayFile {
		return nil
	}
	piece := &Piece{kind: kind, id: id, file: &model.File{Path: c.path}}
	var parsed bool
	if c.err != nil {
		piece.diagnostics = []model.Diagnostic{{
			Code:     codeNotYAML,
			Severity: model.Error,
			Pos:      model.Pos{File: c.path},
			Message:  "the file cannot be read: " + c.err.Error(),
		}}
	} else {
		piece.file.Root, parsed, piece.diagnostics = readYAML(c.path, c.data)
	}
	if parsed {
		switch kind {
		case versionFile:
			var more []model.Diagnostic
			piece.version, more = buildVersion(piece.file)
			piece.diagnostics = append(piece.diagnostics, more...)
		case gatesFile:
			piece.gating = buildGating(piece.file)
		case taskFile:
			piece.task = buildTask(id, piece.file)
		case statusFile:
			piece.status = buildStatus(id, piece.file)
		}
	}
	for i := range piece.diagnostics {
		piece.diagnostics[i].Task = id
	}
	return piece
}

// Compose joins pieces into the project they state, without its links: the
// project's Links is nil and each Subproject.Link is nil. stray lists the
// paths under .tableaux that the layout does not name. The project exists
// when it holds a piece or a stray path, and its files stand in the order of
// pieces, which a caller gives by path. A nil piece is left out.
func Compose(where model.Location, pieces []*Piece, stray []string) *model.Project {
	p := &model.Project{
		Where:    where,
		Exists:   len(pieces) > 0 || len(stray) > 0,
		Stray:    stray,
		Tasks:    map[string]*model.Task{},
		Statuses: map[string]*model.Status{},
	}
	for _, path := range stray {
		p.Diagnostics = append(p.Diagnostics, model.Diagnostic{
			Code:     codeStrayPath,
			Severity: model.Warning,
			Pos:      model.Pos{File: path},
			Message:  "the layout names no such path; a tool leaves it unread",
		})
	}
	for _, piece := range pieces {
		if piece == nil {
			continue
		}
		p.Files = append(p.Files, piece.file)
		switch {
		case piece.version != nil:
			p.Version = piece.version
		case piece.gating != nil:
			p.Gating = piece.gating
		case piece.task != nil:
			p.Tasks[piece.id] = piece.task
		case piece.status != nil:
			p.Statuses[piece.id] = piece.status
		}
		p.Diagnostics = append(p.Diagnostics, piece.diagnostics...)
	}
	sort.SliceStable(p.Diagnostics, func(i, j int) bool {
		a, b := p.Diagnostics[i], p.Diagnostics[j]
		switch {
		case a.Pos.File != b.Pos.File:
			return a.Pos.File < b.Pos.File
		case a.Pos.Line != b.Pos.Line:
			return a.Pos.Line < b.Pos.Line
		case a.Pos.Col != b.Pos.Col:
			return a.Pos.Col < b.Pos.Col
		}
		return a.Code < b.Code
	})
	return p
}

// assemble parses the files of a listing and builds the project they state,
// without its links. It returns the subproject fields the files hold, by
// task id and then in file order, for the Loader to resolve.
func assemble(where model.Location, l listing) (*model.Project, []*model.Subproject) {
	if !l.exists {
		return &model.Project{Where: where}, nil
	}
	pieces := make([]*Piece, len(l.files))
	for i, c := range l.files {
		pieces[i] = parse(c)
	}
	p := Compose(where, pieces, l.stray)
	p.Exists = true
	var fields []*model.Subproject
	for _, id := range p.TaskIDs() {
		fields = append(fields, subprojects(p.Tasks[id])...)
	}
	return p, fields
}
