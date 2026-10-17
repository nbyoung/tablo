package tablo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultViewer is the viewer's rule of 4ed9 decision 18 (a): the author
// email the caller passes, else the repository's user.email, else nobody,
// and a value that is no email is nobody. Git reads no configuration of the
// host's: the global and system files are empty files of the test's own.
func TestDefaultViewer(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_SYSTEM", empty)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	configured := build(t)
	configured.git("olive", "2026-09-01", "config", "user.email", "ben@example.org")
	wrong := build(t)
	wrong.git("olive", "2026-09-01", "config", "user.email", "Ben Okafor")
	bare := build(t)
	nowhere := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(nowhere))
	cases := []struct {
		name, dir, author, want string
	}{
		{"the author email set and valid", configured.dir, "pat@example.org", "pat@example.org"},
		{"the author email set and no email", configured.dir, "not an email", ""},
		{"no author email and a user.email", configured.dir, "", "ben@example.org"},
		{"no author email and a user.email that is no email", wrong.dir, "", ""},
		{"neither", bare.dir, "", ""},
		{"no author email and no repository", nowhere, "", ""},
		{"the author email and no repository", nowhere, "pat@example.org", "pat@example.org"},
	}
	for _, c := range cases {
		if got := DefaultViewer(context.Background(), c.dir, c.author); got != c.want {
			t.Errorf("%s: the viewer is %q; want %q", c.name, got, c.want)
		}
	}
}
