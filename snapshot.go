package tablo

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nbyoung/tablo/internal/audit"
	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/git"
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/internal/validate"
	"github.com/nbyoung/tablo/internal/views"
)

// This file is the one place that names the functions of the Loader, the
// Validator, the Derivation, the Audit and the views (design 4ed9, A1 to A8).

// Options are what a caller states about a repository beyond its files.
type Options struct {
	Trunk   string            // the branch the caller names; read when version.yaml and origin/HEAD name none
	Replace map[string]string // an absolute subproject URL to the directory of its local clone
}

// A Repository is a long-lived handle on one project directory, safe for
// concurrent use. Resolve returns the full id of the commit a ref names.
// Load reads the project and its history once: the ref "" reads the files
// from the working tree and the history from HEAD; a commit-ish reads both
// from that commit; a tree reads the files alone, for Validate. A project
// that fails validation still loads, and the snapshot carries the diagnostics.
type Repository struct {
	dir    string
	opt    Options
	run    git.Runner
	loader *load.Loader
	reader *history.Reader
}

// Open finds the Git repository that holds dir and returns a handle on the
// project whose .tableaux lies in dir, or nearest above it. It reads no
// project file. The error is a *ReadError.
func Open(ctx context.Context, dir string, opt Options) (*Repository, error) {
	if dir == "" {
		dir = "."
	}
	r := &Repository{dir: dir, opt: Options{Trunk: opt.Trunk, Replace: map[string]string{}}}
	for url, clone := range opt.Replace {
		r.opt.Replace[url] = clone
	}
	if _, err := r.run.Discover(ctx, git.Repo{Dir: dir}, "HEAD"); err != nil {
		return nil, &ReadError{Op: "open the repository at " + dir, Err: err}
	}
	r.loader = load.New(load.Options{Replace: r.opt.Replace})
	r.reader = history.NewReader(history.Options{})
	return r, nil
}

// Resolve returns the full id of the commit ref names. The error is a
// *NotFoundError for a ref that names no commit, or a *ReadError.
func (r *Repository) Resolve(ctx context.Context, ref string) (string, error) {
	found, err := r.run.Discover(ctx, git.Repo{Dir: r.dir}, ref)
	if err != nil {
		return "", &ReadError{Op: "resolve " + ref, Err: err}
	}
	if found.Commit == "" {
		return "", &NotFoundError{Kind: "ref", Name: ref}
	}
	return found.Commit, nil
}

// Load reads the project at ref and its history, validates it and derives
// its facts. The error is a *NotFoundError for a ref that names no commit or
// tree, or a *ReadError.
func (r *Repository) Load(ctx context.Context, ref string) (*Snapshot, error) {
	p, err := r.loader.Load(ctx, load.Source{Dir: r.dir, Ref: ref})
	if err != nil {
		if errors.Is(err, load.ErrNoRef) {
			return nil, &NotFoundError{Kind: "ref", Name: ref}
		}
		return nil, &ReadError{Op: "read the project", Err: err}
	}
	var set *history.Set
	if p.Where.Commit != "" {
		if set, err = r.reader.Read(ctx, p, r.opt.Trunk); err != nil {
			return nil, &ReadError{Op: "read the history", Err: err}
		}
	}
	s := newSnapshot(r, p, derive.New(p, set), set)
	if s.source, err = r.source(ctx, p, s.facts, ref); err != nil {
		return nil, err
	}
	s.info = projectOf(p, s.facts)
	return s, nil
}

// newSnapshot validates a project of a load beside its facts.
func newSnapshot(r *Repository, p *model.Project, f *derive.Facts, set *history.Set) *Snapshot {
	s := &Snapshot{repo: r, project: p, facts: f, logs: set,
		ranges: map[string]*derive.Facts{}, linked: map[*model.Link]*Snapshot{}}
	s.diagnostics = validate.Files(p)
	s.diagnostics = append(s.diagnostics, validate.Derived(p, validateFacts(s.facts))...)
	validate.Sort(s.diagnostics)
	s.public = make([]Diagnostic, len(s.diagnostics))
	for i, d := range s.diagnostics {
		s.public[i] = diagnosticOf(d)
	}
	return s
}

