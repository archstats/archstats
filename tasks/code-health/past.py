"""Adds each T0 file's past to the experiment data: commits and fix commits
in the two years before T0, so health can be compared with what the
Hotspots view already knows.

    python3 past.py <repo> <work-dir>   (after history.py)
"""
import csv, os, subprocess, sys
from history import SWEEP, classifier, git


def main():
    repo, work = sys.argv[1], sys.argv[2]
    out = os.path.join(work, os.path.basename(os.path.normpath(repo)))
    meta = dict(kv.split("=") for kv in open(os.path.join(out, "meta.txt")).read().split())
    t0, t0_date = meta["sha"], meta["t0"]
    y, rest = t0_date.split("-", 1)
    since = f"{int(y) - 2}-{rest}"
    with open(os.path.join(out, "files.csv")) as f:
        paths = {r["path"] for r in csv.DictReader(f)}
    commits = dict.fromkeys(paths, 0)
    fixes = dict.fromkeys(paths, 0)
    is_fix = classifier(out)
    log = git(repo, "log", "--no-merges", "--name-only", f"--since={since}", "--format=@@%s", t0)
    subject, touched = None, []

    def settle():
        if subject is None or len(touched) > SWEEP:
            return
        fix = is_fix(subject)
        for p in touched:
            if p in commits:
                commits[p] += 1
                fixes[p] += fix

    for line in log.splitlines():
        if line.startswith("@@"):
            settle()
            subject, touched = line[2:], []
        elif line.strip():
            touched.append(line.strip())
    settle()
    with open(os.path.join(out, "past.csv"), "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["path", "past_commits", "past_fixes"])
        for p in sorted(paths):
            w.writerow([p, commits[p], fixes[p]])
    print(out, "files with past commits:", sum(1 for v in commits.values() if v))


if __name__ == "__main__":
    main()
