# The CLI

The `archstats` command does what the desktop app's scan does, in a terminal or a CI job: it reads a folder, writes the snapshot, prints any table, and fails a build when a rule you wrote in SQL breaks. This page goes from installing it to running it in CI.

## Install {#install}

Download the archive for your system from the [latest release](https://github.com/archstats/archstats/releases/latest): Linux (amd64, arm64), macOS (arm64, amd64) or Windows (amd64). Unpack it and put `archstats` on your `PATH`.

On macOS, a binary downloaded with a browser is quarantined. Clear that once, then check it runs:

```shell
xattr -d com.apple.quarantine ./archstats
archstats --version
```

To build it from source, you need Go 1.23 or later and a C compiler:

```shell
git clone https://github.com/archstats/archstats
cd archstats
go generate ./extensions/treesitter/swift/grammar
go install .
```

The `go generate` step downloads the Swift grammar's generated C, which the repository does not carry. That is also why `go install github.com/archstats/archstats@latest` does not work.

## Write a snapshot {#write-a-snapshot}

Every command reads the folder named with `-f` (or the current folder) and scans it fresh.

```shell
archstats -f path/to/project export sqlite snapshot.db
```

The file is the same snapshot the desktop app writes: open it in the app with **Import Snapshot**, with `sqlite3`, or with any SQL tool. [Snapshot tables](snapshot.md) describes every table. Writing to an existing file adds a second report beside the first, told apart by `report_id`.

## Look at one table {#look-at-one-table}

Print any snapshot table without writing a file:

```shell
archstats -f path/to/project view components \
  -c name,modularity__coupling__afferent,modularity__instability \
  --sorted-by modularity__instability
```

| Flag | Does |
|---|---|
| `-c`, `--column` | Only these columns, comma-separated. |
| `--sorted-by` | Sort by a column; numbers sort high to low. |
| `-o`, `--output-format` | `table` (the default), `csv`, `tsv` or `json`. |
| `--no-header` | Leave the header row out of `table`, `csv` and `tsv`. |

To read what a metric means, ask for its definition. [Metrics](metrics.md) has the same text.

```shell
archstats definitions                          # every metric, one line each
archstats definitions modularity__instability  # one metric in full
```

## Hold the code to rules {#rules}

A rule is a SQL query that lists what breaks it, and the number of rows you expect, usually none. `archstats assert` scans the folder, runs every rule, and exits non-zero when one returns a different count, so a CI job goes red when the architecture drifts.

```yaml
assertions:
  - name: "The domain does not depend on the web layer"
    description: >
      Domain code is reused by the batch jobs, which have no web layer.
    query: |
      SELECT "from", "to", file FROM component_connections_direct
      WHERE "from" LIKE 'shop/domain%' AND "to" LIKE 'shop/web%'
    expect: 0
```

- `name` says what the rule holds, as a sentence; it is printed with the result.
- `description` is optional: why the rule exists.
- `query` is SQLite SQL against the [snapshot tables](snapshot.md). Each row is one thing that breaks the rule, so name the columns that help someone find it, such as `file` and `line`.
- `expect` is how many rows the query should return.

```shell
archstats -f path/to/project assert -r rules.yml
```

Every rule prints ✅ or ❌, and a broken rule prints the rows it found. Write the query against a snapshot first, in the desktop app's SQL console or with `sqlite3`, and move it into the rules file once it returns what you mean.

Archstats also checks some architecture rules of its own, such as "a .NET core must not depend on a plugin". They are in the `rules` table, each with a verdict; hold a codebase to them like any other table:

```yaml
assertions:
  - name: "No built-in architecture rule is broken"
    query: SELECT rule, "from", "to", file, line FROM rules WHERE status = 'violation'
    expect: 0
```

For pipelines and Dockerfiles, [CI/CD rules](rules/cicd.yml) is a ready-made file: images built by a pipeline, base images that name a version, and other hygiene checks. Copy the ones your team wants to hold itself to.

## Run it in CI {#ci}

A GitHub Actions job that holds a repository to its rules:

```yaml
jobs:
  architecture:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # git history, for the co-change and ownership tables
      - run: |
          gh release download --repo archstats/archstats --pattern '*-linux-amd64.tar.gz'
          tar -xzf archstats-*-linux-amd64.tar.gz
        env:
          GH_TOKEN: ${{ github.token }}
      - run: ./archstats -f . assert -r architecture/rules.yml
```

It downloads the latest release for Linux on x86-64. Pin a version with `gh release download vX.Y.Z …`.

## What a scan reads {#scan-flags}

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