// source states what a load read.
func (r *Repository) source(ctx context.Context, p *model.Project, f *derive.Facts, ref string) (Source, error) {
	src := Source{Ref: ref, Dir: p.Where.Dir, Worktree: p.Where.Worktree, OnTrunk: f.OnTrunk()}
	if src.Dir == "" {
		src.Dir = "."
	}
	if p.Where.Worktree {
		out, err := r.run.Run(ctx, git.Repo{Dir: r.dir}, nil, "symbolic-ref", "--quiet", "--short", "HEAD")
		var exit *git.ExitError
		switch {
		case err == nil:
			src.Ref = strings.TrimSpace(string(out))
		case errors.As(err, &exit):
			src.Ref = "HEAD" // detached
		default:
			return Source{}, &ReadError{Op: "read HEAD", Err: err}
		}
	}
	fill(&src, p, f)
	return src, nil
}

// fill states the commit a load read, its date and the trunk.
func fill(src *Source, p *model.Project, f *derive.Facts) {
	if commit := p.Where.Commit; commit != "" {
		src.Commit = &commit
		if log := f.Log(); log != nil && log.Commit(commit) != nil {
			date := log.Commit(commit).Date()
			src.Date = &date
		}
	}
	trunk := f.Trunk()
	src.Trunk.Resolved = trunk.How()
	if src.Trunk.Resolved != "undetermined" {
		name := trunk.Name
		src.Trunk.Name = &name
	}
}

// projectOf names the project, or returns nil when the files give no root
// task. A refused derivation knows no tree, so the root is then the one task
// whose file names no parent.
func projectOf(p *model.Project, f *derive.Facts) *ProjectInfo {
	root := f.Root()
	if root == "" {
		for _, id := range p.TaskIDs() {
			if p.Tasks[id].Parent == nil && idPattern.MatchString(id) {
				if root != "" {
					return nil
				}
				root = id
			}
		}
	}
	task := p.Tasks[root]
	if task == nil {
		return nil
	}
	info := &ProjectInfo{Root: root, Title: task.Title.V, Owner: task.Assignee.V}
	if p.Version != nil {
		info.Tableaux = p.Version.Tableaux.V
	}
	return info
}

// diagnosticOf spells a diagnostic of the Loader or the Validator as the
// envelope does: the path below the project's directory, from .tableaux.
func diagnosticOf(d model.Diagnostic) Diagnostic {
	out := Diagnostic{
		Severity: d.Severity.String(), Code: d.Code, Line: d.Pos.Line, Col: d.Pos.Col,
		Task: d.Task, Gate: d.Gate, Commit: d.Commit, Trailer: d.Trailer, Message: d.Message,
	}
	if d.Pos.File != "" {
		out.Path = ".tableaux/" + d.Pos.File
	}
	return out
}

// A Snapshot is one project at one source. It is immutable and safe for
// concurrent use, and it computes each derived fact once.
type Snapshot struct {
	repo        *Repository
	home        *Snapshot // the snapshot Load returns, for one Linked returns; else nil
	project     *model.Project
	facts       *derive.Facts
	logs        *history.Set       // the passes of the load; nil without history
	diagnostics []model.Diagnostic // sorted
	public      []Diagnostic       // the same, as the envelope spells them
	source      Source
	info        *ProjectInfo

	mu     sync.Mutex
	ranges map[string]*derive.Facts  // by the commit a range starts after, for one the pass lacks
	linked map[*model.Link]*Snapshot // the snapshots Linked returns, by link
}

// Source returns what Load read.
func (s *Snapshot) Source() Source {
	src := s.source
	return src
}

// AuthorTime returns the author time of the commit the load read, with the
// zone Git records, as git log --format=%aI prints it:
// 2026-09-01T21:30:00-07:00. Its day is Source.Date. It is "" where Date is
// nil: a tree, an unborn branch, or a commit the pass lacks.
func (s *Snapshot) AuthorTime() string {
	commit := s.project.Where.Commit
	log := s.facts.Log()
	if commit == "" || log == nil || log.Commit(commit) == nil {
		return ""
	}
	c := log.Commit(commit)
	return time.Unix(c.Time, 0).In(time.FixedZone("", c.Zone*60)).Format("2006-01-02T15:04:05-07:00")
}

// Project returns the project's name and owner, or nil when the files give no
// root task.
func (s *Snapshot) Project() *ProjectInfo {
	if s.info == nil {
		return nil
	}
	info := *s.info
	return &info
}

