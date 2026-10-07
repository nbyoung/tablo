package tablo

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestTrailerLines is the repository-free part of T11: the four lines, the
// reviewed one as trailer.json carries it, and the refusals.
func TestTrailerLines(t *testing.T) {
	tests := []struct {
		t    Trailer
		want string
	}{
		{Trailer{Kind: "authorised", Task: "9f31"}, "Authorised: 9f31"},
		{Trailer{Kind: "reviewed", Task: "9f31", Gate: "validate"}, "Reviewed: 9f31 validate"},
		{Trailer{Kind: "reaffirmed", Task: "07e0"}, "Reaffirmed: 07e0"},
		{Trailer{Kind: "model", Model: "claude-sonnet-5-5"}, "Model: claude-sonnet-5-5"},
		{Trailer{Kind: "model", Model: "vendor/model:1.0~rc[1]"}, "Model: vendor/model:1.0~rc[1]"},
		{Trailer{Kind: "reviewed", Task: "1000", Gate: "a_b-c9"}, "Reviewed: 1000 a_b-c9"},
	}
	for _, tc := range tests {
		got, err := tc.t.Line()
		if err != nil || got != tc.want {
			t.Errorf("%+v: Line = %q, %v; want %q", tc.t, got, err, tc.want)
		}
	}

	var doc struct {
		Data struct {
			Trailer string  `json:"trailer"`
			Kind    string  `json:"kind"`
			Task    string  `json:"task"`
			Gate    string  `json:"gate"`
			Model   *string `json:"model"`
		} `json:"data"`
	}
	if err := json.Unmarshal(golden(t, "trailer.json"), &doc); err != nil {
		t.Fatal(err)
	}
	g := doc.Data
	line, err := Trailer{Kind: g.Kind, Task: g.Task, Gate: g.Gate}.Line()
	if err != nil || line != g.Trailer {
		t.Errorf("trailer.json: Line = %q, %v; want %q", line, err, g.Trailer)
	}
	if !bytes.Equal(trailerEnvelope().JSON(), golden(t, "trailer.json")) {
		t.Errorf("the trailer envelope differs from trailer.json")
	}

	bad := []struct {
		t    Trailer
		want string
	}{
		{Trailer{}, "trailer kind"},
		{Trailer{Kind: "approved", Task: "9f31"}, "approved"},
		{Trailer{Kind: "authorised"}, "task id"},
		{Trailer{Kind: "authorised", Task: "9F31"}, "9F31"},
		{Trailer{Kind: "authorised", Task: "9f3"}, "9f3"},
		{Trailer{Kind: "authorised", Task: "9f31x"}, "9f31x"},
		{Trailer{Kind: "authorised", Task: "9g31"}, "9g31"},
		{Trailer{Kind: "authorised", Task: "9f31", Gate: "design"}, "no gate"},
		{Trailer{Kind: "authorised", Task: "9f31", Model: "m"}, "no model"},
		{Trailer{Kind: "reaffirmed", Task: "9f31", Gate: "design"}, "no gate"},
		{Trailer{Kind: "reviewed", Task: "9f31"}, "needs a gate"},
		{Trailer{Kind: "reviewed", Task: "9f31", Gate: "Design"}, "Design"},
		{Trailer{Kind: "reviewed", Task: "9f31", Gate: "a b"}, "a b"},
		{Trailer{Kind: "model"}, "identifier"},
		{Trailer{Kind: "model", Model: "two words"}, "two words"},
		{Trailer{Kind: "model", Model: "tab\tchar"}, "printable"},
		{Trailer{Kind: "model", Model: "café"}, "printable"},
		{Trailer{Kind: "model", Model: "line\nbreak"}, "printable"},
		{Trailer{Kind: "model", Model: "m", Task: "9f31"}, "no task"},
		{Trailer{Kind: "model", Model: "m", Gate: "design"}, "no task or gate"},
	}
	for _, tc := range bad {
		line, err := tc.t.Line()
		u, ok := err.(*UsageError)
		if !ok || line != "" {
			t.Errorf("%+v: Line = %q, %v; want a *UsageError", tc.t, line, err)
			continue
		}
		if !strings.Contains(u.Msg, tc.want) {
			t.Errorf("%+v: message %q does not contain %q", tc.t, u.Msg, tc.want)
		}
	}
}

// TestGitReadsTheTrailersBack checks that git interpret-trailers parses each
// line, alone and together, as the trailer the line names.
func TestGitReadsTheTrailersBack(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	trailers := []Trailer{
		{Kind: "authorised", Task: "9f31"},
		{Kind: "reviewed", Task: "9f31", Gate: "validate"},
		{Kind: "reaffirmed", Task: "07e0"},
		{Kind: "model", Model: "claude-sonnet-5-5"},
	}
	var lines []string
	for _, tr := range trailers {
		line, err := tr.Line()
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	parse := func(block ...string) string {
		cmd := exec.Command(git, "interpret-trailers", "--parse")
		cmd.Stdin = strings.NewReader("Subject line\n\nBody text.\n\n" + strings.Join(block, "\n") + "\n")
		cmd.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "LC_ALL=C"}
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git interpret-trailers: %v", err)
		}
		return string(out)
	}
	for _, line := range lines {
		if got := parse(line); got != line+"\n" {
			t.Errorf("git reads %q back as %q", line, got)
		}
	}
	if got, want := parse(lines...), strings.Join(lines, "\n")+"\n"; got != want {
		t.Errorf("git reads the block back as %q, want %q", got, want)
	}
}
