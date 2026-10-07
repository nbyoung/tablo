# From a schema leaf to a rule

`validate.Files` validates `File.Root.Plain()` of each file that gave a model against its embedded schema, with `github.com/santhosh-tekuri/jsonschema/v6` and `AssertFormat` on. A **leaf** is a `*jsonschema.ValidationError` with no `Causes`, or one whose kind is `*kind.PropertyNames`; the walk stops there. A leaf has an instance location, the path of the value in the file, and a keyword location, `SchemaURL` after `#` followed by `ErrorKind.KeywordPath()`. The tables map each leaf to one rule. They are total: the last row of each file kind takes every leaf no row above takes.

## Three leaves the tables never see

| Leaf | Why | Who reports the fault |
|------|-----|-----------------------|
| Kind `*kind.PropertyNames` | The schema reports a junction key with no position | J1, J2 and J11 read the keys ([`rules.md`](rules.md)) |
| Keyword location holds `/contains` | One absent state gives a leaf for every state in the list | G12 reads the states |
| Instance location `junctions/<gate>/…` in the run over a whole task file | `oneOf` gives the leaves of all three kinds | The entry validates alone against `task.schema.yaml#/$defs/plain`, `#/$defs/recursive` or `#/$defs/not_applicable`, as `Junction.Kind()` selects, and its leaves take the `junctions` rows below with the prefix `junctions/<gate>` |

A `Mixed` entry and an entry that is no mapping take no schema run: both are J4. The schema rejects each, since no one kind admits the fields of two.

## Position and message, by the kind of the leaf

`node` is the deepest value that exists along the instance location; `where` is the location's segments joined by `.`, or `the file` at the root. `value` is the text of a scalar node. The Validator writes every message itself.

| Kind | Position | Message |
|------|----------|---------|
| `*kind.AdditionalProperties` | One diagnostic for each name in `Properties`, names sorted, at the `KeyPos` of that field in `node` | `% is no field of %`: the name, `where` |
| `*kind.Required` | `node`, the mapping that lacks the field | `% lacks %`: `where`, the names of `Missing` sorted and joined by `, ` |
| `*kind.DependentRequired` | The `model` field of `node` | `% states model and no contributor` |
| Keyword location holds `/items/oneOf` | `node`, the requirement | R8: `% states neither id nor subproject`; R11: `% states both id and subproject` |
| `*kind.Type` | `node` | `% is a %, not a %`: `where`, `node.Kind`, the names of `Want` joined by ` or `; `an` before a vowel |
| `*kind.Pattern` | `node` | `% % does not match %`: `where`, `value`, `Want` |
| `*kind.Format` | `node` | `% % is no email address`, or `… is no URI reference` |
| `*kind.MinLength` | `node` | `% is empty` |
| `*kind.Minimum` | `node` | `% is %, below %` |
| `*kind.MinItems` | `node` | `% holds %, fewer than %` |
| `*kind.Const` | `node` | `% is %, not %`: `where`, `value`, `Want` |
| Any other | `node` | `% does not match its schema` |

`Files` sorts its diagnostics and drops each one equal in every field to the one before it, so the two leaves of a `oneOf` give one R8.

## `version.yaml`

| Instance location | Keyword | Rule | Gate |
|-------------------|---------|------|------|
| any | any | P3 | |

## `gates.yaml`

| Instance location | Keyword | Rule | Gate |
|-------------------|---------|------|------|
| root | `additionalProperties` | G11 | |
| root | `required`, missing `states` alone | G7 | |
| root | any other: `type`, `required` | G3 | |
| `gates` | any: `type`, `minItems` | G3 | |
| `gates/0/key` | holds `/prefixItems/` | G2 | |
| `gates/<i>/key` | any other | G5 | The item's `key`, when a scalar |
| `gates/<i>`, `gates/<i>/<field>` | any | G4 | The item's `key`, when a scalar |
| `states/<i>/key` | any | G5 | |
| `states/<i>/severity` | any | G8 | |
| `states`, `states/<i>`, `states/<i>/<field>` | any | G7 | |
| `reasons/<i>/key` | any | G5 | |
| `reasons`, `reasons/<i>`, `reasons/<i>/<field>` | any | G10 | |
| any other | any | G11 | |

## `tasks/<id>.yaml`

| Instance location | Keyword | Rule | Gate |
|-------------------|---------|------|------|
| root | `additionalProperties` | T3 | |
| root | any other: `type`, `required` | T2 | |
| `title`, `description` | any | T2 | |
| `assignee` | holds `/format` | T4 | |
| `assignee` | any other | T2 | |
| `references`, `references/…` | any | T5 | |
| `parent/id` | any | T6 | |
| `parent`, `parent/order` | any | T11 | |
| `requires` | any | R8 | |
| `requires/<i>` | holds `/items/oneOf` | R11 when the entry states `subproject`, else R8 | |
| `requires/<i>/id` | any | T6 | |
| `requires/<i>/from` | any | R6 | The value, when a scalar |
| `requires/<i>/to` | any | R7 | The value, when a scalar |
| `requires/<i>/subproject/id` | any | T6 | |
| `requires/<i>/subproject/commit` | any | J14 | |
| `requires/<i>/subproject`, `…/url` | any | R11 | |
| `requires/<i>`, `requires/<i>/text` | any other | R8 | |
| `junctions` | any: `type` | J4 | |
| `junctions/<gate>` | holds `/dependentRequired` | J5 | The key |
| `junctions/<gate>` | holds `/$defs/plain/` | J10 | The key |
| `junctions/<gate>` | holds `/$defs/recursive/`, kind `*kind.Required` | J7 | The key |
| `junctions/<gate>` | holds `/$defs/recursive/`, any other | J4 | The key |
| `junctions/<gate>` | any other: the not-applicable schema | J6 | The key |
| `junctions/<gate>/contributor`, `…/reviewer` | any | T4 | The key |
| `junctions/<gate>/references`, `…/references/…` | any | T5 | The key |
| `junctions/<gate>/model` | any | J10 | The key |
| `junctions/<gate>/applies` | any | J6 | The key |
| `junctions/<gate>/subproject/id` | any | T6 | The key |
| `junctions/<gate>/subproject/commit` | any | J14 | The key |
| `junctions/<gate>/subproject`, `…/url` | any | J7 | The key |
| any other | any | T3 | |

## `status/<id>.yaml`

| Instance location | Keyword | Rule | Gate |
|-------------------|---------|------|------|
| any | holds `/then/`, and the file states no scalar `gate` | none: S3 or S5 reports the gate | |
| any | holds `/then/` | S4 | |
| `gate` | any | S5 | The value, when a scalar |
| `state`, `reason` | any | S6 | |
| any other: root, `note` | any | S3 | |

## What the Loader's typed build guarantees

The Loader hands the schema a tree in which every id at the four places is a string, a repeated key appears once, a second document is gone, an alias is null and a tag is dropped. So the schema never reports the type of an unquoted id, and the Validator never checks for a repeated key, a second document, an anchor or a tag: L2 and L3 own those. The typed fields are lenient where the schema is strict, so the Validator takes the kind of a value from the schema leaf and never from `StrField.V`: a rule in code tests `Node.Scalar()` before it reads a field, and stays silent where the schema row already reports the kind.
