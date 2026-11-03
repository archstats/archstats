"""How interest treats new files against old ones.

    python3 fileage.py <work-dir>

A file's age at T0 is from the commit that added it under its T0 path
(git log --diff-filter=A, renames followed, so a moved file keeps its age).
Variants drop the commits made while a file was being written.
"""
import os, subprocess, sys
from datetime import date
import numpy as np
import pandas as pd

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import score as S

W = sys.argv[1]
P = os.environ.get("REPOS", os.path.expanduser("~/workspace-manager/workspaces/personal"))
L = lambda x: np.log2(1 + x)
HL = 182


def added(repo, sha, t0):
    """T0 path -> days before T0 it was added; renames carry the age."""
    log = subprocess.run(["git", "-C", repo, "log", "-M", "--name-status", "--format=@@%cs", sha],
                         capture_output=True, text=True, errors="replace").stdout
    born, alias, day = {}, {}, None
    # newest first: alias maps an older name onto the T0 path
    for line in log.splitlines():
        if line.startswith("@@"):
            day = date.fromisoformat(line[2:])
            continue
        e = line.split("\t")
        if len(e) < 2:
            continue
        if e[0].startswith("R") and len(e) >= 3:
            alias[e[1]] = alias.get(e[2], e[2])
        elif e[0] == "A":
            p = alias.get(e[1], e[1])
            born.setdefault(p, (t0 - day).days)
    return born


rows, evals = [], []
REPOS = S.load(W)
for name, d in REPOS.items():
    d = d[~d.test].copy()
    if (d.fixes > 0).sum() < 20:
        continue
    root = os.path.join(W, name)
    meta = dict(kv.split("=") for kv in open(os.path.join(root, "meta.txt")).read().split())
    t0 = date.fromisoformat(meta["t0"])
    born = added(os.path.join(P, name), meta["sha"], t0)
    d["age"] = d.path.map(born).fillna(99999)
    # per-file commit ages before T0, from pastchanges + pastcommits
    ch = pd.read_csv(os.path.join(root, "pastchanges.csv")).merge(pd.read_csv(os.path.join(root, "pastcommits.csv"))[["commit", "age_days"]], on="commit")
    ch = ch.drop_duplicates(["commit", "origin"]).merge(d[["path", "age"]], left_on="origin", right_on="path")
    ch["w"] = 0.5 ** (ch.age_days / HL)
    since_birth = ch.age - ch.age_days  # days after the file was added
    g = lambda m: ch[m].groupby("origin").w.sum()
    d["decay"] = d.path.map(g(np.ones(len(ch), bool))).fillna(0)
    d["decay_no_add"] = d.path.map(g(since_birth > 0)).fillna(0)
    d["decay_no_30d"] = d.path.map(g(since_birth > 30)).fillna(0)
    d["decay_no_90d"] = d.path.map(g(since_birth > 90)).fillna(0)
    bad = S.h4_points(d).sum(axis=1)
    tb = d.code_lines / 1e9
    d["interest"] = (1 + bad) * L(d.decay) + tb
    variants = {"interest": d.interest,
                "minus the adding commit": (1 + bad) * L(d.decay_no_add) + tb,
                "minus first 30 days": (1 + bad) * L(d.decay_no_30d) + tb,
                "minus first 90 days": (1 + bad) * L(d.decay_no_90d) + tb,
                "first 90 days at half weight": (1 + bad) * L(d.decay_no_90d + 0.5 * (d.decay - d.decay_no_90d)) + tb}
    d["band"] = pd.cut(d.age, [-1, 182, 365, 730, 1e9], labels=["< 6 months", "6-12 months", "1-2 years", "> 2 years"])
    top = d.nlargest(100, "interest")
    for b, g_ in d.groupby("band", observed=False):
        rows.append({"repo": name, "band": b, "files": len(g_) / len(d),
                     "in top 100 by interest": (top.band == b).mean(),
                     "later fixes": g_.fixes.sum() / max(1, d.fixes.sum()),
                     "fixes per 1k lines": 1000 * g_.fixes.sum() / max(1, g_.code_lines.sum()),
                     "median recent commits": g_.decay.median(),
                     "rho within band": S.spearman(g_.interest, g_.fixes) if len(g_) > 30 else np.nan})
    for k, s in variants.items():
        o = d.assign(s=s).sort_values("s", ascending=False)
        t = o.head(100)
        evals.append({"variant": k, "repo": name, "rho": S.spearman(s, d.fixes),
                      "fix@20%lines": o.fixes[o.code_lines.cumsum() <= 0.2 * o.code_lines.sum()].sum() / max(1, o.fixes.sum()),
                      "top100 fixes": t.fixes.sum() / max(1, o.fixes.sum()),
                      "new (<6m) in top100": (t.age <= 182).mean()})

R = pd.DataFrame(rows)
pd.set_option("display.width", 220)
print("== By age at T0, mean over repos")
print(R.groupby("band", observed=False).mean(numeric_only=True).round(3).to_string())
print("\n== Same, pooled fixes per 1k lines and top-100 share per repo")
print(R.pivot(index="repo", columns="band", values="in top 100 by interest").round(2).to_string())
print("\n== Handling the writing burst")
E = pd.DataFrame(evals)
print(E.groupby("variant", sort=False).mean(numeric_only=True).round(3).to_string())
print(E.pivot(index="variant", columns="repo", values="rho").round(3).to_string())
print(E.pivot(index="variant", columns="repo", values="top100 fixes").round(3).to_string())
