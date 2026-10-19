// Package corpustest gives a test the tableaux conformance corpus: a copy of
// the corpus at one locked tableaux commit, embedded into the package, the
// entries built from it in a work directory, the labels that name their
// commits and the rules RULES.md lists.
//
// The copy lives in testdata/corpus, and corpus.lock.yaml names the commit it
// comes from and a digest of its files; corpus.sh maintains both. The
// environment variable TABLO_CORPUS names another corpus directory in place of
// the copy, and corpustest then reads that directory and never writes into it.
//
// No package outside tests imports corpustest, so it adds nothing to the tablo
// binary. A package that uses it calls Main from its TestMain.
package corpustest

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

//go:embed all:testdata/corpus
var vendored embed.FS

//go:embed corpus.lock.yaml
var lockFile []byte

// root is the directory under which every work directory of this process
// lives. Main removes it.
var (
	rootMu sync.Mutex
	root   string
	opened = map[string]*Corpus{}
)

// copied lists what Open copies out of a corpus that TABLO_CORPUS names.
var copied = []string{"entries", "base", "build.sh", "lib.sh", "RULES.md"}

// Main runs a package's tests and then removes the work directory that Open
// made, unless the environment holds TABLO_KEEP; it returns the exit code. A
// package that uses the corpus calls it:
//
//	func TestMain(m *testing.M) { os.Exit(corpustest.Main(m)) }
func Main(m *testing.M) int {
	code := m.Run()
	rootMu.Lock()
	defer rootMu.Unlock()
	if root != "" {
		if os.Getenv("TABLO_KEEP") != "" {
			fmt.Fprintf(os.Stderr, "corpustest: work directory kept at %s\n", root)
		} else {
			_ = os.RemoveAll(root)
		}
	}
	return code
}

// Corpus is the corpus of one test process: the embedded copy, or the
// directory TABLO_CORPUS names, written into a work directory.
type Corpus struct {
	Origin   string // "testdata/corpus", or the value of TABLO_CORPUS
	Vendored bool   // true for the embedded copy
	Work     string // the work directory: entries/, base/, build.sh, lib.sh, RULES.md, build/

	mu     sync.Mutex
	builds map[string]*build
}

// A build is one entry's build, which runs once.
type build struct {
	once sync.Once
	err  error
}

// Open returns the process's corpus and makes the work directory on first
// use. It skips tb when git or sh is absent, and fails tb instead under CI.
func Open(tb testing.TB) *Corpus {
	tb.Helper()
	for _, tool := range []string{"git", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			if os.Getenv("CI") != "" {
				tb.Fatalf("corpus: %s is not on the path, and CI does not skip", tool)
			}
			tb.Skipf("%s is not on the path, so the corpus cannot be built", tool)
			return nil
		}
	}
	origin := os.Getenv("TABLO_CORPUS")
	rootMu.Lock()
	defer rootMu.Unlock()
	if c, ok := opened[origin]; ok {
		return c
	}
	if root == "" {
		dir, err := os.MkdirTemp("", "tablo-corpus-")
		if err != nil {
			tb.Fatalf("corpus: %v", err)
		}
		root = dir
	}
	work, err := os.MkdirTemp(root, "work-")
	if err != nil {
		tb.Fatalf("corpus: %v", err)
	}
	c := &Corpus{Origin: origin, Work: work, builds: map[string]*build{}}
	if origin == "" {
		c.Origin = "testdata/corpus"
		c.Vendored = true
		err = c.writeVendored()
	} else {
		err = c.copyFrom(origin)
	}
	if err != nil {
		tb.Fatalf("corpus: %v", err)
	}
	opened[origin] = c
	return c
}

