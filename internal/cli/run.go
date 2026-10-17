package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nbyoung/tablo"
	"github.com/nbyoung/tablo/internal/git"
)

// getenv reads the environment, for the viewer's default; a test replaces it.
var getenv = os.Getenv

// today returns the host's local date, YYYY-MM-DD, for the default of
// --now; a test replaces it.
var today = func() string { return time.Now().Format("2006-01-02") }

// Run executes one tablo command line and returns its exit code. It writes
// the result to stdout and, on exit 2 or 3, the message to stderr. It reads
// the environment for the viewer and the clock for --now's default, through
// two package variables a test replaces, and nothing else outside -C.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	inv, err := parse(args)
	if err != nil {
		return fail(stderr, err)
	}
	if inv.help {
		if _, err := io.WriteString(stdout, usage); err != nil {
			return fail(stderr, &tablo.ReadError{Op: "write the usage", Err: err})
		}
		return 0
	}
	e, plain, err := inv.run(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	var out []byte
	switch {
	case inv.format == "yaml":
		out = e.YAML()
	case inv.format == "json" || plain == nil:
		out = e.JSON()
	default:
		out = plain
	}
	if _, err := stdout.Write(out); err != nil {
		return fail(stderr, &tablo.ReadError{Op: "write the result", Err: err})
	}
	return e.ExitCode()
}

// fail writes an error's message and returns its exit code: 2 for a usage
// error, 3 for anything else. A message that cannot be written changes no
// exit code.
func fail(stderr io.Writer, err error) int {
	var u *tablo.UsageError
	if errors.As(err, &u) {
		_, _ = fmt.Fprintf(stderr, "tablo: %s\nRun 'tablo --help' for usage.\n", u.Msg)
		return 2
	}
	_, _ = fmt.Fprintf(stderr, "tablo: %s\n", err)
	return 3
}

// run executes a parsed command line. It returns the envelope and, for a
// command with a plain form, that form; plain is nil for a view.
func (inv *invocation) run(ctx context.Context) (e *tablo.Envelope, plain []byte, err error) {
	switch {
	case inv.command == "version":
		e = versionEnvelope()
		return e, []byte(fmt.Sprintf("tablo %s (tableaux %d.%d, schema %s)\n", tablo.Version, tablo.AcceptedMajor, tablo.AcceptedMinor, tablo.SchemaVersion)), nil
	case inv.command == "trailer" && (inv.noCheck || inv.trailer.Kind == "model"):
		e, err = trailerEnvelope(inv.trailer, nil)
		if err != nil {
			return nil, nil, err
		}
		return e, []byte(trailerLine(e) + "\n"), nil
	}
	repo, err := tablo.Open(ctx, inv.dir, tablo.Options{Trunk: inv.trunk, Replace: inv.replaceMap()})
	if err != nil {
		return nil, nil, err
	}
	s, err := repo.Load(ctx, inv.ref)
	if err != nil {
		return nil, nil, err
	}
	switch inv.command {
	case "validate":
		e = s.Validate()
		return e, plainDiagnostics(e.Diagnostics), nil
	case "trailer":
		if src := s.Source(); src.Commit == nil && !src.Worktree {
			return nil, nil, &tablo.ReadError{Op: "trailer reads a commit, and " + src.Ref + " is a tree"}
		}
		if err := s.CheckTrailer(inv.trailer); err != nil {
			return nil, nil, err
		}
		if e, err = trailerEnvelope(inv.trailer, s); err != nil {
			return nil, nil, err
		}
		return e, []byte(trailerLine(e) + "\n"), nil
	}
	p := inv.params
	p.Viewer = inv.viewer
	if p.Viewer == "" {
		p.Viewer = defaultViewer(ctx, inv.dir)
	}
	if inv.view == tablo.Audit && p.Now == "" {
		p.Now = today()
	}
	e, err = s.View(ctx, inv.view, p)
	return e, nil, err
}

// replaceMap returns the --replace options as the library takes them.
func (inv *invocation) replaceMap() map[string]string {
	m := map[string]string{}
	for _, r := range inv.replace {
		m[r[0]] = r[1]
	}
	return m
}

// defaultViewer returns the viewer when --viewer names none: GIT_AUTHOR_EMAIL
// when it is set and not empty, else git config user.email in dir, else
// nobody. A value that is no email address stands for nobody.
func defaultViewer(ctx context.Context, dir string) string {
	email := getenv("GIT_AUTHOR_EMAIL")
	if email == "" {
		values, err := git.Runner{}.Config(ctx, git.Repo{Dir: dir}, nil, `^user\.email$`)
		if err == nil {
			email = values["user.email"]
		}
	}
	if (tablo.Params{Viewer: email}).Check(tablo.Gates) != nil {
		return ""
	}
	return email
}

// versionData is the data of the version command.
type versionData struct {
	Version  string `json:"version"`
	Tableaux string `json:"tableaux"`
	Schema   string `json:"schema"`
}

// versionEnvelope returns the version command's envelope, which reads no
// repository.
func versionEnvelope() *tablo.Envelope {
	return &tablo.Envelope{
		Schema:  tablo.SchemaVersion,
		Command: "version",
		Data: versionData{
			Version:  tablo.Version,
			Tableaux: strconv.Itoa(tablo.AcceptedMajor) + "." + strconv.Itoa(tablo.AcceptedMinor),
			Schema:   tablo.SchemaVersion,
		},
	}
}

// trailerData is the data of the trailer command.
type trailerData struct {
	Trailer string  `json:"trailer"`
	Kind    string  `json:"kind"`
	Task    *string `json:"task"`
	Gate    *string `json:"gate"`
	Model   *string `json:"model"`
}

// trailerEnvelope returns the trailer command's envelope: over the snapshot
// it checked the trailer against, or over none. It carries no diagnostics:
// the trailer is the result, and validate reports the project.
func trailerEnvelope(t tablo.Trailer, s *tablo.Snapshot) (*tablo.Envelope, error) {
	line, err := t.Line()
	if err != nil {
		return nil, err
	}
	str := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	e := &tablo.Envelope{
		Schema:  tablo.SchemaVersion,
		Command: "trailer",
		Data:    trailerData{Trailer: line, Kind: t.Kind, Task: str(t.Task), Gate: str(t.Gate), Model: str(t.Model)},
	}
	if s != nil {
		src := s.Source()
		e.Source, e.Project = &src, s.Project()
	}
	return e, nil
}

// trailerLine returns the line a trailer envelope holds.
func trailerLine(e *tablo.Envelope) string { return e.Data.(trailerData).Trailer }

// plainDiagnostics writes one line per diagnostic,
// <path>:<line>:<col>: <severity>: <code> <message>, without the parts a
// diagnostic lacks; nothing for none.
func plainDiagnostics(list []tablo.Diagnostic) []byte {
	var b strings.Builder
	for _, d := range list {
		if d.Path != "" {
			b.WriteString(d.Path + ":")
			if d.Line > 0 {
				b.WriteString(strconv.Itoa(d.Line) + ":")
				if d.Col > 0 {
					b.WriteString(strconv.Itoa(d.Col) + ":")
				}
			}
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%s: %s %s\n", d.Severity, d.Code, d.Message)
	}
	return []byte(b.String())
}
