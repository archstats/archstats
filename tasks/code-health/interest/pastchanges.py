"""Every change to a T0 file in the two years before T0, in the shape
featurize's `touched` reads (commit, path at that commit, T0 path, fix),
plus each commit's age at T0 for decayed churn.

    python3 pastchanges.py <repo> <work-dir>

Writes <work-dir>/<name>/pastchanges.csv and pastcommits.csv.
"""
import csv, os, subprocess, sys
from datetime import date

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
from history import SWEEP, classifier

repo, work = sys.argv[1], sys.argv[2]
out = os.path.join(work, os.path.basename(os.path.normpath(repo)))
meta = dict(kv.split("=") for kv in open(os.path.join(out, "meta.txt")).read().split())
t0 = date.fromisoformat(meta["t0"])
since = f"{t0.year - 2}-{t0.month:02d}-{t0.day:02d}"
# Newest first, so a rename maps its old name onto the T0 path before older commits use it.
tracked = {r["path"]: r["path"] for r in csv.DictReader(open(os.path.join(out, "files.csv")))}
is_fix = classifier(out)
log = subprocess.run(["git", "-C", repo, "log", "--no-merges", "-M", "--name-status", f"--since={since}",
                      "--format=@@%H\t%cs\t%s", meta["sha"]], capture_output=True, text=True, errors="replace").stdout

changes, commits = [], []


def settle(sha, day, subject, entries):
    if sha is None:
        return
    fix = int(is_fix(subject))
    sweep = len(entries) > SWEEP
    renames = []
    for e in entries:
        status, path = e[0], e[-1]
        origin = tracked.get(path)
        if origin is None:
            continue
        if not sweep and not status.startswith("D"):
            changes.append((sha, path, origin, fix))
        if status.startswith("R") and len(e) >= 3:
            renames.append((e[1], origin))
    for old, origin in renames:
        tracked[old] = origin
    commits.append((sha, (t0 - date.fromisoformat(day)).days, fix))


sha = day = subject = None
entries = []
for line in log.splitlines():
    if line.startswith("@@"):
        settle(sha, day, subject, entries)
        sha, day, subject = line[2:].split("\t", 2)
        entries = []
    elif line.strip():
        entries.append(line.split("\t"))
settle(sha, day, subject, entries)

with open(os.path.join(out, "pastchanges.csv"), "w", newline="") as f:
    w = csv.writer(f)
    w.writerow(["commit", "path", "origin", "fix"])
    w.writerows(changes)
with open(os.path.join(out, "pastcommits.csv"), "w", newline="") as f:
    w = csv.writer(f)
    w.writerow(["commit", "age_days", "fix"])
    w.writerows(commits)
print(out, len(changes), "past file changes")