// Diagnostics returns what the Loader and the Validator report, sorted and
// never nil.
func (s *Snapshot) Diagnostics() []Diagnostic {
	return append([]Diagnostic{}, s.public...)
}

// HasTask reports whether the project holds the task.
func (s *Snapshot) HasTask(id string) bool {
	return idPattern.MatchString(id) && s.project.Tasks[id] != nil
}

// GateKeys returns the gate keys of gates.yaml, in order.
func (s *Snapshot) GateKeys() []string {
	if s.project.Gating == nil {
		return nil
	}
	var keys []string
	for _, g := range s.project.Gating.Gates {
		if g.Key.Node.Scalar() {
			keys = append(keys, g.Key.V)
		}
	}
	return keys
}

// Viewer returns the roles the email holds anywhere in the project, in
// README.md's order; an observer when it holds none or is "".
func (s *Snapshot) Viewer(email string) Viewer {
	v := Viewer{Roles: []Role{RoleObserver}}
	if email == "" {
		return v
	}
	v.Email = &email
	if roles := s.facts.Roles(email); len(roles) > 0 {
		v.Roles = make([]Role, len(roles))
		for i, r := range roles {
			v.Roles[i] = Role(r)
		}
	}
	return v
}

// Checkouts returns the directories of the submodule checkouts whose HEAD
// gave a pin of this load, absolute and sorted: what a watcher polls beside
// the working tree, since a moved checkout changes what the project reads
// (4f60 decision 9). It is empty for a load at a ref.
func (s *Snapshot) Checkouts() []string {
	seen := map[*model.Project]bool{s.project: true}
	queue := []*model.Project{s.project}
	dirs := map[string]bool{}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, link := range p.Links {
			if link.Checkout != "" {
				dirs[link.Checkout] = true
			}
			if link.Project != nil && !seen[link.Project] {
				seen[link.Project] = true
				queue = append(queue, link.Project)
			}
		}
	}
	list := []string{}
	for dir := range dirs {
		list = append(list, dir)
	}
	sort.Strings(list)
	return list
}

// envelope returns the parts of an envelope every command over a snapshot
// shares.
func (s *Snapshot) envelope(command string) *Envelope {
	src := s.Source()
	return &Envelope{
		Schema:      SchemaVersion,
		Command:     command,
		Source:      &src,
		Project:     s.Project(),
		Diagnostics: s.Diagnostics(),
	}
}

// validateData is the data of the validate command.
type validateData struct {
	Valid       bool `json:"valid"`
	Errors      int  `json:"errors"`
	Warnings    int  `json:"warnings"`
	Information int  `json:"information"`
}

// Validate returns the validate command's envelope.
func (s *Snapshot) Validate() *Envelope {
	e := s.envelope("validate")
	var d validateData
	for _, x := range s.diagnostics {
		switch x.Severity {
		case model.Error:
			d.Errors++
		case model.Warning:
			d.Warnings++
		default:
			d.Information++
		}
	}
	d.Valid = d.Errors == 0
	e.Data = d
	return e
}

