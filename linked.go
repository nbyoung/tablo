package tablo

import (
	"context"
	"fmt"
	"path"
	"regexp"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/git"
	"github.com/nbyoung/tablo/internal/model"
)

// absoluteURL matches a url with a scheme and ://, as the Loader tells an
// absolute URL from a path.
var absoluteURL = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// A LinkError reports a subproject field whose link leads to no project
// (4ed9 decision 18 b): the url names a link of the load, and the Loader
// found no project there. Problem is the reason's name, as RULES.md's J8
// and J9 read it: "outside", "missing", "no clone", "bad commit", "commit
// absent", "no project" or "too deep". Detail says more.
type LinkError struct {
	URL, Commit     string // the link: an absolute URL as written or the path cleaned, and its commit
	Problem, Detail string
}

// Error names the link and why it leads to no project.
func (e *LinkError) Error() string {
	msg := fmt.Sprintf("the subproject %q leads to no project: %s", e.URL, e.Problem)
	if e.Detail != "" {
		msg += ", " + e.Detail
	}
	return msg
}

// Linked returns the snapshot of the project a subproject field links to,
// as this load read it (4ed9 decision 18 b): at the commit the link fixes,
// from a submodule's checkout or store, from a directory of the same
// repository, or from the clone that Options.Replace or Git's configuration
// names for an absolute URL. It runs no second walk of the repository: the
// project, its passes and its facts are the ones Load already holds. The
// snapshot has its own source, project, diagnostics and views; its Checkouts
// are those of the links it holds in turn.
//
// url is the field's url: an absolute URL as written, or a path, which
// Linked cleans as the Loader does, so that "./firmware" and "firmware" name
// one link. commit is "" unless the files link one absolute URL at more than
// one commit; then it names the one wanted. The error is a *NotFoundError of
// kind "subproject" for a url that names no link of the load, a *LinkError
// for a link that leads to no project, and a *UsageError for a url linked at
// several commits with none named.
func (s *Snapshot) Linked(url, commit string) (*Snapshot, error) {
	key := url
	if !absoluteURL.MatchString(url) {
		key = path.Clean(url)
	}
	var found []*model.Link
	for _, l := range s.project.Links {
		if (l.URL == key || l.URL == url) && (commit == "" || l.Commit == commit) {
			found = append(found, l)
		}
	}
	switch {
	case len(found) == 0:
		return nil, &NotFoundError{Kind: "subproject", Name: url}
	case len(found) > 1:
		return nil, usagef("the subproject %q is linked at %d commits; name the commit", url, len(found))
	}
	l := found[0]
	if l.Problem != model.Resolved || l.Project == nil {
		return nil, &LinkError{URL: l.URL, Commit: l.Commit, Problem: l.Problem.String(), Detail: l.Detail}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, ok := s.linked[l]; ok {
		return sub, nil
	}
	f := s.facts.Sub(l)
	if f == nil {
		// The parent's derivation refuses, and its facts lead nowhere; the
		// link's project derives from the same passes.
		f = derive.New(l.Project, s.logs)
	}
	sub := newSnapshot(s.repo, l.Project, f, s.logs)
	sub.home = s
	if s.home != nil {
		sub.home = s.home
	}
	p := l.Project
	sub.source = Source{Ref: l.Commit, Dir: p.Where.Dir, Worktree: p.Where.Worktree, OnTrunk: f.OnTrunk()}
	if l.Form == model.Directory {
		// A directory of the same repository reads at the parent's source.
		sub.source.Ref = s.source.Ref
	}
	if sub.source.Dir == "" {
		sub.source.Dir = "."
	}
	fill(&sub.source, p, f)
	sub.info = projectOf(p, f)
	s.linked[l] = sub
	return sub, nil
}

// resolveRef returns the full id of the commit a ref names in the
// snapshot's repository: the one Open names, or a linked project's own.
func (s *Snapshot) resolveRef(ctx context.Context, ref string) (string, error) {
	if s.home == nil {
		return s.repo.Resolve(ctx, ref)
	}
	found, err := s.repo.run.Discover(ctx, git.Repo{GitDir: s.project.Where.GitDir}, ref)
	if err != nil {
		return "", &ReadError{Op: "resolve " + ref, Err: err}
	}
	if found.Commit == "" {
		return "", &NotFoundError{Kind: "ref", Name: ref}
	}
	return found.Commit, nil
}
