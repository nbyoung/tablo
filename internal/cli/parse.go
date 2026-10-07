// Package cli is the tablo command: the argument parser, the plain forms and
// the usage text, over the root package that does the work.
package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nbyoung/tablo"
)

// An invocation is one parsed command line.
type invocation struct {
	help    bool
	command string      // the command word; empty only with help
	view    tablo.View  // the command, when it is a view
	dir     string      // -C, the directory the search for .tableaux starts from
	ref     string      // the commit-ish; for a range, its <to> side; empty reads the working tree
	trunk   string      // --trunk
	replace [][2]string // --replace, each as {url, dir}, in the order given
	viewer  string      // --viewer
	format  string      // "", "json" or "yaml"
	params  tablo.Params
	trailer tablo.Trailer
	noCheck bool // trailer --no-check
}

// arity is the number of values each option takes. Every option but -C stands
// with two hyphens.
var arity = map[string]int{
	"-C": 1, "--ref": 1, "--trunk": 1, "--replace": 1, "--viewer": 1,
	"--json": 0, "--yaml": 0, "--help": 0,
	"--role": 1, "--level": 1, "--task": 1, "--person": 1, "--proposed": 0,
	"--brief": 2, "--window": 1, "--columns": 1, "--historical": 0,
	"--stale": 1, "--now": 1, "--no-check": 0,
}

// viewOptions are the options only a view takes, whichever views those are.
var viewOptions = map[string]bool{
	"--role": true, "--level": true, "--task": true, "--person": true,
	"--proposed": true, "--brief": true, "--window": true, "--columns": true,
	"--historical": true, "--stale": true, "--now": true,
}

// viewCommands are the ten views, named as the mockups are.
var viewCommands = []tablo.View{
	tablo.Gates, tablo.Task, tablo.Authority, tablo.Assignment, tablo.Queue,
	tablo.Blockage, tablo.Tableau, tablo.Context, tablo.History, tablo.Audit,
}

// trailerWords is the number of words each trailer kind takes after its name.
var trailerWords = map[string]int{"authorised": 1, "reviewed": 2, "reaffirmed": 1, "model": 1}

// An option is one option as written, with its values.
type option struct {
	name string
	vals []string
}

func usageErr(format string, a ...any) error {
	return &tablo.UsageError{Msg: fmt.Sprintf(format, a...)}
}

// parse reads one command line, without the program name, by the six rules of
// design 4ed9. It returns a *tablo.UsageError for a line that is wrong. It
// reads no environment, so the rule that queue and context need a person or a
// viewer waits for the resolution of defaults.
func parse(args []string) (*invocation, error) {
	opts, words, err := scan(args)
	if err != nil {
		return nil, err
	}
	for _, o := range opts {
		if o.name == "--help" {
			return &invocation{help: true}, nil
		}
	}
	if len(words) == 0 {
		return nil, usageErr("no command given")
	}
	inv := &invocation{command: words[0], dir: "."}
	words = words[1:]
	for _, v := range viewCommands {
		if string(v) == inv.command {
			inv.view = v
		}
	}
	if err := inv.takeWords(words); err != nil {
		return nil, err
	}
	for _, o := range opts {
		if err := inv.apply(o); err != nil {
			return nil, err
		}
	}
	if inv.view != "" {
		inv.params.Viewer = inv.viewer
		if err := inv.params.Check(inv.view); err != nil {
			return nil, err
		}
	}
	return inv, nil
}

// scan splits the arguments into options and words by rules 1 to 4.
func scan(args []string) (opts []option, words []string, err error) {
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			words = append(words, args[i+1:]...)
			return opts, words, nil
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a, "=")
			n, ok := arity[name]
			if !ok {
				return nil, nil, usageErr("unknown option %s", name)
			}
			o := option{name: name}
			switch {
			case hasVal && n != 1:
				if n == 0 {
					return nil, nil, usageErr("%s takes no value", name)
				}
				return nil, nil, usageErr("%s takes %d values, which follow it as separate arguments", name, n)
			case hasVal:
				o.vals = []string{val}
			default:
				if i+n >= len(args) {
					return nil, nil, usageErr("%s needs %s", name, valuesWord(n))
				}
				o.vals = append(o.vals, args[i+1:i+1+n]...)
				i += n
			}
			if seen[name] && name != "--replace" {
				return nil, nil, usageErr("%s is given twice", name)
			}
			seen[name] = true
			opts = append(opts, o)
		case a == "-C":
			if i+1 >= len(args) {
				return nil, nil, usageErr("-C needs a directory")
			}
			if seen["-C"] {
				return nil, nil, usageErr("-C is given twice")
			}
			seen["-C"] = true
			opts = append(opts, option{name: "-C", vals: []string{args[i+1]}})
			i++
		case strings.HasPrefix(a, "-") && len(a) > 1:
			name, _, _ := strings.Cut(a[1:], "=")
			if _, ok := arity["--"+name]; ok && len(name) > 1 {
				return nil, nil, usageErr("%s is not an option; write --%s", a, a[1:])
			}
			return nil, nil, usageErr("unknown option %s", a)
		default:
			words = append(words, a)
		}
	}
	return opts, words, nil
}

func valuesWord(n int) string {
	if n == 1 {
		return "a value"
	}
	return strconv.Itoa(n) + " values"
}

