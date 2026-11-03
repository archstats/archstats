"""Variants of bad code x churn, judged like analyze.py.

    python3 interest.py <work-dir>
"""
import os, sys
import numpy as np
import pandas as pd

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import score as S

W = sys.argv[1]
repos = {}
for k, v in S.load(W).items():
    v = v[~v.test]
    if (v.fixes > 0).sum() < 20:
        continue
    v = v.merge(pd.read_csv(os.path.join(W, k, "pastlog.csv")), on="path", how="left").fillna(0)
    repos[k] = v.copy()

L = lambda x: np.log2(1 + x)


def variants(d):
    p = S.h4_points(d)
    bad = p.sum(axis=1)
    lines = d.code_lines.clip(lower=1)
    tb = d.code_lines / 1e9
    v = {}
    # churn measures, bad fixed to the full health deductions
    v["base: bad x commits(2y)"] = bad * L(d.past_commits)
    v["bad x commits(12m)"] = bad * L(d.c12)
    v["bad x commits(6m)"] = bad * L(d.c6)
    v["bad x commits(3m)"] = bad * L(d.c3)
    v["bad x decayed commits (6m half-life)"] = bad * L(d.decay)
    v["bad x authors"] = bad * L(d.authors)
    v["bad x lines churned"] = bad * L(d.churned)
    v["bad x relative churn (lines/size)"] = bad * L(d.churned / lines)
    v["bad x decayed fix commits"] = bad * L(d.fix_decay)
    # what "bad" means, churn fixed to decayed commits
    c = L(d.decay)
    v["complex-code ded. x decayed"] = p["complex code"] * c
    v["coupling ded. x decayed"] = p["coupling"] * c
    v["size ded. x decayed"] = p["size"] * c
    v["bad minus size x decayed"] = (p["complex code"] + p["coupling"]) * c
    # shape: does a green file that changes a lot still rank?
    v["(1 + bad) x decayed"] = (1 + bad) * c
    v["(1 + bad) x decayed x (1 + authors/5)"] = (1 + bad) * c * (1 + d.authors / 5)
    v["rank: sqrt(pct bad x pct decayed)"] = np.sqrt(bad.rank(pct=True) * d.decay.rank(pct=True))
    # references
    v["ref: size"] = d.code_lines.astype(float)
    v["ref: decayed commits"] = d.decay + tb
    v["ref: 4-way rank mean"] = (d.code_lines.rank(pct=True) + (d.complex_fn_lines + d.max_cog / 1e3).rank(pct=True)
                                 + bad.rank(pct=True) + (L(d.code_lines) * L(d.past_commits)).rank(pct=True)) / 4
    return {k: x + tb for k, x in v.items()}


def evaluate(d, s):
    s = pd.Series(np.asarray(s, float), index=d.index)
    r = {"rho": S.spearman(s, d.fixes), "rho|size": S.partial(s, d.fixes, d.code_lines)}
    busy = d.commits >= 3
    r["fixshare"] = S.spearman(s[busy], (d.fixes / d.commits.clip(lower=1))[busy])
    o = d.assign(s=s, tb=S.tiebreak(d.path)).sort_values(["s", "tb"], ascending=[False, True])
    r["fix@20%lines"] = o.fixes[o.code_lines.cumsum() <= 0.2 * o.code_lines.sum()].sum() / max(1, o.fixes.sum())
    top = o.head(20)
    r["top20 fixes"] = top.fixes.sum() / max(1, o.fixes.sum())
    r["top20 lines"] = top.code_lines.sum() / max(1, o.code_lines.sum())
    r["top20 hit>=3"] = float((top.fixes >= 3).mean())
    top50 = o.head(50)
    r["top50 fixes"] = top50.fixes.sum() / max(1, o.fixes.sum())
    return r


rows = []
for n, d in repos.items():
    for k, s in variants(d).items():
        rows.append({"variant": k, "repo": n, **evaluate(d, s)})
R = pd.DataFrame(rows)
T = R.groupby("variant", sort=False).mean(numeric_only=True)
base = R[R.variant == "base: bad x commits(2y)"].set_index("repo").rho
R["beats base"] = R.apply(lambda r: r.rho > base[r.repo] + 1e-9, axis=1)
T["repos > base"] = R.groupby("variant", sort=False)["beats base"].sum()
pd.set_option("display.width", 250)
print(T.round(3).to_string())

# What a top-20 list looks like under the best candidate, in one repo
for n in ["hibernate-orm", "sakai"]:
    d = repos[n]
    v = variants(d)
    o = d.assign(s=v["(1 + bad) x decayed"], health=S.h4(d)).sort_values("s", ascending=False).head(8)
    print(f"\n{n}: top of (1 + bad) x decayed")
    print(o[["path", "code_lines", "health", "decay", "authors", "fixes"]].assign(path=o.path.str[-60:]).round(1).to_string(index=False))
