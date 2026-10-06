# Conformance report

tablo 0.1.0 reads tableaux 0.3. Corpus: testdata/corpus at tableaux ccb3947.

3 entries: 1 pass, 1 fail, 0 known, 1 skipped.

## Rules

4 rules: 1 confirmed, 2 failing, 1 not run, 0 without an entry.

| Rule | Severity | Stated | Matched | Missing | Unexpected | Result    | Entries           |
|------|----------|--------|---------|---------|------------|-----------|-------------------|
| T8   | error    | 1      | 1       | 0       | 0          | confirmed | `tree-two-roots`  |
| R9   | warning  | 1      | 0       | 1       | 0          | failing   | `weather-station` |
| S11  | error    | 0      | 0       | 0       | 1          | failing   |                   |
| H1   | warning  | 0      | 0       | 0       | 0          | not run   | `unknown-trailer` |

Findings outside the rules: 0.

## Facts

| Fact          | Stated | Agree | Differ | Undecided |
|---------------|--------|-------|--------|-----------|
| valid         | 2      | 2     | 0      | 0         |
| findings      | 2      | 1     | 1      | 0         |
| project       | 9      | 9     | 0      | 0         |
| parent        | 6      | 6     | 0      | 0         |
| authorities   | 6      | 6     | 0      | 0         |
| authorisation | 18     | 18    | 0      | 0         |
| junctions     | 31     | 30    | 0      | 1         |
| applicable    | 2      | 2     | 0      | 0         |
| requires      | 3      | 3     | 0      | 0         |
| reviews       | 2      | 2     | 0      | 0         |
| subproject    | 3      | 3     | 0      | 0         |
| status        | 33     | 32    | 1      | 0         |
| events        | 10     | 9     | 1      | 0         |
| refs          | 6      | 6     | 0      | 0         |
| total         | 133    | 129   | 3      | 1         |

## Entries that do not pass

| Entry             | Result  | Reason                                                        |
|-------------------|---------|---------------------------------------------------------------|
| `unknown-trailer` | skipped | language 1.0.0: tablo reads major 0                           |
| `weather-station` | fail    | tasks.4e2b.status.gate: want function, got design; and 3 more |
