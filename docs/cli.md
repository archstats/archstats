# Commands

Every command reads the folder named with `-f` (or the current folder) and runs a fresh scan of it.

## Write a snapshot

```shell
archstats -f path/to/project export sqlite snapshot.db
```

The file holds every table in [Snapshot tables](snapshot.md). Writing to an existing file adds a second report beside the first, told apart by `report_id`.

## Look at one table

```shell
archstats -f path/to/project view components \
  -c name,modularity__coupling__afferent,modularity__coupling__efferent,modularity__instability \
  --sorted-by modularity__instability
```

| Flag | Does |
|---|---|
| `-c`, `--column` | Only these columns, comma-separated. |
| `--sorted-by` | Sort by a column; numbers sort high to low. |
| `-o`, `--output-format` | `table` (the default), `csv`, `tsv` or `json`. |
| `--no-header` | Leave the header row out of `table`, `csv` and `tsv`. |

Any table in [Snapshot tables](snapshot.md) can be viewed by its name.

## Check rules

```shell
archstats -f path/to/project assert -r rules.yml
```

Runs SQL rules against a scan and fails when one is broken. See [Rules](rules.md).

## Look up a metric

```shell
archstats definitions                          # every metric, one line each
archstats definitions modularity__instability  # one metric in full
```

[Metrics](metrics.md) has the same text.

## What a scan reads

Archstats switches on what the folder needs by looking at its files: a language pack for each language it finds, and git history when the folder is a repository or holds several. Name extensions with `-e` to choose them yourself; naming any turns that discovery off.

| Flag | Does |
|---|---|
| `-f`, `--working-dir` | The folder to read. |
| `-e`, `--extension` | Extensions to run, repeatable. |
| `--ignore` | Paths to leave out, in `.gitignore` syntax, repeatable. `.gitignore` and `.archstatsignore` files in the tree already count. |
| `--component-strategy` | How files become components: `fallback` (declared where the language declares them, else the directory; the default), `declared`, or `directory`. |
| `--git-since`, `--git-after` | Passed to `git log`, to read part of the history. |
| `--git-based-on` | What "last 90 days" counts back from: `head` (the newest commit scanned, the default, so a commit reads the same on any day) or `now`. |
| `--git-max-changes-per-commit` | Leave commits that touch more files than this out of co-change (default 100; 0 keeps all). |
| `--file-matrix` | Also write `file_matrix`, every pair of files that changed together. Large. |
| `-v`, `--verbose` | Log what the scan does. |

`archstats --help` and `archstats <command> --help` list every flag.

## Use it in CI

A GitHub Actions job that holds a repository to its rules:

```yaml
jobs:
  architecture:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # git history, for the co-change and ownership tables
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - run: go install github.com/archstats/archstats@latest
      - run: archstats -f . assert -r architecture/rules.yml
```

`go install` builds with cgo, which the hosted runners have. To skip the build, download a release archive instead.
