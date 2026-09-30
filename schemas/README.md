# Schemas

This directory holds a copy of the JSON Schema files from the
[tableaux](https://github.com/nbyoung/tableaux) repository, one per Tableaux
file kind: `version`, `gates`, `task`, `status` and `history`, each named
`<kind>.schema.yaml` after its `$id`. `embed.go` embeds them into the binary,
so a user installs nothing beside `tablo`.

The files come from the tableaux schema-files task (`e3cb`), which extracts
them from SYNTAX.md and tags them with a language version. The copy here
matches the tableaux commit below.

To copy them, record the tableaux commit here and refresh the files:

| Field            | Value       |
|------------------|-------------|
| tableaux commit  | `f3e3e2e`   |
| tableaux version | `0.1.0`     |

The tableaux version copied here must be one the module accepts; see
`Accepts` in the root package.
