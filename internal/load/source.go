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

// assemble parses the files of a listing and builds the project they state,
// without its links. It returns the subproject fields the files hold, by
// task id and then in file order, for the Loader to resolve.
func assemble(where model.Location, l listing) (*model.Project, []*model.Subproject) {
	p := &model.Project{Where: where, Exists: l.exists}
	if !l.exists {
		return p, nil
	}
	p.Stray = l.stray
	p.Tasks = map[string]*model.Task{}
	p.Statuses = map[string]*model.Status{}
	for _, stray := range l.stray {
		p.Diagnostics = append(p.Diagnostics, model.Diagnostic{
			Code:     codeStrayPath,
			Severity: model.Warning,
			Pos:      model.Pos{File: stray},
			Message:  "the layout names no such path; a tool leaves it unread",
		})
	}
	for _, c := range l.files {
		kind, id := layout(c.path)
		file := &model.File{Path: c.path}
		p.Files = append(p.Files, file)
		var parsed bool
		var diagnostics []model.Diagnostic
		if c.err != nil {
			diagnostics = []model.Diagnostic{{
				Code:     codeNotYAML,
				Severity: model.Error,
				Pos:      model.Pos{File: c.path},
				Message:  "the file cannot be read: " + c.err.Error(),
			}}
		} else {
			file.Root, parsed, diagnostics = readYAML(c.path, c.data)
		}
		if parsed {
			switch kind {
			case versionFile:
				var more []model.Diagnostic
				p.Version, more = buildVersion(file)
				diagnostics = append(diagnostics, more...)
			case gatesFile:
				p.Gating = buildGating(file)
			case taskFile:
				p.Tasks[id] = buildTask(id, file)
			case statusFile:
				p.Statuses[id] = buildStatus(id, file)
			}
		}
		for i := range diagnostics {
			diagnostics[i].Task = id
		}
		p.Diagnostics = append(p.Diagnostics, diagnostics...)
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
	var fields []*model.Subproject
	for _, id := range p.TaskIDs() {
		fields = append(fields, subprojects(p.Tasks[id])...)
	}
	return p, fields
}
