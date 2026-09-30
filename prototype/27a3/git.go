package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// git runs git in repo and returns trimmed standard output.
func git(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// show returns a file's text at a commit, or "" and false when it is absent.
func show(repo, commit, path string) (string, bool) {
	out, err := git(repo, "show", commit+":"+path)
	return out, err == nil
}

// listFiles returns the paths under dir at a commit.
func listFiles(repo, commit, dir string) []string {
	out, err := git(repo, "ls-tree", "-r", "--name-only", commit, dir+"/")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}
