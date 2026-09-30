# Validator prototype (8118)

## The question

Can the validator run from the embedded schemas alone, and do the structural
rules of README.md run as code and emit diagnostics with positions and
severities that agree with the corpus? The riskiest parts are the schema step
(no YAML or JSON Schema module is available) and the positions (a schema
failure must point at a line and column).

## Run

```
export PATH=$PATH:/usr/local/go/bin
go run ./prototype/8118 /home/nbyoung/Projects/Tableaux/tableaux/corpus/build/requires-cycle
go test ./prototype/8118 -v            # skips the corpus tests when it is absent
```

The command reads `.tableaux` in a working tree, prints
`file:line:col: severity: RULE message` and exits 1 on any error.

## What it shows

- `yaml.go` reads the YAML subset the files use (block and flow collections,
  plain, quoted and folded scalars, comments) and keeps a line and column on
  every node and key. It parses all five embedded schemas, every corpus
  project file and every `expected.yaml` findings list.
- `schema.go` validates a parsed file against `schemas.FS` (the embedded
  files) for the keywords those schemas use: type, const, pattern, minLength,
  minimum, minItems, format (email), required, dependentRequired, properties,
  additionalProperties, propertyNames, items, prefixItems, contains, allOf,
  oneOf, not, if/then, $ref.
- `rules.go` names each schema failure by its RULES.md id (a path and keyword
  table; junction and status failures read the offending entry) and runs
  T8, T9, T10, R1 to R5, J1, J3 and T7 as code over the loaded tasks.

Against the built corpus (`TestCorpusSample`): 32 entries (31 invalid and one warning-only), a few per
rule family, each report exactly the rules, severities, files, tasks and
gates that its `expected.yaml` states, and no other. `weather-station`
reports nothing.

```
$ go run ./prototype/8118 corpus/build/requires-cycle
tasks/b2c9.yaml:8:11: error: R2 requirement on c3d7 closes a cycle
tasks/c3d7.yaml:6:11: error: R2 requirement on b2c9 closes a cycle
$ go run ./prototype/8118 corpus/build/tree-parent-cycle
tasks/c3d7.yaml:5:15: error: T10 parent chain loops through c3d7 and never reaches the root
tasks/d4e8.yaml:5:15: error: T10 parent chain loops through d4e8 and never reaches the root
$ go run ./prototype/8118 corpus/build/recursive-on-parent
tasks/e4a1.yaml:8:3: error: J3 a parent states a recursive junction
$ go run ./prototype/8118 corpus/build/unquoted-id
tasks/1a00.yaml:5:15: warning: T7 id 1000 is not a quoted string
$ go run ./prototype/8118 corpus/build/weather-station
$
```

`TestCorpusCoverage` compares all 83 built entries: 48 agree, 35 differ, and
every difference is a finding the prototype does not yet raise. It raises no
finding that an `expected.yaml` lacks.

## What it leaves out

- The loader: it reads a working tree, not a Git ref (the loader task).
- Rules that need more than the files: R9, S11, P5, J8, J9, J13, H1 to H3,
  and the derived facts. `weather-station` expects the R9 warning, which needs
  status and derivation, so the prototype's silence there is the structural
  result only.
- Rules the prototype has no code for yet: P2, P4, T12, R6, R7, R10, J12, G6,
  G9, G10 (uniqueness), G12 beyond the schema, S1, S2, S4 to S10, S12.
- YAML beyond the subset: anchors, tags, multiple documents, duplicate keys,
  multi-line plain scalars. A real reader needs `gopkg.in/yaml.v3` (and a
  JSON Schema module such as `github.com/santhosh-tekuri/jsonschema/v6`
  or this file's small validator) as the implementation dependency; this
  prototype replaces both with about 900 lines of Go (1,300 with tests), more than a prototype should hold.
- A schema failure that cascades (a missing status `gate` also fails the
  undefined rule) is filtered by hand for that one case.

## What the design gate must decide

- Whether the module takes the YAML and JSON Schema dependencies or keeps a
  reader of its own. The embedded schemas run unchanged in either.
- How a schema failure maps to a rule id. The prototype uses a path and
  keyword table plus a look at the entry; the design could instead tag rules
  in the schemas, or check the rules in code and use the schema only as a
  gate.
- Whether the id is a string only when quoted. YAML reads an unquoted `1000`
  as an integer; the prototype warns (T7) and reads it as a string, as
  corpus/check.py does.
- Where a project-level finding (T8) sits when it has no file: the prototype
  names the second root's file, or `tasks` when there is no root.
