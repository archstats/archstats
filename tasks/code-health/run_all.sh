#!/bin/sh
# Rebuilds the whole experiment: every repo's T0 measurements, future and
# past changes, and the function-level attribution of each later edit.
#
#   tasks/code-health/run_all.sh <work-dir> <repo>[:years] ...
set -e
HERE=$(cd "$(dirname "$0")" && pwd)
WORK=$1
shift
(cd "$HERE/featurize" && go build -o "$WORK/featurize" .)
for spec in "$@"; do
	repo=${spec%%:*}
	years=2
	case $spec in *:[0-9]*) years=${spec##*:} ;; esac
	name=$(basename "$repo")
	python3 "$HERE/history.py" "$repo" "$WORK" "$years" | tail -1
	python3 "$HERE/past.py" "$repo" "$WORK" | tail -1
	"$WORK/featurize" touched "$repo" "$WORK/$name/changes.csv" "$WORK/$name/touched.csv"
done
