package load

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nbyoung/tablo/internal/git"
	"github.com/nbyoung/tablo/internal/model"
)

// absoluteURL matches a url with a scheme and ://; any other url is a path
// relative to the repository root.
var absoluteURL = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// fullHash matches a commit's full hash, in either object format.
var fullHash = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// target is one distinct subproject of a project: its link and the fields
// that name it.
type target struct {
	link   *model.Link
	stated string // an absolute URL: the commit field as written
	path   string // a path: the url cleaned; "" for an absolute URL or a path that is Outside
}

// resolve finds the project of every subproject field of n, sets each
// field's Link, and sets the project's Links, by URL then commit. Two fields
// that name one target share one Link: one path, whatever commit either
// states, or one absolute URL at one commit.
func (s *session) resolve(ctx context.Context, n *node) error {
	targets := map[string]*target{}
	var order []*target
	for _, field := range n.fields {
		if !field.URL.Node.Scalar() {
			continue
		}
		url := field.URL.V
		var id string
		t := &target{link: &model.Link{URL: url}}
		switch clean := path.Clean(url); {
		case absoluteURL.MatchString(url):
			t.link.Form = model.URL
			if field.Commit.Node.Scalar() {
				t.stated = field.Commit.V
			}
			id = "url\x00" + url + "\x00" + t.stated
		case url == "" || path.IsAbs(url) || clean == ".." || strings.HasPrefix(clean, "../"):
			t.link.Problem = model.Outside
			t.link.Detail = "the path is empty, absolute or leaves the repository"
			id = "outside\x00" + url
		default:
			t.link.URL, t.path = clean, clean
			id = "path\x00" + clean
		}
		if known, ok := targets[id]; ok {
			t = known
		} else {
			targets[id] = t
			order = append(order, t)
		}
		field.Link = t.link
	}
	if len(order) == 0 {
		return nil
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if a.link.URL != b.link.URL {
			return a.link.URL < b.link.URL
		}
		return a.stated < b.stated
	})
	var paths []*target
	for _, t := range order {
		n.project.Links = append(n.project.Links, t.link)
		switch {
		case t.link.Problem != model.Resolved:
		case t.link.Form == model.URL && !fullHash.MatchString(t.stated):
			t.link.Problem = model.BadCommit
			t.link.Detail = "an absolute URL needs a commit field that is a full hash"
		case n.depth >= maxDepth:
			t.link.Problem = model.TooDeep
			t.link.Detail = "more than eight links from the project first asked for"
		case t.link.Form == model.URL:
			if err := s.resolveURL(ctx, n, t); err != nil {
				return err
			}
		default:
			paths = append(paths, t)
		}
	}
	return s.resolvePaths(ctx, n, paths)
}

