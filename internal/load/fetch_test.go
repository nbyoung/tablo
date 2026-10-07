package load

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// T23: the network waits to be asked. A file:// URL stands in for a host.
func TestFetch(t *testing.T) {
	lib := repository(t, "lib")
	plan(t, lib, "", "")
	first := commit(t, lib, "Plan the library")
	url := "file://" + filepath.ToSlash(lib)
	parent := repository(t, "parent")
	plan(t, parent, "", "{ subproject: { url: \""+url+"\", commit: "+first+" } }")
	commit(t, parent, "Name the library by URL")

	cache := t.TempDir()
	clones := func() []string {
		t.Helper()
		list, err := os.ReadDir(filepath.Join(cache, "clones"))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		var names []string
		for _, e := range list {
			names = append(names, e.Name())
		}
		return names
	}

	// Fetch is off: an empty cache gives NoClone, and the load makes no clone.
	quiet := New(Options{CacheDir: cache})
	if got := link(t, load(t, quiet, parent, "HEAD"), url); got.Form != model.URL || got.Problem != model.NoClone || got.Commit != first {
		t.Errorf("with an empty cache: %+v", got)
	}
	if names := clones(); len(names) != 0 {
		t.Errorf("a load with Fetch off left %v in the cache", names)
	}

	// Asked, the Loader clones into the cache, and the same Loader then resolves.
	if err := quiet.Fetch(t.Context(), url, first); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	names := clones()
	if len(names) != 1 || len(names[0]) != 64+len(".git") || !strings.HasSuffix(names[0], ".git") {
		t.Fatalf("the cache holds %v, want one clone named by the URL's SHA-256", names)
	}
	if !exists(filepath.Join(cache, "clones", names[0], "HEAD")) || exists(filepath.Join(cache, "clones", names[0], ".git")) {
		t.Errorf("the clone %s is not bare", names[0])
	}
	got := link(t, load(t, quiet, parent, "HEAD"), url)
	if got.Problem != model.Resolved || got.Commit != first || got.Project.Tasks["a000"] == nil {
		t.Errorf("after the fetch: %+v, %s", got, got.Detail)
	}

	// The library moves on. The clone lacks the new commit until a fetch brings it.
	write(t, lib, ".tableaux/status/a000.yaml", "gate: design\nstate: nominal\n")
	second := commit(t, lib, "Record a status")
	plan(t, parent, "", "{ subproject: { url: \""+url+"\", commit: "+second+" } }")
	if got := link(t, load(t, quiet, parent, ""), url); got.Problem != model.CommitAbsent || got.Commit != second {
		t.Errorf("a commit the cache lacks, Fetch off: %+v", got)
	}
	eager := New(Options{CacheDir: cache, Fetch: true})
	if got := link(t, load(t, eager, parent, ""), url); got.Problem != model.Resolved || got.Commit != second || got.Project.Statuses["a000"] == nil {
		t.Errorf("a commit the cache lacks, Fetch on: %+v, %s", got, got.Detail)
	}
	if names := clones(); len(names) != 1 {
		t.Errorf("the second fetch left %v in the cache", names)
	}
	// With Fetch on and an empty cache, one load clones and resolves.
	fresh := New(Options{CacheDir: t.TempDir(), Fetch: true})
	if got := link(t, load(t, fresh, parent, ""), url); got.Problem != model.Resolved || got.Commit != second {
		t.Errorf("an empty cache, Fetch on: %+v, %s", got, got.Detail)
	}
	// A URL the fetch cannot reach leaves NoClone and says why.
	plan(t, parent, "", "{ subproject: { url: \"file:///no/such/repository\", commit: "+second+" } }")
	if got := link(t, load(t, fresh, parent, ""), "file:///no/such/repository"); got.Problem != model.NoClone || got.Detail == "" {
		t.Errorf("a URL with no repository, Fetch on: %+v", got)
	}

	// What Fetch refuses, before git runs: a transport that runs a command,
	// a URL git would take as an option, a path, and a commit that is no hash.
	marker := filepath.Join(t.TempDir(), "ran")
	refused := []struct{ url, commit string }{
		{"ext::sh -c 'touch " + marker + "'", first},
		{"--upload-pack=touch " + marker, first},
		{"-u", first},
		{lib, first},
		{"ftp://example.org/lib.git", first},
		{"FILE://" + filepath.ToSlash(lib), first},
		{url, "main"},
		{url, "--upload-pack=touch " + marker},
		{url, first[:12]},
	}
	before := clones()
	for _, tt := range refused {
		if err := quiet.Fetch(t.Context(), tt.url, tt.commit); err == nil {
			t.Errorf("Fetch(%q, %q) is no error", tt.url, tt.commit)
		}
	}
	if exists(marker) {
		t.Error("a refused URL ran a command")
	}
	if after := clones(); len(after) != len(before) {
		t.Errorf("a refused fetch changed the cache: %v", after)
	}
	// A commit the repository does not hold fails after the fetch.
	if err := quiet.Fetch(t.Context(), url, strings.Repeat("0", 40)); err == nil || !strings.Contains(err.Error(), "does not hold") {
		t.Errorf("Fetch of an absent commit: %v", err)
	}
}
