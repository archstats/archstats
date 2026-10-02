"""Builds one repo's experiment data: the tree as it stood two years before
HEAD, measured by featurize, and what happened to each of its files since.

Health is read at T0 and judged by the future, so a score can only win by
predicting trouble, not by describing trouble that already happened.

    python3 history.py <repo> <work-dir> [years]

Writes <work-dir>/<name>/{files,functions,outcomes}.csv.
"""
import csv, json, os, re, subprocess, sys
from datetime import date

# A fix commit, by its subject. Word-bounded: "prefix" and "suffix" are not fixes.
FIX = re.compile(r"\b(fix(es|ed|ing)?|bug(s|fix(es)?)?|hotfix|defect|regression|crash(es|ed)?|npe|broken|incorrect|wrong)\b", re.I)
# A commit that touches more files than this is a sweep (format, rename,
# licence header) and says nothing about any one of them.
SWEEP = 30

HERE = os.path.dirname(os.path.abspath(__file__))
TICKET = re.compile(r"\b[A-Z][A-Z0-9]+-\d+\b")


def classifier(out):
    """A commit's fix-ness: by the Jira type of the tickets it names when
    jira.py fetched them, by keyword otherwise."""
    path = os.path.join(out, "jira.json")
    types = json.load(open(path)) if os.path.exists(path) else {}

    def is_fix(subject):
        known = [types[k] for k in TICKET.findall(subject) if k in types]
        if known:
            return any(t == "Bug" for t in known)
        return bool(FIX.search(subject))
    return is_fix


def git(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], check=True, capture_output=True, text=True, errors="replace").stdout


def main():
    repo, work = sys.argv[1], sys.argv[2]
    years = int(sys.argv[3]) if len(sys.argv) > 3 else 2
    name = os.path.basename(os.path.normpath(repo))
    out = os.path.join(work, name)
    os.makedirs(out, exist_ok=True)

    head = git(repo, "log", "-1", "--format=%cs").strip()
    y, m, d = map(int, head.split("-"))
    t0_date = date(y - years, m, min(d, 28)).isoformat()
    t0 = git(repo, "rev-list", "-1", f"--before={t0_date}", "HEAD").strip()
    tree = os.path.join(out, "tree")
    if not os.path.isdir(tree):
        os.makedirs(tree)
        subprocess.run(f"git -C '{repo}' archive {t0} | tar -x -C '{tree}'", shell=True, check=True)
    featurize = os.environ.get("FEATURIZE", os.path.join(work, "featurize"))
    subprocess.run([featurize, tree, out], check=True)

    with open(os.path.join(out, "files.csv")) as f:
        tracked = {r["path"]: r["path"] for r in csv.DictReader(f)}
    commits = {p: 0 for p in tracked}
    fixes = {p: 0 for p in tracked}

    log = git(repo, "log", "--no-merges", "--reverse", "-M", "--name-status", "--format=@@%H\t%s", f"{t0}..HEAD")
    is_fix = classifier(out)
    changes = []  # (commit, path at commit, T0 path, fix) for function attribution
    entries = []
    for line in log.splitlines() + ["@@end\t"]:
        if line.startswith("@@"):
            if entries:
                settle(entries, tracked, commits, fixes, is_fix(subject), sha, changes)
            sha, _, subject = line[2:].partition("\t")
            entries = []
        elif line.strip():
            entries.append(line.split("\t"))
    with open(os.path.join(out, "outcomes.csv"), "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["path", "commits", "fixes"])
        for p in commits:
            w.writerow([p, commits[p], fixes[p]])
    with open(os.path.join(out, "changes.csv"), "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["commit", "path", "origin", "fix"])
        w.writerows(changes)
    with open(os.path.join(out, "meta.txt"), "w") as f:
        f.write(f"head={head} t0={t0_date} sha={t0}\n")
    print(name, "t0", t0_date, "files", len(commits), "fix-touched", sum(1 for v in fixes.values() if v))


def settle(entries, tracked, commits, fixes, is_fix, sha, changes):
    sweep = len(entries) > SWEEP
    for e in entries:
        status = e[0]
        if status.startswith("R") and len(e) >= 3:
            old, new = e[1], e[2]
            origin = tracked.pop(old, None)
            if origin is not None:
                tracked[new] = origin
                if not sweep:
                    commits[origin] += 1
                    fixes[origin] += is_fix
                    changes.append((sha, new, origin, int(is_fix)))
            continue
        path = e[-1]
        origin = tracked.get(path)
        if origin is None:
            continue
        if not sweep:
            commits[origin] += 1
            fixes[origin] += is_fix
            if not status.startswith("D"):
                changes.append((sha, path, origin, int(is_fix)))
        if status.startswith("D"):
            tracked.pop(path, None)


if __name__ == "__main__":
    main()