// writeVendored writes the embedded copy into the work directory.
func (c *Corpus) writeVendored() error {
	return fs.WalkDir(vendored, "testdata/corpus", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, "testdata/corpus"), "/")
		target := filepath.Join(c.Work, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := vendored.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// copyFrom copies what a build needs out of the corpus directory dir.
func (c *Corpus) copyFrom(dir string) error {
	if info, err := os.Stat(filepath.Join(dir, "entries")); err != nil || !info.IsDir() {
		return fmt.Errorf("TABLO_CORPUS=%s holds no entries directory", dir)
	}
	for _, name := range copied {
		src := filepath.Join(dir, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			target := filepath.Join(c.Work, rel)
			if d.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			if !d.Type().IsRegular() {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0o644)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Names lists the entries, sorted.
func (c *Corpus) Names() []string {
	list, err := os.ReadDir(filepath.Join(c.Work, "entries"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range list {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// Env is the closed environment for git and for the command under test. It
// turns off git's automatic maintenance, which detaches after a commit and
// can still write into a repository that a rebuild removes.
func (c *Corpus) Env() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + c.Work,
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=maintenance.auto",
		"GIT_CONFIG_VALUE_0=false",
		"GIT_CONFIG_KEY_1=gc.auto",
		"GIT_CONFIG_VALUE_1=0",
		"TZ=UTC",
		"LC_ALL=C",
	}
}

// Entry is one corpus entry, ready to read.
type Entry struct {
	Name     string
	Expected *yaml.Node        // the mapping at the root of expected.yaml, with line numbers
	Language string            // the language field
	Valid    bool              // the valid field
	Repo     string            // the repository to read: build/<name>, or the source repository
	Ref      string            // the ref in view: ref, source.ref, or main
	Trunk    string            // the branch the caller names; empty unless trunk is { caller: <branch> }
	Replace  map[string]string // absolute URL to built repository, from the replace field
	InPlace  bool              // the entry has source

	labels map[string]string // label to hash
	hashes map[string]string // hash to label
}

// Entry reads an entry's expected.yaml and builds its repository on first
// use. It skips tb for a read-in-place entry of the embedded copy.
func (c *Corpus) Entry(tb testing.TB, name string) *Entry {
	tb.Helper()
	e, err := c.read(name)
	if err != nil {
		tb.Fatalf("corpus: %v", err)
	}
	if e.InPlace && c.Vendored {
		tb.Skipf("%s is read in place, and the embedded copy has no tableaux checkout beside it; set TABLO_CORPUS", name)
		return nil
	}
	if !e.InPlace {
		if err := c.built(name); err != nil {
			tb.Fatalf("corpus: %v", err)
		}
		if err := e.readLabels(c.Work); err != nil {
			tb.Fatalf("corpus: %v", err)
		}
	}
	return e
}

// read parses an entry's expected.yaml without building it.
func (c *Corpus) read(name string) (*Entry, error) {
	file := filepath.Join(c.Work, "entries", name, "expected.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("entry %s: %w", name, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("entry %s: expected.yaml: %w", name, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("entry %s: expected.yaml is not one mapping", name)
	}
	top := doc.Content[0]
	e := &Entry{Name: name, Expected: top, Ref: "main"}
	if got := scalar(top, "entry"); got != name {
		return nil, fmt.Errorf("entry %s: expected.yaml names the entry %q", name, got)
	}
	e.Language = scalar(top, "language")
	e.Valid = scalar(top, "valid") == "true"
	if t := key(top, "trunk"); t != nil && t.Kind == yaml.MappingNode {
		e.Trunk = scalar(t, "caller")
	}
	e.Repo = filepath.Join(c.Work, "build", name)
	if ref := scalar(top, "ref"); ref != "" {
		e.Ref = ref
	}
	if src := key(top, "source"); src != nil {
		e.InPlace = true
		dir := filepath.Join(c.Work, "entries", name)
		if !c.Vendored {
			dir = filepath.Join(c.Origin, "entries", name)
			if abs, err := filepath.Abs(dir); err == nil {
				dir = abs
			}
		}
		e.Repo = filepath.Clean(filepath.Join(dir, scalar(src, "repository")))
		e.Ref = scalar(src, "ref")
	}
	if r := key(top, "replace"); r != nil && r.Kind == yaml.MappingNode {
		e.Replace = map[string]string{}
		for i := 0; i+1 < len(r.Content); i += 2 {
			e.Replace[r.Content[i].Value] = filepath.Join(c.Work, "build", name+"."+r.Content[i+1].Value)
		}
	}
	return e, nil
}

// key returns the value of a key in a mapping node, or nil.
func key(m *yaml.Node, name string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == name {
			return m.Content[i+1]
		}
	}
	return nil
}

// scalar returns the text of a scalar key in a mapping node, or "".
func scalar(m *yaml.Node, name string) string {
	if n := key(m, name); n != nil && n.Kind == yaml.ScalarNode {
		return n.Value
	}
	return ""
}

// built builds one entry once per corpus and returns the error of that build.
func (c *Corpus) built(name string) error {
	c.mu.Lock()
	b := c.builds[name]
	if b == nil {
		b = &build{}
		c.builds[name] = b
	}
	c.mu.Unlock()
	b.once.Do(func() { b.err = c.runBuild(name) })
	return b.err
}

// runBuild runs build.sh for one entry in the work directory under the closed
// environment.
func (c *Corpus) runBuild(name string) error {
	cmd := exec.Command("sh", "build.sh", name)
	cmd.Dir = c.Work
	cmd.Env = c.Env()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("building %s: %w\n%s", name, err, bytes.TrimSpace(out))
	}
	return nil
}

// readLabels reads build/<name>.labels.txt and the labels files of the
// entry's subprojects, build/<name>.<sub>.labels.txt.
func (e *Entry) readLabels(work string) error {
	e.labels = map[string]string{}
	e.hashes = map[string]string{}
	files, err := filepath.Glob(filepath.Join(work, "build", e.Name+".*labels.txt"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	// The entry's own file first, so its labels win over a subproject's.
	own := filepath.Join(work, "build", e.Name+".labels.txt")
	sort.SliceStable(files, func(i, j int) bool { return files[i] == own && files[j] != own })
	for _, f := range files {
		in, err := os.Open(f)
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(in)
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) != 2 {
				continue
			}
			if _, dup := e.labels[fields[0]]; !dup {
				e.labels[fields[0]] = fields[1]
			}
			if _, dup := e.hashes[fields[1]]; !dup {
				e.hashes[fields[1]] = fields[0]
			}
		}
		err = sc.Err()
		if cerr := in.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

var forty = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Hash resolves a label through the entry's labels file and then through the
// labels files of its subprojects (build/<name>.*.labels.txt). Forty
// lowercase hexadecimal digits resolve to themselves.
func (e *Entry) Hash(label string) (hash string, ok bool) {
	if forty.MatchString(label) {
		return label, true
	}
	hash, ok = e.labels[label]
	return hash, ok
}

// Label is the reverse of Hash, for messages; empty when no label names the hash.
func (e *Entry) Label(hash string) string { return e.hashes[hash] }

// Rule is one row of a rule table in RULES.md.
type Rule struct {
	ID       string   // "P1" … "H6"
	Severity string   // error, warning or information
	Entries  []string // the entries the row names
}

var (
	ruleID  = regexp.MustCompile(`^[A-Z][0-9]+$`)
	entryRe = regexp.MustCompile("`([^`]+)`")
)

// Rules reads the rule tables of RULES.md, in file order. It takes each table
// row of the form | <id> | <severity> | … | <entries> | whose first cell
// matches ^[A-Z][0-9]+$, as check.py does.
func (c *Corpus) Rules(tb testing.TB) []Rule {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join(c.Work, "RULES.md"))
	if err != nil {
		tb.Fatalf("corpus: %v", err)
	}
	var rules []Rule
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 3 || !ruleID.MatchString(strings.TrimSpace(cells[0])) {
			continue
		}
		r := Rule{ID: strings.TrimSpace(cells[0]), Severity: strings.TrimSpace(cells[1])}
		for _, m := range entryRe.FindAllStringSubmatch(cells[len(cells)-1], -1) {
			r.Entries = append(r.Entries, m[1])
		}
		rules = append(rules, r)
	}
	return rules
}

// Digest computes the digest of the embedded copy, as corpus.sh digest does:
// the SHA-256 of the lines "<sha256 of the file>  ./<path>", one per regular
// file outside build/, sorted bytewise by path.
func Digest() string {
	var paths []string
	err := fs.WalkDir(vendored, "testdata/corpus", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := "./" + strings.TrimPrefix(p, "testdata/corpus/")
		if d.IsDir() || strings.HasPrefix(rel, "./build/") {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		panic(err)
	}
	sort.Strings(paths)
	var lines strings.Builder
	for _, rel := range paths {
		data, err := vendored.ReadFile(path.Join("testdata/corpus", rel))
		if err != nil {
			panic(err)
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&lines, "%s  %s\n", hex.EncodeToString(sum[:]), rel)
	}
	total := sha256.Sum256([]byte(lines.String()))
	return hex.EncodeToString(total[:])
}

// Lock reads corpus.lock.yaml, embedded beside the copy.
func Lock() (repository, commit, tooling, digest string) {
	var lock struct {
		Repository string `yaml:"repository"`
		Commit     string `yaml:"commit"`
		Tooling    string `yaml:"tooling"`
		Digest     string `yaml:"digest"`
	}
	if err := yaml.Unmarshal(lockFile, &lock); err != nil {
		panic(err)
	}
	return lock.Repository, lock.Commit, lock.Tooling, lock.Digest
}