// takeWords reads the words after the command by rule 5.
func (inv *invocation) takeWords(words []string) error {
	switch {
	case inv.command == "task":
		if len(words) == 0 {
			return usageErr("task needs a task id")
		}
		if len(words) > 1 {
			return usageErr("unexpected argument %q after the task id", words[1])
		}
		inv.params.Task = words[0]
	case inv.command == "trailer":
		if len(words) == 0 {
			return usageErr("trailer needs authorised, reviewed, reaffirmed or model")
		}
		kind := words[0]
		n, ok := trailerWords[kind]
		if !ok {
			return usageErr("%q is not a trailer kind: authorised, reviewed, reaffirmed or model", kind)
		}
		rest := words[1:]
		if len(rest) < n {
			return usageErr("trailer %s needs %d argument(s), not %d", kind, n, len(rest))
		}
		if len(rest) > n {
			return usageErr("unexpected argument %q after trailer %s", rest[n], kind)
		}
		inv.trailer.Kind = kind
		switch kind {
		case "model":
			inv.trailer.Model = rest[0]
		case "reviewed":
			inv.trailer.Task, inv.trailer.Gate = rest[0], rest[1]
		default:
			inv.trailer.Task = rest[0]
		}
		if _, err := inv.trailer.Line(); err != nil {
			return err
		}
	case inv.view != "" || inv.command == "validate" || inv.command == "version":
		if len(words) > 0 {
			return usageErr("unexpected argument %q after %s", words[0], inv.command)
		}
	default:
		return usageErr("unknown command %q", inv.command)
	}
	return nil
}

// apply stores one option and checks its value by rule 6.
func (inv *invocation) apply(o option) error {
	if viewOptions[o.name] && inv.view == "" {
		return usageErr("%s is not taken by %s", o.name, inv.command)
	}
	if o.name == "--no-check" && inv.command != "trailer" {
		return usageErr("--no-check is not taken by %s", inv.command)
	}
	if o.name == "--task" && inv.view == tablo.Task {
		return usageErr("--task is not taken by task, which takes the id as its argument")
	}
	v := o.vals
	p := &inv.params
	switch o.name {
	case "-C":
		if v[0] == "" {
			return usageErr("-C needs a directory")
		}
		inv.dir = v[0]
	case "--ref":
		return inv.setRef(v[0])
	case "--trunk":
		if v[0] == "" || strings.HasPrefix(v[0], "-") {
			return usageErr("--trunk %q is not a branch name", v[0])
		}
		inv.trunk = v[0]
	case "--replace":
		url, dir, ok := strings.Cut(v[0], "=")
		if !ok || url == "" || dir == "" {
			return usageErr("--replace %q is not of the form <url>=<dir>", v[0])
		}
		for _, r := range inv.replace {
			if r[0] == url {
				return usageErr("--replace names %q twice", url)
			}
		}
		inv.replace = append(inv.replace, [2]string{url, dir})
	case "--viewer":
		if err := (tablo.Params{Viewer: v[0]}).Check(tablo.Gates); err != nil || v[0] == "" {
			return usageErr("--viewer %q is not an email address", v[0])
		}
		inv.viewer = v[0]
	case "--json", "--yaml":
		f := o.name[2:]
		if inv.format != "" && inv.format != f {
			return usageErr("--json and --yaml exclude each other")
		}
		inv.format = f
	case "--no-check":
		inv.noCheck = true
	case "--role":
		p.Role = tablo.Role(v[0])
	case "--level":
		p.Level = tablo.Level(v[0])
	case "--task":
		p.Task = v[0]
	case "--person":
		p.Person = v[0]
	case "--proposed":
		p.Proposed = true
	case "--historical":
		p.Historical = true
	case "--brief":
		p.Brief = &tablo.Brief{Task: v[0], Gate: v[1]}
	case "--columns":
		p.Columns = strings.Split(v[0], ",")
	case "--window":
		n, err := strconv.Atoi(v[0])
		if err != nil {
			return usageErr("--window %q is not a number", v[0])
		}
		p.Window = &n
	case "--stale":
		n, err := strconv.Atoi(v[0])
		if err != nil {
			return usageErr("--stale %q is not a number", v[0])
		}
		if n < 1 {
			return usageErr("--stale %d is below one", n)
		}
		p.Stale = n
	case "--now":
		p.Now = v[0]
	}
	return nil
}

// setRef reads --ref: a commit-ish, or on history a range <from>..<to>. An
// empty <to> reads HEAD and an empty <from> reads the whole history.
func (inv *invocation) setRef(ref string) error {
	if ref == "" {
		return usageErr("--ref needs a ref")
	}
	from, to, isRange := strings.Cut(ref, "..")
	if !isRange {
		if strings.HasPrefix(ref, "-") {
			return usageErr("--ref %q starts with a hyphen", ref)
		}
		inv.ref = ref
		return nil
	}
	if strings.HasPrefix(to, ".") || strings.Contains(to, "..") {
		return usageErr("--ref %q is a three-dot range; write <from>..<to>", ref)
	}
	if inv.command != "history" {
		return usageErr("--ref %q is a range, which only history takes", ref)
	}
	if strings.HasPrefix(from, "-") || strings.HasPrefix(to, "-") {
		return usageErr("--ref %q holds a ref that starts with a hyphen", ref)
	}
	if to == "" {
		to = "HEAD"
	}
	inv.params.From, inv.ref = from, to
	return nil
}
