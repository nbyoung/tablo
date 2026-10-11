package views

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The helpers of the Derivation's tests for a repository a test changes.

// copyTree copies a directory with its symbolic links and file modes.
func copyTree(t testing.TB, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm()|0o200)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// clone copies a built corpus repository into a temporary directory, for a
// test that changes it, and returns the copy's path.
func clone(t testing.TB, name string) string {
	t.Helper()
	to := filepath.Join(t.TempDir(), name)
	copyTree(t, builtAt(t, name), to)
	return to
}

// gitRun runs the git executable in dir and returns what it prints, trimmed.
// It skips the test when the host has no git.
func gitRun(t testing.TB, dir string, args ...string) string {
	t.Helper()
	exe, err := exec.LookPath("git")
	if err != nil {
		t.Skip("the host has no git executable")
	}
	cmd := exec.Command(exe, append([]string{"-c", "protocol.file.allow=always", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}