// resolvePaths classifies every path a project links to in one process, an
// ls-tree at a revision and an ls-files in the working tree, and reads what
// each leads to.
func (s *session) resolvePaths(ctx context.Context, n *node, targets []*target) error {
	if len(targets) == 0 {
		return nil
	}
	worktree := n.project.Where.Worktree
	var ask []string
	for _, t := range targets {
		switch {
		case t.path == ".":
		case worktree && exists(filepath.Join(n.place.top, filepath.FromSlash(t.path), ".git")):
		default:
			ask = append(ask, t.path)
		}
	}
	entries := map[string]git.Entry{}
	if len(ask) > 0 {
		var list []git.Entry
		var err error
		if worktree {
			list, err = s.run.LsFiles(ctx, n.place.repo, ask...)
		} else {
			list, err = s.run.LsTree(ctx, n.place.repo, n.key.treeish, false, ask...)
		}
		if err != nil {
			return err
		}
		for _, e := range list {
			entries[e.Path] = e
		}
	}
	modules := &modules{}
	for _, t := range targets {
		entry, listed := entries[t.path]
		var err error
		switch {
		case t.path == ".":
			err = s.resolveDirectory(ctx, n, t, "")
		case worktree && exists(filepath.Join(n.place.top, filepath.FromSlash(t.path), ".git")):
			err = s.resolveSubmodule(ctx, n, t, "", modules)
		case listed && entry.Mode == git.ModeGitlink:
			err = s.resolveSubmodule(ctx, n, t, entry.ID, modules)
		case worktree && isDir(filepath.Join(n.place.top, filepath.FromSlash(t.path))),
			!worktree && listed && entry.Mode == git.ModeTree:
			err = s.resolveDirectory(ctx, n, t, t.path)
		default:
			t.link.Problem = model.Missing
			t.link.Detail = "no submodule and no directory is at the path"
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// resolveDirectory reads a project in a directory of the same repository, at
// the same commit as the project that names it or from the same working tree.
func (s *session) resolveDirectory(ctx context.Context, n *node, t *target, dir string) error {
	t.link.Form = model.Directory
	var project *model.Project
	if where := n.project.Where; where.Worktree {
		where.Dir = dir
		k := key{gitDir: where.GitDir, top: where.Top, dir: dir}
		if project = s.seen[k]; project == nil {
			read, err := s.readDisk(ctx, n.place, where, n.depth+1)
			if err != nil {
				return err
			}
			project = read.project
		}
	} else {
		var err error
		project, err = s.project(ctx, n.place, n.key.treeish, where.Commit, dir, n.depth+1)
		if err != nil {
			return err
		}
	}
	settle(t.link, project)
	return nil
}

// modules is the paths and names .gitmodules gives a project's submodules,
// read at most once for the project.
type modules struct {
	read  bool
	names map[string]string // path to name
}

// name returns the name .gitmodules gives the submodule at a path, read at
// the same revision as the project, or from disk for the working tree.
func (m *modules) name(ctx context.Context, s *session, n *node, p string) (string, error) {
	if !m.read {
		source := []string{"--blob", n.key.treeish + ":.gitmodules"}
		if n.project.Where.Worktree {
			source = []string{"--file", filepath.Join(n.place.top, ".gitmodules")}
		}
		values, err := s.run.Config(ctx, n.place.repo, source, `^submodule\..*\.path$`)
		if err != nil {
			return "", err
		}
		m.read, m.names = true, map[string]string{}
		for k, v := range values {
			name := strings.TrimSuffix(strings.TrimPrefix(k, "submodule."), ".path")
			m.names[path.Clean(v)] = name
		}
	}
	return m.names[p], nil
}

// resolveSubmodule reads a project in a submodule at its pin. pin is the
// gitlink of the tree or the index; "" takes the pin from the HEAD of the
// checkout, which the working tree holds at the path. The objects come from
// the checkout when one exists, else from modules/<name> under the Git
// directory and then under the common Git directory.
func (s *session) resolveSubmodule(ctx context.Context, n *node, t *target, pin string, m *modules) error {
	t.link.Form, t.link.Commit = model.Submodule, pin
	var stores []string
	checkout := ""
	if n.place.top != "" {
		checkout = filepath.Join(n.place.top, filepath.FromSlash(t.path))
		if exists(filepath.Join(checkout, ".git")) {
			stores = append(stores, filepath.Join(checkout, ".git"))
		} else {
			checkout = ""
		}
	}
	if checkout == "" {
		name, err := m.name(ctx, s, n, t.path)
		if err != nil {
			return err
		}
		if name == "" || strings.Contains("/"+name+"/", "/../") {
			t.link.Problem = model.NoClone
			t.link.Detail = ".gitmodules names no submodule at the path"
			return nil
		}
		for _, base := range []string{n.place.gitDir, n.place.common} {
			store := filepath.Join(base, "modules", filepath.FromSlash(name))
			if isDir(store) && (len(stores) == 0 || stores[0] != store) {
				stores = append(stores, store)
			}
		}
	}
	if len(stores) == 0 {
		t.link.Problem = model.NoClone
		t.link.Detail = "the submodule has no clone in this repository"
		return nil
	}
	rev := pin
	if rev == "" {
		rev = "HEAD"
	}
	sub, found, err := s.open(ctx, stores[0], rev)
	if err != nil {
		return err
	}
	switch {
	case !found:
		t.link.Problem = model.NoClone
		t.link.Detail = "the submodule's clone is no Git repository"
		return nil
	case sub.commit == "":
		t.link.Problem = model.CommitAbsent
		t.link.Detail = "the submodule's clone does not hold the commit " + rev
		return nil
	}
	sub.place.top = checkout
	if pin == "" {
		t.link.Commit, t.link.Checkout = sub.commit, checkout
	}
	project, err := s.project(ctx, sub.place, sub.commit, sub.commit, "", n.depth+1)
	if err != nil {
		return err
	}
	settle(t.link, project)
	return nil
}

// resolveURL reads a project in another repository at the commit the field
// states. The clone is the one Options.Replace names, else the one Git's
// configuration names under tableaux.<url>.path, else the one in the cache.
func (s *session) resolveURL(ctx context.Context, n *node, t *target) error {
	url, commit := t.link.URL, t.stated
	t.link.Commit = commit
	clone, mapped := s.loader.options.Replace[url]
	if !mapped {
		var err error
		if clone, mapped, err = s.configured(ctx, url); err != nil {
			return err
		}
	}
	fetch := false
	if !mapped {
		var err error
		if clone, err = s.loader.clonePath(url); err != nil {
			t.link.Problem, t.link.Detail = model.NoClone, err.Error()
			return nil
		}
		fetch = s.loader.options.Fetch
	}
	if !exists(clone) {
		if !fetch {
			t.link.Problem = model.NoClone
			t.link.Detail = "no clone of the URL is at " + clone
			return nil
		}
		if err := s.loader.Fetch(ctx, url, commit); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			t.link.Problem, t.link.Detail = model.NoClone, err.Error()
			return nil
		}
		fetch = false // the clone is new, so a second fetch brings nothing
	}
	store := clone
	if dotGit := filepath.Join(clone, ".git"); exists(dotGit) {
		store = dotGit
	}
	other, found, err := s.open(ctx, store, commit)
	if err != nil {
		return err
	}
	if found && other.commit == "" && fetch {
		if err := s.loader.Fetch(ctx, url, commit); err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if other, found, err = s.open(ctx, store, commit); err != nil {
			return err
		}
	}
	switch {
	case !found:
		t.link.Problem = model.NoClone
		t.link.Detail = "no Git repository is at " + clone
		return nil
	case other.commit == "":
		t.link.Problem = model.CommitAbsent
		t.link.Detail = "the clone at " + clone + " does not hold the commit"
		return nil
	}
	project, err := s.project(ctx, other.place, other.commit, other.commit, "", n.depth+1)
	if err != nil {
		return err
	}
	settle(t.link, project)
	return nil
}

// configured returns the clone Git's configuration names for an absolute
// URL, under tableaux.<url>.path. One process reads every such key, once a
// load, in the repository of the project first asked for.
func (s *session) configured(ctx context.Context, url string) (string, bool, error) {
	if s.paths == nil {
		values, err := s.run.Config(ctx, s.home.repo, []string{"--type=path"}, `^tableaux\..*\.path$`)
		if err != nil {
			return "", false, err
		}
		s.paths = values
	}
	clone, ok := s.paths["tableaux."+url+".path"]
	return clone, ok && clone != "", nil
}

// opened is another repository and the commit a revision names in it.
type opened struct {
	place  place
	commit string // "" when the repository does not hold the revision
}

// open addresses the repository whose Git directory, or gitfile, is store and
// resolves rev in it, in one process. found is false when store is no
// repository.
func (s *session) open(ctx context.Context, store, rev string) (o opened, found bool, err error) {
	repo := git.Repo{GitDir: store}
	d, err := s.run.Discover(ctx, repo, rev)
	if errors.Is(err, git.ErrNoRepository) {
		return opened{}, false, nil
	}
	if err != nil {
		return opened{}, false, err
	}
	return opened{place: place{repo: repo, gitDir: d.GitDir, common: d.CommonDir}, commit: d.Commit}, true, nil
}

// settle gives a link the project the Loader read for it, or the problem
// that no .tableaux is there.
func settle(link *model.Link, project *model.Project) {
	if !project.Exists {
		link.Problem = model.NoProject
		link.Detail = "no .tableaux directory is there"
		return
	}
	link.Project = project
}

// exists reports whether anything is at a path.
func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// isDir reports whether a path is a directory, a symbolic link to one not included.
func isDir(p string) bool {
	info, err := os.Lstat(p)
	return err == nil && info.IsDir()
}
