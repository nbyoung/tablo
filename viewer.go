package tablo

import (
	"context"

	"github.com/nbyoung/tablo/internal/git"
)

// DefaultViewer returns the viewer when the caller names none, by the rule
// of 4ed9 decision 18 (a): author, when it is not empty; else the user.email
// that Git's configuration gives in dir, the repository's and the user's;
// else nobody, "". A value that is no email address stands for nobody, and
// so does a directory Git cannot read. The library reads no environment:
// the caller passes the value of GIT_AUTHOR_EMAIL as author.
//
//	viewer := tablo.DefaultViewer(ctx, dir, os.Getenv("GIT_AUTHOR_EMAIL"))
func DefaultViewer(ctx context.Context, dir, author string) string {
	email := author
	if email == "" {
		if dir == "" {
			dir = "."
		}
		values, err := git.Runner{}.Config(ctx, git.Repo{Dir: dir}, nil, `^user\.email$`)
		if err == nil {
			email = values["user.email"]
		}
	}
	if !emailPattern.MatchString(email) {
		return ""
	}
	return email
}
