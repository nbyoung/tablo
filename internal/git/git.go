// Package git is the one place that runs the git executable: it discovers a
// repository, runs a command in it, lists trees and the index, and reads
// blobs in one batch. Every command runs with --no-optional-locks, so a view
// that polls never takes the index lock, and prints paths under -z.
package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrNotFound says the git executable is not there to run.
var ErrNotFound = errors.New("git: the executable is not found")

// ExitError is a git command that ran and failed.
type ExitError struct {
	Args   []string // the arguments after the executable
	Code   int      // the exit code
	Stderr string   // what git wrote to standard error, trimmed
}

// Error returns the command's first argument words and what git wrote.
func (e *ExitError) Error() string {
	return fmt.Sprintf("git %s: exit %d: %s", strings.Join(e.Args, " "), e.Code, e.Stderr)
}

// Runner runs one git executable.
type Runner struct {
	Exe string // the executable; "" is "git"
}

// Repo addresses a repository. Exactly one field is set.
type Repo struct {
	// Dir is a directory of the repository the caller asked for. Git discovers
	// the repository from it, in the caller's environment, as a command the
	// person types there does.
	Dir string
	// GitDir is the Git directory of another repository, or the gitfile that
	// names it: a submodule or a clone. Git runs with --git-dir and without
	// the variables that bind a process to a repository, so a hook's GIT_DIR
	// never redirects the read. The repository serves as a store of objects:
	// the working tree its configuration names may be gone, as a submodule's
	// is once its checkout is removed, so git takes the Git directory itself
	// as the working tree, which no command here reads.
	GitDir string
}

// localEnv lists the variables `git rev-parse --local-env-vars` prints: the
// ones that bind a process to one repository.
var localEnv = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_CONFIG",
	"GIT_CONFIG_PARAMETERS",
	"GIT_CONFIG_COUNT",
	"GIT_OBJECT_DIRECTORY",
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_IMPLICIT_WORK_TREE",
	"GIT_GRAFT_FILE",
	"GIT_INDEX_FILE",
	"GIT_NO_REPLACE_OBJECTS",
	"GIT_REPLACE_REF_BASE",
	"GIT_PREFIX",
	"GIT_SHALLOW_FILE",
	"GIT_COMMON_DIR",
}

// foreignEnv returns env without the variables that bind a process to a
// repository, and without the configuration pairs GIT_CONFIG_COUNT counts.
func foreignEnv(env []string) []string {
	out := make([]string, 0, len(env))
next:
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		for _, local := range localEnv {
			if name == local {
				continue next
			}
		}
		if strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// gitfile returns the Git directory a gitfile names, or p itself when p is
// no gitfile: a submodule's checkout holds the file .git with one line,
// "gitdir: <path>", relative to the checkout.
func gitfile(p string) string {
	info, err := os.Stat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return p
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return p
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
	if !ok {
		return p
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(p), target)
	}
	return target
}

// command builds the process for args in repo.
func (r Runner) command(ctx context.Context, repo Repo, args []string) (*exec.Cmd, []string) {
	exe := r.Exe
	if exe == "" {
		exe = "git"
	}
	full := []string{"--no-optional-locks", "--literal-pathspecs"}
	if repo.GitDir != "" {
		store := gitfile(repo.GitDir)
		full = append(full, "--git-dir="+store, "--work-tree="+store)
	} else if repo.Dir != "" {
		full = append(full, "-C", repo.Dir)
	}
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, exe, full...)
	if repo.GitDir != "" {
		cmd.Env = foreignEnv(os.Environ())
	}
	return cmd, full
}

// Run runs git with args in repo, feeds it stdin, and returns what it writes
// to standard output. A command that exits with a failure returns its output
// so far and an *ExitError; a missing executable returns ErrNotFound.
func (r Runner) Run(ctx context.Context, repo Repo, stdin []byte, args ...string) ([]byte, error) {
	cmd, full := r.command(ctx, repo, args)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return stdout.Bytes(), &ExitError{Args: full, Code: exit.ExitCode(), Stderr: strings.TrimSpace(stderr.String())}
	}
	return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
}

// Discovery is what one rev-parse tells of a repository and a revision.
type Discovery struct {
	GitDir     string // the Git directory, absolute; a linked worktree has its own
	CommonDir  string // the common Git directory, absolute: the repository's identity
	Bare       bool   // the repository is bare; meaningful for Repo.Dir alone
	InWorkTree bool   // the directory is inside a working tree; false for Repo.GitDir
	Top        string // the working tree's root, absolute; "" unless InWorkTree
	Prefix     string // the directory relative to Top, with forward slashes and no trailing one; "" at the root
	Commit     string // the commit the revision names, in full; "" when it names none
	Detail     string // what git wrote to standard error when the revision names no commit
}

// ErrNoRepository says the directory is in no Git repository. The error that
// wraps it carries what git wrote.
var ErrNoRepository = errors.New("git: not a repository")

