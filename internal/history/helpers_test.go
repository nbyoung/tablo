package history

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// corpus returns the directory of the built conformance corpus, and skips
// the test when it is absent. It carries the Loader's stand-in for the
// helper of task 4b4f (assumption A7): TABLEAUX_CORPUS, else
// ../tableaux/corpus/build beside the checkout.
func corpus(t testing.TB) string {
	t.Helper()
	dir := os.Getenv("TABLEAUX_CORPUS")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "tableaux", "corpus", "build")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Skipf("the built corpus is absent at %s; set TABLEAUX_CORPUS", dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// entry returns the path of a built corpus repository, which a test reads
// and never changes.
func entry(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join(corpus(t), name)
}

// labelsOf returns the commits a labels file names.
func labelsOf(t testing.TB, file string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	hashes := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if label, hash, ok := strings.Cut(line, " "); ok {
			hashes[label] = hash
		}
	}
	return hashes
}

// labels returns the commits the build names in <entry>.labels.txt.
func labels(t testing.TB, entry string) map[string]string {
	t.Helper()
	return labelsOf(t, filepath.Join(corpus(t), entry+".labels.txt"))
}

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
	copyTree(t, entry(t, name), to)
	return to
}

// gitEnv is the fixed identity and date of every commit a test makes, with
// no configuration of the host's.
func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=Olive Marsh", "GIT_AUTHOR_EMAIL=olive@example.org",
		"GIT_COMMITTER_NAME=Olive Marsh", "GIT_COMMITTER_EMAIL=olive@example.org",
		"GIT_AUTHOR_DATE=2026-09-01T12:00:00+00:00", "GIT_COMMITTER_DATE=2026-09-01T12:00:00+00:00",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
	)
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

// cases builds the repository of design 27a3's cases under a temporary
// directory and returns its path and the commits it labels. It skips the
// test when the host has no shell or no git.
func cases(t testing.TB) (repo string, hashes map[string]string) {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("the host has no sh executable")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("the host has no git executable")
	}
	script, err := filepath.Abs(filepath.Join("..", "derive", "testdata", "cases.sh"))
	if err != nil {
		t.Fatal(err)
	}
	repo = filepath.Join(t.TempDir(), "cases")
	cmd := exec.Command(sh, script, repo)
	cmd.Env = gitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cases.sh: %v\n%s", err, out)
	}
	return repo, labelsOf(t, repo+".labels.txt")
}

// write puts files under dir, each path relative to it.
func write(t testing.TB, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// project is the smallest project a test commits: a root and nothing else.
// trunk is the line of version.yaml that names the trunk, or "".
func project(trunk string) map[string]string {
	return map[string]string{
		".tableaux/version.yaml":    "tableaux: 0.3.1\n" + trunk,
		".tableaux/gates.yaml":      "gates:\n  - { key: undefined }\n  - { key: defined }\nstates:\n  - { key: undefined, severity: 0 }\n  - { key: complete, severity: 0 }\n",
		".tableaux/tasks/e4a1.yaml": "title: Root\nassignee: olive@example.org\n",
	}
}

// loaded reads a source with a Loader whose cache is an empty temporary
// directory, and fails the test on a load error.
func loaded(t testing.TB, options load.Options, dir, ref string) *model.Project {
	t.Helper()
	if options.CacheDir == "" {
		options.CacheDir = t.TempDir()
	}
	p, err := load.New(options).Load(context.Background(), load.Source{Dir: dir, Ref: ref})
	if err != nil {
		t.Fatalf("Load(%q, %q): %v", dir, ref, err)
	}
	return p
}

// pass reads the passes of a project and returns the home project's.
func pass(t testing.TB, r *Reader, p *model.Project, trunk string) *Log {
	t.Helper()
	set, err := r.Read(context.Background(), p, trunk)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return set.Of(p)
}
