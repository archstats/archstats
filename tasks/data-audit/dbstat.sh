#!/bin/sh
# Table sizes of one or more snapshots, largest first: where the bytes go.
# Usage: tasks/data-audit/dbstat.sh snapshot.db [more.db ...]
for db in "$@"; do
  printf '== %s  (%s bytes)\n' "$db" "$(wc -c < "$db" | tr -d ' ')"
  sqlite3 "$db" "SELECT name, round(sum(pgsize) / 1048576.0, 1) AS mb FROM dbstat GROUP BY name ORDER BY 2 DESC LIMIT 15"
done