// Discover finds the repository of repo and resolves rev to a commit, in one
// process. A revision that names no commit leaves Commit empty and is no
// error. It needs Git 2.31, for --path-format.
func (r Runner) Discover(ctx context.Context, repo Repo, rev string) (Discovery, error) {
	out, err := r.Run(ctx, repo, nil, "rev-parse", "--path-format=absolute",
		"--git-dir", "--git-common-dir", "--is-bare-repository", "--is-inside-work-tree",
		"--show-prefix", "--show-cdup", "--end-of-options", rev+"^{commit}")
	var exit *ExitError
	if err != nil && !errors.As(err, &exit) {
		return Discovery{}, err
	}
	lines := strings.Split(string(out), "\n")
	// The four answers that every repository gives come first. A working tree
	// then prints its prefix and its way up, and a bare repository one empty
	// line; the marker ends them, and the commit follows it.
	marker := -1
	for i := 4; i < len(lines); i++ {
		if lines[i] == "--end-of-options" {
			marker = i
			break
		}
	}
	if marker < 0 {
		detail := ""
		if exit != nil {
			detail = exit.Stderr
		}
		return Discovery{}, fmt.Errorf("%w: %s", ErrNoRepository, detail)
	}
	d := Discovery{
		GitDir:     filepath.Clean(lines[0]),
		CommonDir:  filepath.Clean(lines[1]),
		Bare:       lines[2] == "true",
		InWorkTree: lines[3] == "true",
	}
	if d.InWorkTree && repo.Dir != "" && marker == 6 {
		d.Prefix = strings.TrimSuffix(lines[4], "/")
		// --show-cdup stays relative under --path-format=absolute.
		dir, absErr := filepath.Abs(repo.Dir)
		if absErr != nil {
			return Discovery{}, absErr
		}
		d.Top = filepath.Join(dir, filepath.FromSlash(lines[5]))
	}
	if exit != nil {
		d.Detail = exit.Stderr
		return d, nil
	}
	if marker+1 < len(lines) {
		d.Commit = lines[marker+1]
	}
	return d, nil
}

// Tree resolves rev to a tree's id, for a revision that is a tree and no
// commit. ok is false when rev names no tree.
func (r Runner) Tree(ctx context.Context, repo Repo, rev string) (id string, ok bool, err error) {
	out, err := r.Run(ctx, repo, nil, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{tree}")
	var exit *ExitError
	if errors.As(err, &exit) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(string(out)), true, nil
}

// Entry is one line of a tree or of the index.
type Entry struct {
	Mode string // 100644, 100755, 120000 a symbolic link, 160000 a gitlink, 040000 a tree
	ID   string // the object's id
	Path string // relative to the repository root, with forward slashes
}

// The modes that matter to a reader of a tree.
const (
	ModeTree    = "040000"
	ModeGitlink = "160000"
)

// Regular reports whether the entry is a regular file.
func (e Entry) Regular() bool { return strings.HasPrefix(e.Mode, "100") }

// LsTree lists the entries of treeish at paths, each relative to the
// repository root. With recursive it lists every file below each path; without
// it lists each path's own entry, among those of the trees above it and
// beside it when paths nest, so a caller looks its paths up. A path that is
// absent lists nothing.
func (r Runner) LsTree(ctx context.Context, repo Repo, treeish string, recursive bool, paths ...string) ([]Entry, error) {
	args := []string{"ls-tree", "-z", "--full-tree"}
	if recursive {
		args = append(args, "-r")
	} else {
		// Without -t, a path below another path named hides that one's own entry.
		args = append(args, "-t")
	}
	args = append(args, "--end-of-options", treeish, "--")
	args = append(args, paths...)
	out, err := r.Run(ctx, repo, nil, args...)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, record := range records(out) {
		// <mode> SP <type> SP <id> TAB <path>
		meta, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("git ls-tree: unexpected record %q", record)
		}
		entries = append(entries, Entry{Mode: fields[0], ID: fields[2], Path: path})
	}
	return entries, nil
}

// LsFiles lists the index entries at paths, each relative to the working
// tree's root; repo.Dir is that root. An unmerged path lists once per stage.
func (r Runner) LsFiles(ctx context.Context, repo Repo, paths ...string) ([]Entry, error) {
	args := append([]string{"ls-files", "-s", "-z", "--"}, paths...)
	out, err := r.Run(ctx, repo, nil, args...)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, record := range records(out) {
		// <mode> SP <id> SP <stage> TAB <path>
		meta, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("git ls-files: unexpected record %q", record)
		}
		entries = append(entries, Entry{Mode: fields[0], ID: fields[1], Path: path})
	}
	return entries, nil
}

// records splits output that ends each record with a NUL.
func records(out []byte) []string {
	parts := strings.Split(string(out), "\x00")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}

// Config returns the configuration values whose key matches pattern, by key
// as git prints it: the section and the variable in lower case, the
// subsection as written. source selects what git reads: nil for the
// repository's configuration and the user's, or arguments such as
// {"--blob", "<commit>:.gitmodules"} and {"--file", "<path>"}. A source that
// is absent gives no value and no error.
func (r Runner) Config(ctx context.Context, repo Repo, source []string, pattern string) (map[string]string, error) {
	args := append([]string{"config"}, source...)
	args = append(args, "-z", "--get-regexp", pattern)
	out, err := r.Run(ctx, repo, nil, args...)
	var exit *ExitError
	if err != nil && !errors.As(err, &exit) {
		return nil, err
	}
	values := map[string]string{}
	for _, record := range records(out) {
		key, value, _ := strings.Cut(record, "\n")
		values[key] = value // the last value of a key wins, as git reads it
	}
	return values, nil
}

// Blobs reads the objects ids name through one cat-file --batch and returns
// their contents in the same order. An id that names no object is an error.
func (r Runner) Blobs(ctx context.Context, repo Repo, ids []string) ([][]byte, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	out, err := r.Run(ctx, repo, []byte(strings.Join(ids, "\n")+"\n"), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(bytes.NewReader(out))
	blobs := make([][]byte, 0, len(ids))
	for _, id := range ids {
		// <id> SP <type> SP <size> LF <contents> LF, or <id> SP missing LF
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("git cat-file: no header for %s", id)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			return nil, fmt.Errorf("git cat-file: %s", strings.TrimSpace(header))
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("git cat-file: %s", strings.TrimSpace(header))
		}
		data := make([]byte, size+1)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, fmt.Errorf("git cat-file: short contents for %s", id)
		}
		blobs = append(blobs, data[:size])
	}
	return blobs, nil
}
