"""Per-file history in the two years before T0, with dates, authors and
lines, for churn variants: python3 pastlog.py <repo> <work-dir>"""
import csv, math, os, subprocess, sys
from datetime import date

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
from history import SWEEP, classifier

repo, work = sys.argv[1], sys.argv[2]
out = os.path.join(work, os.path.basename(os.path.normpath(repo)))
meta = dict(kv.split("=") for kv in open(os.path.join(out, "meta.txt")).read().split())
t0 = date.fromisoformat(meta["t0"])
since = f"{t0.year - 2}-{t0.month:02d}-{t0.day:02d}"
paths = {r["path"] for r in csv.DictReader(open(os.path.join(out, "files.csv")))}
is_fix = classifier(out)
log = subprocess.run(["git", "-C", repo, "log", "--no-merges", "--numstat", f"--since={since}",
                      "--format=@@%cs\t%ae\t%s", meta["sha"]], capture_output=True, text=True, errors="replace").stdout

S = {p: {"c3": 0, "c6": 0, "c12": 0, "decay": 0.0, "authors": set(), "churned": 0, "fix_decay": 0.0, "last": 9999} for p in paths}


def settle(day, author, subject, touched):
    if day is None or len(touched) > SWEEP:
        return
    age = (t0 - date.fromisoformat(day)).days
    w = 0.5 ** (age / 182)
    fix = is_fix(subject)
    for p, n in touched:
        s = S.get(p)
        if s is None:
            continue
        s["c3"] += age <= 91
        s["c6"] += age <= 182
        s["c12"] += age <= 365
        s["decay"] += w
        s["fix_decay"] += w * fix
        s["authors"].add(author)
        s["churned"] += n
        s["last"] = min(s["last"], age)


day = author = subject = None
touched = []
for line in log.splitlines():
    if line.startswith("@@"):
        settle(day, author, subject, touched)
        day, author, subject = line[2:].split("\t", 2)
        touched = []
    elif line.strip():
        parts = line.split("\t", 2)
        if len(parts) != 3:
            continue
        a, d, p = parts
        touched.append((p, (int(a) if a.isdigit() else 0) + (int(d) if d.isdigit() else 0)))
settle(day, author, subject, touched)

with open(os.path.join(out, "pastlog.csv"), "w", newline="") as f:
    w = csv.writer(f)
    w.writerow(["path", "c3", "c6", "c12", "decay", "authors", "churned", "fix_decay", "days_since"])
    for p, s in sorted(S.items()):
        w.writerow([p, s["c3"], s["c6"], s["c12"], round(s["decay"], 4), len(s["authors"]), s["churned"], round(s["fix_decay"], 4), s["last"]])
print(out, "done")
