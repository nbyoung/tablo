package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo"
)

func ptr[T any](v T) *T { return &v }

// TestParse is T1: options before and after the command, =, two values, --,
// and each usage error, by the option or value its message names.
func TestParse(t *testing.T) {
	tests := []struct {
		name string
		args string
		want invocation
	}{
		{"options before", "-C dir --ref r validate --json", invocation{command: "validate", dir: "dir", ref: "r", format: "json"}},
		{"options after", "validate --json --ref r -C dir", invocation{command: "validate", dir: "dir", ref: "r", format: "json"}},
		{"the mockups' form", "task e9c6 --ref 3cdae52", invocation{command: "task", view: tablo.Task, dir: ".", ref: "3cdae52", params: tablo.Params{Task: "e9c6"}}},
		{"between the words", "--ref r task --level glance e9c6 --yaml", invocation{command: "task", view: tablo.Task, dir: ".", ref: "r", format: "yaml", params: tablo.Params{Task: "e9c6", Level: "glance"}}},
		{"equals", "gates --ref=main --task=e9c6 --trunk=trunk --viewer=a@b.c", invocation{command: "gates", view: tablo.Gates, dir: ".", ref: "main", trunk: "trunk", viewer: "a@b.c", params: tablo.Params{Task: "e9c6", Viewer: "a@b.c"}}},
		{"two values", "queue --person a@b.c --brief c74a mockup", invocation{command: "queue", view: tablo.Queue, dir: ".", params: tablo.Params{Person: "a@b.c", Brief: &tablo.Brief{Task: "c74a", Gate: "mockup"}}}},
		{"the end of options", "-- validate", invocation{command: "validate", dir: "."}},
		{"words after the end of options", "validate -- extra-looking", invocation{}},
		{"a model with a hyphen", "trailer model -- -weird-id", invocation{command: "trailer", dir: ".", trailer: tablo.Trailer{Kind: "model", Model: "-weird-id"}}},
		{"replace twice", "validate --replace u=/a --replace v=/b", invocation{command: "validate", dir: ".", replace: [][2]string{{"u", "/a"}, {"v", "/b"}}}},
		{"replace with an equals in the dir", "validate --replace=u=/a=b", invocation{command: "validate", dir: ".", replace: [][2]string{{"u", "/a=b"}}}},
		{"trailer reviewed", "trailer reviewed 9f31 validate --no-check", invocation{command: "trailer", dir: ".", noCheck: true, trailer: tablo.Trailer{Kind: "reviewed", Task: "9f31", Gate: "validate"}}},
		{"trailer authorised", "--no-check trailer authorised 9f31", invocation{command: "trailer", dir: ".", noCheck: true, trailer: tablo.Trailer{Kind: "authorised", Task: "9f31"}}},
		{"version takes the common options", "version --ref x", invocation{command: "version", dir: ".", ref: "x"}},
		{"tableau window", "tableau --window 0 --historical", invocation{command: "tableau", view: tablo.Tableau, dir: ".", params: tablo.Params{Window: ptr(0), Historical: true}}},
		{"tableau columns", "tableau --columns mockup,design", invocation{command: "tableau", view: tablo.Tableau, dir: ".", params: tablo.Params{Columns: []string{"mockup", "design"}}}},
		{"context by task", "context --task 2034", invocation{command: "context", view: tablo.Context, dir: ".", params: tablo.Params{Task: "2034"}}},
		{"audit", "audit --stale 3 --now 2026-10-06 --role owner", invocation{command: "audit", view: tablo.Audit, dir: ".", params: tablo.Params{Stale: 3, Now: "2026-10-06", Role: "owner"}}},
		{"authority proposed", "authority --proposed", invocation{command: "authority", view: tablo.Authority, dir: ".", params: tablo.Params{Proposed: true}}},
		{"a range", "history --ref a1b2c3d..e4f5a6b", invocation{command: "history", view: tablo.History, dir: ".", ref: "e4f5a6b", params: tablo.Params{From: "a1b2c3d"}}},
		{"a range without its end", "history --ref a1b2c3d..", invocation{command: "history", view: tablo.History, dir: ".", ref: "HEAD", params: tablo.Params{From: "a1b2c3d"}}},
		{"a range without its start", "history --ref=..main", invocation{command: "history", view: tablo.History, dir: ".", ref: "main"}},
		{"a ref on history", "history --ref main", invocation{command: "history", view: tablo.History, dir: ".", ref: "main"}},
		{"help", "--help", invocation{help: true}},
		{"help after a command", "validate --help", invocation{help: true}},
		{"help beats a missing command's usage error", "--json --help", invocation{help: true}},
	}
	for _, tc := range tests {
		args := strings.Fields(tc.args)
		got, err := parse(args)
		if tc.name == "words after the end of options" {
			if err == nil || !strings.Contains(err.Error(), "extra-looking") {
				t.Errorf("%s: %q gives %v, want an error naming the extra word", tc.name, tc.args, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: parse(%q) = %v", tc.name, tc.args, err)
			continue
		}
		if !reflect.DeepEqual(*got, tc.want) {
			t.Errorf("%s: parse(%q)\n got %+v\nwant %+v", tc.name, tc.args, *got, tc.want)
		}
	}
}

// TestParseUsageErrors covers each rule's refusals: the error is a
// *tablo.UsageError, which Run turns into exit 2, and its message names the
// option or the value at fault.
func TestParseUsageErrors(t *testing.T) {
	tests := []struct {
		rule string
		args string
		want string
	}{
		{"1", "", "no command"},
		{"1", "--json", "no command"},
		{"1", "validate --nope", "--nope"},
		{"1", "frobnicate", "frobnicate"},
		{"1", "view tableau", "view"},
		{"1", "status", "status"},
		{"2", "validate --ref", "--ref"},
		{"2", "validate -C", "-C"},
		{"2", "validate --json=true", "--json"},
		{"2", "queue --brief c74a", "--brief"},
		{"2", "queue --brief=c74a --person a@b.c", "--brief"},
		{"2", "queue --person a@b.c --brief c74a", "--brief"},
		{"3", "validate -ref main", "--ref"},
		{"3", "validate -json", "--json"},
		{"3", "validate -x", "-x"},
		{"3", "validate -Cdir", "-Cdir"},
		{"4", "validate --ref a --ref b", "--ref"},
		{"4", "validate -C a -C b", "-C"},
		{"4", "validate --json --json", "--json"},
		{"4", "validate --replace u=/a --replace u=/b", "u"},
		{"5", "validate extra", "extra"},
		{"5", "version extra", "extra"},
		{"5", "gates extra", "extra"},
		{"5", "task", "task id"},
		{"5", "task e9c6 e9c7", "e9c7"},
		{"5", "trailer", "trailer"},
		{"5", "trailer bogus 9f31", "bogus"},
		{"5", "trailer authorised", "authorised"},
		{"5", "trailer authorised 9f31 extra", "extra"},
		{"5", "trailer reviewed 9f31", "reviewed"},
		{"5", "trailer reviewed 9f31 validate extra", "extra"},
		{"5", "trailer model", "model"},
		{"5", "trailer model a b", "b"},
		{"6", "validate --task e9c6", "--task"},
		{"6", "validate --level glance", "--level"},
		{"6", "version --person a@b.c", "--person"},
		{"6", "trailer authorised 9f31 --window 1", "--window"},
		{"6", "validate --no-check", "--no-check"},
		{"6", "gates --no-check", "--no-check"},
		{"6", "gates --person a@b.c", "--person"},
		{"6", "gates --window 1", "--window"},
		{"6", "gates --proposed", "--proposed"},
		{"6", "task e9c6 --task e9c6", "--task"},
		{"6", "tableau --task e9c6", "--task"},
		{"6", "assignment --historical", "--historical"},
		{"6", "queue --now 2026-10-06", "--now"},
		{"6", "authority --brief c74a mockup", "--brief"},
		{"6", "history --stale 3", "--stale"},
		{"6", "tableau --window 1 --columns mockup", "--columns"},
		{"6", "context --task 2034 --person a@b.c", "--task or --person"},
		{"6", "queue --brief c74a mockup --level glance", "--level"},
		{"6", "queue --brief c74a mockup --role owner", "--role"},
		{"6", "gates --ref a..b", "a..b"},
		{"6", "tableau --ref a..b", "range"},
		{"6", "validate --ref a..b", "range"},
		{"6", "history --ref a...b", "a...b"},
		{"6", "history --ref a..b..c", "a..b..c"},
		{"6", "history --ref -a..b", "-a"},
		{"6", "history --ref a..-b", "-b"},
		{"6", "validate --json --yaml", "--yaml"},
		{"6", "validate --yaml --json", "--json"},
		{"6", "gates --task E9C6", "E9C6"},
		{"6", "gates --task e9c", "e9c"},
		{"6", "task zzzz", "zzzz"},
		{"6", "queue --person ada", "ada"},
		{"6", "queue --person a@b@c", "a@b@c"},
		{"6", "gates --viewer ada", "--viewer"},
		{"6", "validate --viewer ada", "--viewer"},
		{"6", "validate --viewer=", "--viewer"},
		{"6", "tableau --columns Mockup", "Mockup"},
		{"6", "tableau --columns a,,b", "gate key"},
		{"6", "tableau --columns=", "gate key"},
		{"6", "queue --brief c74a Mock", "Mock"},
		{"6", "queue --brief xxxx mockup", "xxxx"},
		{"6", "audit --now 2026/10/06", "2026/10/06"},
		{"6", "audit --now 2026-13-01", "2026-13-01"},
		{"6", "tableau --window -1", "--window"},
		{"6", "tableau --window x", "x"},
		{"6", "audit --stale 0", "--stale"},
		{"6", "audit --stale -3", "--stale"},
		{"6", "audit --stale x", "x"},
		{"6", "gates --role boss", "boss"},
		{"6", "gates --level deep", "deep"},
		{"6", "validate --ref -x", "-x"},
		{"6", "validate --ref=", "--ref"},
		{"6", "validate --trunk -x", "-x"},
		{"6", "validate --replace nodir", "nodir"},
		{"6", "validate --replace =/a", "--replace"},
		{"6", "validate --replace u=", "--replace"},
		{"6", "validate -C ''", "-C"},
		{"6", "trailer authorised 9F31", "9F31"},
		{"6", "trailer reviewed 9f31 Validate", "Validate"},
	}
	for _, tc := range tests {
		args := strings.Fields(tc.args)
		for i, a := range args {
			if a == "''" {
				args[i] = ""
			}
		}
		got, err := parse(args)
		u, ok := err.(*tablo.UsageError)
		if !ok {
			t.Errorf("rule %s: parse(%q) = %+v, %v; want a *tablo.UsageError", tc.rule, tc.args, got, err)
			continue
		}
		if !strings.Contains(u.Msg, tc.want) {
			t.Errorf("rule %s: parse(%q) message %q does not name %q", tc.rule, tc.args, u.Msg, tc.want)
		}
	}
}

// TestParseModelIdentifier checks the one word Fields cannot spell.
func TestParseModelIdentifier(t *testing.T) {
	_, err := parse([]string{"trailer", "model", "a\u00a0b"})
	if u, ok := err.(*tablo.UsageError); !ok || !strings.Contains(u.Msg, "printable") {
		t.Errorf("a model identifier with a no-break space gives %v", err)
	}
}

// TestMockupLines is T2: every line of mockup-lines.txt parses, is a view and
// passes Params.Check.
func TestMockupLines(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "mockup-lines.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 68 {
		t.Errorf("mockup-lines.txt holds %d lines, want 68", len(lines))
	}
	seen := map[tablo.View]bool{}
	for _, line := range lines {
		words := strings.Fields(line)
		if len(words) < 2 || words[0] != "tablo" {
			t.Errorf("line %q does not start with tablo and a command", line)
			continue
		}
		inv, err := parse(words[1:])
		if err != nil {
			t.Errorf("%q: %v", line, err)
			continue
		}
		if inv.view == "" {
			t.Errorf("%q is no view command", line)
			continue
		}
		if err := inv.params.Check(inv.view); err != nil {
			t.Errorf("%q: Check: %v", line, err)
		}
		seen[inv.view] = true
	}
	for _, v := range viewCommands {
		if !seen[v] {
			t.Errorf("no mockup line runs %s", v)
		}
	}
}

// TestUsageText checks that usage.txt names every command and exit code.
func TestUsageText(t *testing.T) {
	for _, v := range viewCommands {
		if !strings.Contains(usage, "\n  "+string(v)+" ") {
			t.Errorf("usage.txt lacks the view %s", v)
		}
	}
	for _, want := range []string{"validate", "trailer authorised <id>", "trailer reviewed <id> <gate>", "trailer reaffirmed <id>", "trailer model <identifier>", "version", "--person", "--ref", "--trunk", "--replace", "--viewer", "--json | --yaml", "--help", "--no-check"} {
		if !strings.Contains(usage, want) {
			t.Errorf("usage.txt lacks %q", want)
		}
	}
	for _, old := range []string{"--for", "--at ", "--historical-junctions", "view <name>"} {
		if strings.Contains(usage, old) {
			t.Errorf("usage.txt spells the older form %q", old)
		}
	}
	for code := '0'; code <= '3'; code++ {
		if !strings.Contains(usage, "\n  "+string(code)+"  ") {
			t.Errorf("usage.txt lacks exit code %c", code)
		}
	}
}
