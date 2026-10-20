# The declarations of design cdb0

What each package gains or changes, as Go declarations with their doc comments. A declaration not shown stands as it is on `main`. The trial of [Tests](../cdb0.md#tests) compiles every declaration below against `main` at `42e3081`, but for one departure: its `tip` takes the node and reads the whole project at the default branch, where this design reads `version.yaml` alone.

## `internal/model` (4f60)

```go
// The tableaux language versions the module accepts.
const (
	AcceptedMajor = 0
	AcceptedMinor = 4
)

// Release is a language version, as major, minor and patch.
type Release struct{ Major, Minor, Patch int }

// The releases from which a reading of the method changes. A commit whose
// version.yaml states a language before one reads as that language did.
var (
	// ModelTrailer introduced the Model: trailer: H6 exempts a commit before it.
	ModelTrailer = Release{0, 2, 1}
	// AgentIdentity introduced agents.yaml, the pairing of an email with a
	// model and the nearest person as an agent's reviewer: before it a
	// junction is an agent's only where it states a model, an agent's
	// default reviewer is the assignee, and H7 exempts the commit.
	AgentIdentity = Release{0, 4, 0}
)

// Before reports whether version.yaml states a well-formed language before
// r. A nil or malformed version precedes nothing, so the newest reading
// applies to it.
func (v *Version) Before(r Release) bool

// Before reports whether the text of a language version, as version.yaml
// writes it, names one before r; "" and a malformed text precede nothing.
func Before(text string, r Release) bool

// Project gains:
//	Agents *Agents // nil when agents.yaml is absent or does not parse

// Agents is agents.yaml: the identities that are agents.
type Agents struct {
	File   *File
	Emails []StrField // each item under agents, in file order, whatever it is
}

// Lists reports whether the file lists an email among its scalar items.
func (a *Agents) Lists(email string) bool

// Task gains, between Requires and Junctions:
//	Subproject *Subproject // the task-level field; nil when the file states none

// Waives reports whether the entry states reviewer: false, which waives review.
func (j *Junction) Waives() bool

// Holder says which field of a task file a subproject field is.
type Holder int

const (
	OnJunction    Holder = iota // junctions.<gate>.subproject
	OnRequirement               // requires[i].subproject
	OnTask                      // the task-level subproject
)

// Subproject gains:
//	On Holder

// Unpinned reports whether the field is a requirement's on an absolute URL
// that states no commit, which a tool reads at the tip of that project's trunk.
func (s *Subproject) Unpinned() bool

// Problem gains, after TooDeep, with the name "no trunk":
//	NoTrunk // an unpinned URL whose clone holds no trunk to read

// Link gains:
//	Unpinned bool   // a requirement's absolute URL with no commit: Commit is the tip of Trunk
//	Trunk    string // Unpinned: the trunk's name, or "HEAD" where version.yaml names none; else ""
```

## `internal/load` (4f60)

```go
// The files of the layout gain agentsFile, and layout names "agents.yaml".

// buildAgents types agents.yaml, with an entry for every item written.
func buildAgents(file *model.File) *model.Agents

// buildSubproject types a subproject field and records which field holds it.
func buildSubproject(node *model.Value, on model.Holder) *model.Subproject

// subprojects returns every subproject field of a task: a requirement's,
// then the task-level field, then a junction's, each in file order.
func subprojects(t *model.Task) []*model.Subproject

// tip resolves the trunk an unpinned link reads, in the clone at store: the
// name version.yaml states at the clone's default branch,
// refs/remotes/origin/HEAD else HEAD, read alone as listTree reads a file;
// else that branch itself. The name resolves refs/remotes/origin/<name>
// first, then refs/heads/<name>, as in every linked repository (27a3
// decision 5). It sets the link's Trunk and Commit, or NoTrunk.
func (s *session) tip(ctx context.Context, store string, t *target) (string, error)

// FetchTip brings url into the cache with every branch, for a requirement
// that reads the tip of its trunk. Load comes here only under Options.Fetch,
// for a URL with no mapped clone, once per load.
func (l *Loader) FetchTip(ctx context.Context, url string) error
```

`resolve` keys an unpinned field `"tip\x00" + url`, apart from any link to the URL at a commit, and skips the full-hash check for it. `reach` treats an unpinned link as a link the memo does not keep, so that the memo serves no project that reaches one.

## `internal/history` (27a3)

```go
// CoAuthors returns the identities the commit's Co-Authored-By: trailers
// name, in order: each value of the form Name <email>; it skips any other.
// It reads the key in the case SYNTAX.md writes, as the pass reads every trailer.
func (c *Commit) CoAuthors() []Person
```

## `internal/derive` (27a3)

```go
// By gains:
//	ByNearest // no entry states the reviewer of an agent, and the nearest authority who is a person takes it

// Field's doc changes: Task is, for ByNearest, the ancestor whose assignee
// reviews, and Node that ancestor's assignee.

// Junction gains:
//	Delegated bool // recursive by the task-level subproject field, not by an entry at the gate
//	Agent     bool // a plain junction whose contributor is an agent, as the version of the files reads it

// Agent reports whether an identity is an agent's as the files of the facts
// state it: listed in agents.yaml, or, without that file, the contributor of
// an entry of any task that states a model beside it. A project before 0.4.0
// reads an agent at a junction by its model alone; Junction.Agent applies
// that reading.
func (f *Facts) Agent(email string) bool

// nearest returns the nearest person among a task's assignee and its
// authorities: the assignee when it is a person, else the nearest authority
// who is one (ByNearest), else the assignee.
func (f *Facts) nearest(task *model.Task) Field

// Moot says why a Reaffirmed: trailer confirms nothing (H8).
type Moot int

const (
	NotMoot      Moot = iota
	MootParent         // the task is a parent at the commit
	MootSnapshot       // the task's next junction is recursive at the commit
	MootAbsent         // the task is a leaf with no status file at the commit
)

// Event gains:
//	Moot Moot // reaffirmed: why the task has no status of its own to confirm, as the files stand at the commit

// Reading gains:
//	CoAuthor string // the first agent a Co-Authored-By: trailer names on a person's commit; "" for none
```

`resolve(task, gate)` reads, in this order: at a gate whose index in `gates.yaml` is 2 or more, a task that states the field and no entry of its own there is recursive, `Entry` the task and `Delegated` set; otherwise the walk of today, where an entry that `Waives` supplies the reviewer `Field{By: ByTask, Task: cur.ID, Node: …}` with `V` empty, which stops inheritance as a stated reviewer does, and a `reviewer` that YAML reads as `true` supplies nothing. Then the defaults: `Agent` is `Model.V != ""` before `AgentIdentity`, and `Model.V != "" || f.agent(Contributor.V)` from it; an agent with no reviewer by an entry takes the assignee before `AgentIdentity` and `nearest` from it. The version is `f.p.Version`, the project the facts derive from: the one in view, or for `At(commit)` the one at that commit. `resolved` takes a recursive junction's field from the task-level field where `Delegated` is set. `fields`, and so `naming` and `commits`, add the task-level field after the requirements'. `Marks` draws `agent` from `Junction.Agent`; `DefaultMarks` drops the reviewer where it is `ByAssignee` or `ByNearest`; `Roles` holds `agent` beside `contributor` at a junction whose `Agent` is set. `reach` in the past reads an unpinned field at `Link.Commit`, the tip the load read.

`ownEvents` adds, for a commit with two parents or more whose `Trunk` is 0 or more, an `authorised` event for each path of `Changes` that is a task file, before it reads the trailers, so the trailer of the same commit adds none. A `reaffirmed` event takes `Moot` from the facts at its commit. `readingAt` fills `CoAuthor` where the author is no agent.

## `internal/validate` (8118)

```go
// Resolved gains:
//	Agent bool // the contributor is an agent at the junction, as the version at the files reads it

// Commit gains, and Trailers now also carries Co-Authored-By:
//	AuthorAgent bool     // the author's identity is an agent's, as the files at the commit judge it
//	CoAgents    []string // the emails of the Co-Authored-By: trailers that name an agent at the commit, in message order

// Event gains Moot, and Gate is also the junction a reaffirmed event's commit is at:
//	type Event struct{ Task, Kind, Gate, Moot string } // Moot: "parent", "recursive" or "absent" on a reaffirmed event; else ""
```

`fileKind` gains `agentsFile` and `schemaNames` gains `"agents"`. `shape.kindAt` resolves `recursive` at a task's own gate where the task states the field, the gate's index is 2 or more and the task states no entry there, so S7 to S10, S12, R6, R7 and J12 read the field as the Derivation does (8118 decision 13 d). The rules are [`rules.md`](rules.md).

## The root package (4ed9)

`resolvedOf` copies `Junction.Agent`. `methodTrailers` adds `Co-Authored-By`. `commitsOf` sets `AuthorAgent` from `then.Agent(author)`, `CoAgents` from `c.CoAuthors()` filtered by `then.Agent`, and each event's `Moot` by the words `parent`, `recursive` and `absent`; a reaffirmed event's `Gate` is its reading's. `LinkError`'s doc names `no trunk` among the problems.

## `view` and `internal/views` (493e, 886d, 8ed1)

```go
// LineDetail gains, after Committer:
//	CoAuthor string `json:"co_author"` // the first agent a Co-Authored-By: trailer names on a person's commit; else ""

// ModelCheck gains, after Reading:
//	CoAuthor string `json:"co_author"` // the first agent a Co-Authored-By: trailer names on a person's commit; else ""

// Junction.ReviewerByDefault's doc reads: the default supplies the reviewer,
// the assignee or the nearest person, since no entry states one.

// BriefSource.Kind takes "nearest" beside task, assignee and default; ID and
// Title then name the ancestor whose assignee reviews.

// Stated.Key takes "subproject", the task-level field, beside
// junctions.<gate>.<field> and assignee.

// HistoryProvenance.Commands: the git log commands of README.md#history,
// over the subtree and the range: two, and a third where the trunk is
// determined.
```

`statedField` gives `ByNearest` as `Stated{By: <ancestor>, Key: "assignee", File, Line, Col}` at that ancestor's assignee, and a waiver as the task's entry at `junctions.<gate>.reviewer` with the value `false`. `suppliers` gives a `Delegated` junction's `subproject` at the field, key `subproject`. `historyCommands` appends `git log --format='%H %as %ae %s' --first-parent --merges [<from>..]<trunk> -- <task paths>` where `Trunk().Tip` is not empty.

`envelope.schema.yaml`: `view_finding_act` gains `state_commit` and `raise_language`; `view_source_kind` gains `nearest`; `view_model_check` and the history line's detail require `co_author`, a string.

## `internal/audit` (dada)

```go
// Act gains, before None:
//	StateCommit   Act = "state_commit"   // someone states the commit an unpinned requirement lacks
//	RaiseLanguage Act = "raise_language" // the subproject raises its version.yaml to the project's language
```