// CheckTrailer returns a *UsageError when t is malformed and a
// *NotFoundError when t names a task the project lacks or, for a reviewed
// trailer, a gate that gates.yaml lacks or that does not apply to the task.
func (s *Snapshot) CheckTrailer(t Trailer) error {
	if _, err := t.Line(); err != nil {
		return err
	}
	if t.Kind == "model" {
		return nil
	}
	if !s.HasTask(t.Task) {
		return &NotFoundError{Kind: "task", Name: t.Task}
	}
	if t.Kind != "reviewed" {
		return nil
	}
	if !contains(s.GateKeys(), t.Gate) {
		return &NotFoundError{Kind: "gate", Name: t.Gate}
	}
	if s.facts.Refused() != nil {
		return nil // no tree to apply the gate to: the syntax and gates.yaml decide
	}
	if applies := s.facts.Applicable(t.Task); len(applies) == 0 || applies[0] == t.Gate || !contains(applies, t.Gate) {
		return &NotFoundError{Kind: "gate", Name: t.Gate} // undefined takes no review
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// View checks and resolves p for v, derives the view and returns its
// envelope. The error is a *UsageError, a *NotFoundError or a *ReadError;
// an invalid project is no error: the envelope then has nil Data.
func (s *Snapshot) View(ctx context.Context, v View, p Params) (*Envelope, error) {
	if err := p.Check(v); err != nil {
		return nil, err
	}
	if s.source.Commit == nil && !s.source.Worktree {
		return nil, &ReadError{Op: "the view " + string(v) + " reads a commit, and " + s.source.Ref + " is a tree"}
	}
	r, err := s.resolve(v, p)
	if err != nil {
		return nil, err
	}
	e := s.envelope(string(v))
	viewer := s.Viewer(r.Viewer)
	e.Viewer = &viewer
	e.Params = Resolved{View: v, Params: r}
	if r.From != "" {
		commit, err := s.Resolve(ctx, r.From)
		if err != nil {
			return nil, err
		}
		e.Source.From = &SourceFrom{Ref: r.From, Commit: commit}
		r.From = commit
	}
	data, err := derivers[v](ctx, s, r)
	switch {
	case errors.Is(err, errNotDerivable):
		data = nil
	case err != nil:
		return nil, err
	}
	e.Data = data
	return e, nil
}

// resolve writes in the defaults of the command table and checks that the
// tasks and gates p names are the project's.
func (s *Snapshot) resolve(v View, p Params) (Params, error) {
	root := ""
	if s.info != nil {
		root = s.info.Root
	}
	switch v {
	case Authority, Assignment, Queue, Blockage, History, Audit:
		if p.Task == "" {
			p.Task = root
		}
	}
	switch {
	case v == Queue && p.Person == "":
		p.Person = p.Viewer
		if p.Person == "" {
			return p, usagef("queue needs --person or a viewer")
		}
	case v == Context && p.Task == "" && p.Person == "":
		p.Person = p.Viewer
		if p.Person == "" {
			return p, usagef("context needs --task, --person or a viewer")
		}
	}
	if (v == Tableau || v == Context) && p.Window == nil && len(p.Columns) == 0 {
		one := 1
		p.Window = &one
	}
	if v == Audit {
		if p.Stale == 0 {
			p.Stale = audit.DefaultStale
		}
		if p.Minimum == "" {
			p.Minimum = "warning"
		}
	}
	p.Level = s.level(v, p)
	if p.Task != "" && p.Task != root && !s.HasTask(p.Task) {
		return p, &NotFoundError{Kind: "task", Name: p.Task}
	}
	gates := s.GateKeys()
	for _, c := range p.Columns {
		if !contains(gates, c) {
			return p, &NotFoundError{Kind: "gate", Name: c}
		}
	}
	if b := p.Brief; b != nil {
		if !s.HasTask(b.Task) {
			return p, &NotFoundError{Kind: "task", Name: b.Task}
		}
		if !contains(gates, b.Gate) {
			return p, &NotFoundError{Kind: "gate", Name: b.Gate}
		}
	}
	return p, nil
}

// opensAt says, per view, the roles that open it at detail and at
// provenance; nil for detail is every role. Every other role opens it at
// glance. It is the last column of design 4ed9's command table.
var opensAt = map[View]struct{ detail, provenance []Role }{
	Gates:      {nil, nil},
	Task:       {nil, nil},
	Authority:  {[]Role{RoleOwner, RoleAuthority, RoleAssignee}, nil},
	Assignment: {[]Role{RoleOwner, RoleAuthority, RoleAssignee}, nil},
	Queue:      {[]Role{RoleContributor, RoleAgent, RoleReviewer, RoleAuthority}, nil},
	Blockage:   {[]Role{RoleOwner, RoleAuthority, RoleContributor}, nil},
	Tableau:    {[]Role{RoleOwner}, nil},
	Context:    {[]Role{RoleAssignee, RoleContributor, RoleAuthority}, nil},
	History:    {nil, []Role{RoleOwner}},
	Audit:      {[]Role{RoleOwner, RoleAuthority}, []Role{RoleOwner}},
}

// level resolves the level: --brief gives provenance; else --level; else the
// level --role opens the view at; else the deepest level the viewer's roles
// open it at.
func (s *Snapshot) level(v View, p Params) Level {
	switch {
	case p.Brief != nil:
		return LevelProvenance
	case p.Level != "":
		return p.Level
	}
	roles := []Role{p.Role}
	if p.Role == "" {
		roles = s.Viewer(p.Viewer).Roles
	}
	open := opensAt[v]
	level := LevelGlance
	for _, role := range roles {
		for _, r := range open.provenance {
			if r == role {
				return LevelProvenance
			}
		}
		if open.detail == nil {
			level = LevelDetail
		}
		for _, r := range open.detail {
			if r == role {
				level = LevelDetail
			}
		}
	}
	return level
}

// A deriver computes one view's data from a snapshot and checked, resolved
// parameters. It returns errNotDerivable when the project's errors prevent
// the derivation; View then returns the envelope with nil Data.
type deriver func(ctx context.Context, s *Snapshot, p Params) (any, error)

// errNotDerivable says the Derivation refuses the project (27a3 decision 6).
var errNotDerivable = errors.New("tablo: the project's errors prevent the derivation")

// refused reports whether the Derivation refuses the project.
func (s *Snapshot) refused() bool { return s.facts.Refused() != nil }

// derivers names each view task's function once; nothing else calls them.
// Each entry is the one its view task's model.md gives, behind the refusal
// that the deriver's contract asks.
var derivers = map[View]deriver{
	Gates: func(_ context.Context, s *Snapshot, p Params) (any, error) {
		if s.refused() {
			return nil, errNotDerivable
		}
		return views.Gates(s.facts, query(p)), nil
	},
	Task: func(_ context.Context, s *Snapshot, p Params) (any, error) {
		if s.refused() {
			return nil, errNotDerivable
		}
		return views.Task(s.facts, query(p)), nil
	},
	Authority: func(_ context.Context, s *Snapshot, p Params) (any, error) {
		if s.refused() {
			return nil, errNotDerivable
		}
		return views.Authority(s.facts, query(p)), nil
	},
	Assignment: func(_ context.Context, s *Snapshot, p Params) (any, error) {
		if s.refused() {
			return nil, errNotDerivable
		}
		return views.Assignment(s.facts, query(p)), nil
	},
	Tableau: func(_ context.Context, s *Snapshot, p Params) (any, error) {
		if s.refused() {
			return nil, errNotDerivable
		}
		return views.Tableau(s.facts, query(p)), nil
	},
	Context: func(_ context.Context, s *Snapshot, p Params) (any, error) {
		if s.refused() {
			return nil, errNotDerivable
		}
		return views.Context(s.facts, query(p)), nil
	},
	Queue: func(_ context.Context, s *Snapshot, p Params) (any, error) {
		if s.refused() {
			return nil, errNotDerivable
		}
		return views.Queue(s.facts, query(p)), nil
	},
	Blockage: func(_ context.Context, s *Snapshot, p Params) (any, error) {
		if s.refused() {
			return nil, errNotDerivable
		}
		return views.Blockage(s.facts, query(p)), nil
	},
	History: func(ctx context.Context, s *Snapshot, p Params) (any, error) {
		if s.refused() {
			return nil, errNotDerivable
		}
		f, err := s.ranged(ctx, p.From)
		if err != nil {
			return nil, err
		}
		return views.History(f, query(p)), nil
	},
	Audit: func(_ context.Context, s *Snapshot, p Params) (any, error) {
		r, err := audit.Run(audit.Input{Facts: s.facts, Diagnostics: s.diagnostics, Now: p.Now, Stale: p.Stale})
		if err != nil {
			return nil, usage(err) // a *DateError as a *UsageError (dada A1)
		}
		if r == nil {
			return nil, errNotDerivable // the Derivation refuses: data is null
		}
		return views.Audit(s.facts, r, query(p)), nil
	},
}

// usage returns an error of the Audit's input as a *UsageError.
func usage(err error) error {
	var date *audit.DateError
	if errors.As(err, &date) {
		if date.Now == "" {
			return usagef("audit needs --now, the date staleness counts from")
		}
		return usagef("--now %q is not a date of the form YYYY-MM-DD", date.Now)
	}
	return &UsageError{Msg: err.Error()}
}

// ranged returns the facts whose pass holds the commit a range starts
// after: the snapshot's own, unless its pass lacks the commit, which a start
// that is no ancestor of the source or of the trunk leaves out. Then the
// facts derive once more from the same files and a pass that also walks
// from that commit, and the snapshot keeps them.
func (s *Snapshot) ranged(ctx context.Context, from string) (*derive.Facts, error) {
	log := s.facts.Log()
	if from == "" || log == nil || log.Commit(from) != nil {
		return s.facts, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.ranges[from]; ok {
		return f, nil
	}
	var set *history.Set
	var err error
	if s.home == nil {
		set, err = s.repo.reader.Read(ctx, s.project, s.repo.opt.Trunk, from)
	} else {
		set, err = s.repo.reader.ReadLinked(ctx, s.home.project, s.project, from)
	}
	if err != nil {
		return nil, &ReadError{Op: "read the history from " + from, Err: err}
	}
	f := derive.New(s.project, set)
	s.ranges[from] = f
	return f, nil
}

// query maps checked, resolved parameters to what a view takes.
func query(p Params) views.Query {
	q := views.Query{
		Task: p.Task, Person: p.Person, Proposed: p.Proposed,
		Window: 1, Columns: p.Columns, Historical: p.Historical,
		From: p.From, Minimum: model.Warning,
	}
	switch p.Level {
	case LevelDetail:
		q.Level = views.Detail
	case LevelProvenance:
		q.Level = views.Provenance
	}
	if p.Window != nil {
		q.Window = *p.Window
	}
	if p.Brief != nil {
		q.Brief = &views.BriefOf{Task: p.Brief.Task, Gate: p.Brief.Gate}
	}
	switch p.Minimum {
	case "error":
		q.Minimum = model.Error
	case "information":
		q.Minimum = model.Information
	}
	return q
}

// validateFacts fills the facts the Validator's rules beyond the files read
// from the Derivation (8118 decisions 4, 13 and 15; 27a3 decision 10).
func validateFacts(f *derive.Facts) *validate.Facts {
	vf := &validate.Facts{Junctions: map[validate.Junction]validate.Resolved{}}
	for _, id := range f.Order() {
		for i, c := range f.Requires(id) {
			if c.Why != derive.Determined {
				continue
			}
			vf.Requirements = append(vf.Requirements, validate.Condition{Task: id, Index: i, Origin: c.Origin,
				From: c.From, To: c.To, Stands: c.Stands, Met: c.Met, Due: c.Due, MetAtTip: c.AtTrunk == derive.Yes})
		}
		for _, j := range f.Junctions(id) {
			if r, ok := resolvedOf(j); ok {
				vf.Junctions[validate.Junction{Task: id, Gate: j.Gate}] = r
			}
		}
	}
	log := f.Log()
	if log == nil {
		return vf
	}
	h := &validate.History{OffTrunk: offTrunk(f), Reviews: reviews(f)}
	if t := f.Trunk(); t.How() != "undetermined" {
		h.Trunk = t.Name
	}
	h.Commits = commitsOf(f, log)
	vf.History = h
	return vf
}

// resolvedOf returns a junction as the Validator reads it, and false for a
// junction that does not apply.
func resolvedOf(j *derive.Junction) (validate.Resolved, bool) {
	switch j.Kind {
	case model.Plain:
		return validate.Resolved{Contributor: j.Contributor.V, Model: j.Model.V, Reviewer: j.Reviewer.V, Agent: j.Agent}, true
	case model.Recursive:
		return validate.Resolved{Recursive: true}, true
	}
	return validate.Resolved{}, false
}

// reviews returns the accepting commit of each reviewed junction a leaf's
// status passes, and of each leaf's next junction: "" where nothing accepts
// yet. History.Reviews needs a key for every one, or S11 rejects the status.
func reviews(f *derive.Facts) map[validate.Junction]string {
	out := map[validate.Junction]string{}
	put := func(r *derive.Review) {
		if r == nil {
			return
		}
		commit := ""
		if r.Commit != nil {
			commit = r.Commit.ID
		}
		out[validate.Junction{Task: r.Task, Gate: r.Gate}] = commit
	}
	statuses := f.Project().Statuses
	for _, id := range f.Order() {
		if !f.Leaf(id) {
			continue
		}
		for _, r := range f.Reviews(id) {
			put(r)
		}
		gate := "undefined"
		if s := statuses[id]; s != nil {
			gate = s.Gate.V
		}
		if next := f.Next(id, gate); next != "" {
			put(f.Accepted(id, next))
		}
	}
	return out
}

// offTrunk returns each linkage whose commit its subproject's trunk does not
// hold on its first-parent line: a recursive junction's, at each gate that
// names it; the task-level field's, once; and a cross-project requirement's.
func offTrunk(f *derive.Facts) []validate.Pin {
	var pins []validate.Pin
	off := func(l *model.Link) (string, bool) {
		pin := f.Pin(l)
		if pin == nil || pin.OnTrunk != derive.No {
			return "", false
		}
		return f.Sub(l).Trunk().Name, true
	}
	for _, id := range f.Order() {
		for _, s := range f.Snapshots(id) {
			trunk, ok := off(s.Link)
			if !ok {
				continue
			}
			field := false // the task-level field names the link, once and with no gate
			for _, gate := range s.Gates {
				if j := f.Junction(id, gate); j != nil && j.Delegated {
					field = true
					continue
				}
				pins = append(pins, validate.Pin{Task: id, Gate: gate, Index: -1, Trunk: trunk})
			}
			if field {
				pins = append(pins, validate.Pin{Task: id, Index: -1, Trunk: trunk})
			}
		}
		for i, c := range f.Requires(id) {
			if c.Link == nil {
				continue
			}
			if trunk, ok := off(c.Link); ok {
				pins = append(pins, validate.Pin{Task: id, Index: i, Trunk: trunk})
			}
		}
	}
	return pins
}

// The keys of the method's trailers, which History.Commits carries.
var methodTrailers = map[string]bool{"Authorised": true, "Reviewed": true, "Reaffirmed": true, "Model": true, "Co-Authored-By": true}

// commitsOf returns every commit the source reaches that carries a method
// trailer, gives an event of the project's own or is at a junction, newest
// first, each with the junctions it is at and those its Reviewed: trailers
// name, and whether its author and its co-authors are agents, as the files
// stand at that commit.
func commitsOf(f *derive.Facts, log *history.Log) []validate.Commit {
	events := map[string][]*derive.Event{}
	for _, e := range f.History() {
		if e.Sub == "" {
			events[e.Commit.ID] = append(events[e.Commit.ID], e)
		}
	}
	var out []validate.Commit
	for _, c := range log.Commits {
		if !c.InSource {
			continue
		}
		vc := validate.Commit{Hash: c.ID, Author: c.Author.Email, Committer: c.Committer.Email}
		for _, t := range c.Trailers {
			if methodTrailers[t.Key] {
				vc.Trailers = append(vc.Trailers, validate.Trailer{Key: t.Key, Value: t.Value})
			}
		}
		seen := map[validate.Junction]bool{}
		for _, e := range events[c.ID] {
			vc.Events = append(vc.Events, validate.Event{Task: e.Task, Kind: string(e.Kind), Gate: e.Gate})
			// A commit is at the junction of the gate it records when it
			// changes a status's gate; any other commit to a status, and a
			// Reaffirmed: commit, is at the task's next applicable junction
			// (README.md, "Junctions"). The Derivation's reading names it.
			if e.Reading != nil && e.Reading.Gate != "" {
				j := validate.Junction{Task: e.Task, Gate: e.Reading.Gate}
				if !seen[j] {
					seen[j] = true
					vc.At = append(vc.At, j)
				}
			}
		}
		if len(vc.Trailers) == 0 && len(vc.Events) == 0 {
			continue
		}
		named := append([]validate.Junction(nil), vc.At...)
		for _, t := range vc.Trailers {
			if words := strings.Fields(t.Value); t.Key == "Reviewed" && len(words) == 2 {
				named = append(named, validate.Junction{Task: words[0], Gate: words[1]})
			}
		}
		then := f.At(c.ID)
		if p := log.Project(c.ID); p != nil && p.Version != nil {
			vc.Tableaux = p.Version.Tableaux.V
		}
		vc.AuthorAgent = then.Agent(c.Author.Email)
		for _, p := range c.CoAuthors() {
			if then.Agent(p.Email) {
				vc.CoAgents = append(vc.CoAgents, p.Email)
			}
		}
		for _, j := range named {
			if jj := then.Junction(j.Task, j.Gate); jj != nil {
				if r, ok := resolvedOf(jj); ok {
					if vc.Junctions == nil {
						vc.Junctions = map[validate.Junction]validate.Resolved{}
					}
					vc.Junctions[j] = r
				}
			}
		}
		out = append(out, vc)
	}
	return out
}
