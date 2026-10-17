package tablo

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestAuthorTime is the commit's time with its zone (4ed9 decision 18 c): a
// commit authored late in the evening west of UTC, whose UTC day is the
// next, gives its time as git log --format=%aI prints it, and its day is
// Source.Date.
func TestAuthorTime(t *testing.T) {
	b := small(t)
	b.put(map[string]string{"status/b2c9.yaml": "gate: defined\nstate: nominal\nnote: Late\n"})
	b.git("pat", "2026-09-04", "add", "-A")
	const when = "2026-09-04T21:30:00-07:00"
	cmd := exec.Command("git", "-C", b.dir, "commit", "-q", "--no-gpg-sign", "-m", "Work late")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Pat Singh", "GIT_AUTHOR_EMAIL=pat@example.org",
		"GIT_COMMITTER_NAME=Pat Singh", "GIT_COMMITTER_EMAIL=pat@example.org",
		"GIT_AUTHOR_DATE="+when, "GIT_COMMITTER_DATE="+when,
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	printed := b.git("pat", "2026-09-04", "log", "-1", "--format=%aI")
	s := b.load("main")
	got := s.AuthorTime()
	if got != when || got != printed {
		t.Errorf("AuthorTime = %q; want %q, as git log prints %q", got, when, printed)
	}
	at, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("AuthorTime %q is no RFC 3339 time: %v", got, err)
	}
	date := s.Source().Date
	if date == nil || at.Format("2006-01-02") != *date || *date != "2026-09-04" {
		t.Errorf("the time's day is %s and Source.Date %v; want both 2026-09-04", at.Format("2006-01-02"), date)
	}
	if day := at.UTC().Format("2006-01-02"); day != "2026-09-05" {
		t.Errorf("the UTC day is %s; the test wants a zone that moves it", day)
	}
	if w := b.load(""); w.AuthorTime() != when {
		t.Errorf("the working tree's AuthorTime = %q; want HEAD's, %q", w.AuthorTime(), when)
	}
	tree := strings.TrimSpace(b.git("pat", "2026-09-04", "rev-parse", "HEAD^{tree}"))
	if got := b.load(tree).AuthorTime(); got != "" {
		t.Errorf("a tree's AuthorTime = %q; want none", got)
	}
}
