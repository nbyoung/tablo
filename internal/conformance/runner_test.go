package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// callTimeout bounds every call of the command.
const callTimeout = 30 * time.Second

// observerEmail is the viewer of every call: it holds no role in any entry.
const observerEmail = "observer@example.org"

var (
	binOnce sync.Once
	binPath string
	binErr  error
	binDir  string
)

// binary returns the command under test: the file TABLO_BIN names, else the
// command built once from cmd/tablo into a temporary directory that TestMain
// removes.
func binary(tb testing.TB) string {
	tb.Helper()
	binOnce.Do(func() {
		if p := os.Getenv("TABLO_BIN"); p != "" {
			binPath, binErr = filepath.Abs(p)
			return
		}
		dir, err := os.MkdirTemp("", "tablo-conformance-")
		if err != nil {
			binErr = err
			return
		}
		binDir = dir
		out := filepath.Join(dir, "tablo")
		cmd := exec.Command("go", "build", "-o", out, "../../cmd/tablo")
		if msg, err := cmd.CombinedOutput(); err != nil {
			binErr = fmt.Errorf("go build ./cmd/tablo: %v\n%s", err, msg)
			return
		}
		binPath = out
	})
	if binErr != nil {
		tb.Fatalf("command: %v", binErr)
	}
	return binPath
}

// A runner calls the command under a closed environment.
type runner struct {
	bin     string
	env     []string
	timeout time.Duration
}

// A reply is one answered call.
type reply struct {
	code int            // the exit code
	env  map[string]any // the envelope
	line string         // the command line, for a message
}

// run calls the command and returns its envelope. The error text begins with
// "command:" and names the call: an exit code of 2 or 3, a timeout, output that
// is not one JSON envelope, or a schema other than tablo/1.
func (r *runner) run(args ...string) (*reply, error) {
	timeout := r.timeout
	if timeout == 0 {
		timeout = callTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.bin, args...)
	cmd.Env = r.env
	cmd.WaitDelay = time.Second
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	rep := &reply{line: r.bin + " " + strings.Join(args, " ")}
	err := cmd.Run()
	if ctx.Err() != nil {
		return rep, fmt.Errorf("command: %s: timeout after %s", rep.line, timeout)
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		rep.code = exit.ExitCode()
	default:
		return rep, fmt.Errorf("command: %s: %v", rep.line, err)
	}
	if rep.code != 0 && rep.code != 1 {
		return rep, fmt.Errorf("command: %s: exit %d: %s", rep.line, rep.code, firstLine(errOut.String()))
	}
	dec := json.NewDecoder(&out)
	if err := dec.Decode(&rep.env); err != nil {
		return rep, fmt.Errorf("command: %s: output is not a JSON envelope: %s", rep.line, firstLine(errOut.String()))
	}
	if _, err := dec.Token(); err != io.EOF {
		return rep, fmt.Errorf("command: %s: output is more than one JSON value", rep.line)
	}
	if s, _ := rep.env["schema"].(string); s != "tablo/1" {
		return rep, fmt.Errorf("command: %s: schema %q, not tablo/1", rep.line, s)
	}
	return rep, nil
}

// firstLine returns the first line of standard error, or a word that says there is none.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(no message on standard error)"
	}
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// languageMajor reads the language version the command accepts, as major.minor,
// through version --json (A7).
func (r *runner) language() (string, error) {
	rep, err := r.run("version", "--json")
	if err != nil {
		return "", err
	}
	data := asMap(rep.env["data"])
	v := text(data["tableaux"])
	if v == "" {
		return "", fmt.Errorf("command: %s: data.tableaux is absent", rep.line)
	}
	return v, nil
}

// major is the part of a version before the first dot.
func major(v string) string {
	m, _, _ := strings.Cut(v, ".")
	return m
}
