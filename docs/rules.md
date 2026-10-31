# Rules

A rule is a SQL query against the snapshot that lists what breaks it, and the number of rows you expect. `archstats assert` scans the folder, runs every rule and fails when one returns a different count, so a CI job goes red when the architecture drifts.

## Write a rules file

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

| Field | Holds |
|---|---|
| `name` | What the rule says, as a sentence. It is printed with the result. |
| `description` | Optional: why the rule exists. |
| `query` | SQLite SQL against the [snapshot tables](snapshot.md). Each row is one thing that breaks the rule; name the columns that help someone find it, such as `file` and `line`. |
| `expect` | How many rows the query should return. Usually `0`. |

## Run it

```shell
archstats -f path/to/project assert -r rules.yml
```

Every rule prints ✅ or ❌. A broken rule prints the rows it found. The command exits non-zero when any rule is broken or its SQL fails, so CI fails with it ([example job](cli.md#use-it-in-ci)).

Write a query against a snapshot first, in the desktop app's SQL console or with `sqlite3`, and move it into the rules file once it returns what you mean.

## Templates

- [CI/CD rules](rules/cicd.yml): images built by a pipeline, base images that name a version, pipelines read in full, and the other hygiene checks the CI/CD reading records. Copy the ones your team wants to hold itself to.

## Rules Archstats checks itself

The `rules` table holds the engine's own architecture rules, such as "core must not depend on a plugin" for .NET plugin architectures, each with a verdict: `violation`, `ok` or `not_applicable`. They are listed under *Architecture Rules* in [Metrics](metrics.md). Hold a codebase to them like any other table:

```yaml
assertions:
  - name: "No built-in architecture rule is broken"
    query: SELECT rule, "from", "to", file, line FROM rules WHERE status = 'violation'
    expect: 0
```
