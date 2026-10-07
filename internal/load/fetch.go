package load

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/nbyoung/tablo/internal/git"
)

// fetchable matches the URLs Fetch passes to git: the five schemes that name
// a host or a file. A transport such as ext::, which runs a command, and a
// URL that git would take as an option match nothing here.
var fetchable = regexp.MustCompile(`^(https|http|ssh|git|file)://`)

// clonePath returns where the cache keeps the clone of url:
// <CacheDir>/clones/<sha256 of url>.git.
func (l *Loader) clonePath(url string) (string, error) {
	cache := l.options.CacheDir
	if cache == "" {
		user, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		cache = filepath.Join(user, "tableaux")
	}
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(cache, "clones", hex.EncodeToString(sum[:])+".git"), nil
}

// Fetch brings url into the cache so that commit is there. It clones url
// bare, or fetches every branch into the clone the cache holds, and fails
// when commit is still absent. It is the one place the Loader reaches a
// network, and Load comes here only under Options.Fetch.
func (l *Loader) Fetch(ctx context.Context, url, commit string) error {
	if !fetchable.MatchString(url) {
		return fmt.Errorf("fetch %q: the URL is not https, http, ssh, git or file", url)
	}
	if !fullHash.MatchString(commit) {
		return fmt.Errorf("fetch %q: the commit %q is no full hash", url, commit)
	}
	clone, err := l.clonePath(url)
	if err != nil {
		return err
	}
	if exists(clone) {
		// The URL follows --, so git never reads it as an option.
		_, err = l.run.Run(ctx, git.Repo{GitDir: clone}, nil, "fetch", "--quiet", "--", url, "+refs/heads/*:refs/heads/*")
	} else {
		err = l.clone(ctx, url, clone)
	}
	if err != nil {
		return fmt.Errorf("fetch %q: %w", url, err)
	}
	found, err := l.run.Discover(ctx, git.Repo{GitDir: clone}, commit)
	if err != nil {
		return fmt.Errorf("fetch %q: %w", url, err)
	}
	if found.Commit == "" {
		return fmt.Errorf("fetch %q: the repository does not hold the commit %s", url, commit)
	}
	return nil
}

// clone clones url bare beside its place in the cache and renames the clone
// into place, so that a concurrent load never reads half a clone.
func (l *Loader) clone(ctx context.Context, url, clone string) error {
	parent := filepath.Dir(clone)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	scratch, err := os.MkdirTemp(parent, "clone-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if _, err := l.run.Run(ctx, git.Repo{}, nil, "clone", "--quiet", "--bare", "--", url, scratch); err != nil {
		return err
	}
	// A concurrent fetch may win the rename; its clone serves as well.
	if err := os.Rename(scratch, clone); err != nil && !exists(clone) {
		return err
	}
	return nil
}
