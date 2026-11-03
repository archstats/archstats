"""Function-level interest: does a complex function that keeps being edited
predict where fixes land next, and does it narrow a hot file down to what
to fix?

    python3 funcinterest.py <work-dir>

Inputs per repo (after pastchanges.py and featurize touched on both sides):
functions.csv (T0), pasttouched.csv + pastcommits.csv (edits before T0),
touched.csv (edits after T0), pastlog.csv (file churn for the file score).
"""
import os, sys
import numpy as np
import pandas as pd

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import score as S

W = sys.argv[1]
HALF_LIFE = 182
L = lambda x: np.log2(1 + x)


def edits(touched, commits=None):
    t = pd.read_csv(touched, keep_default_na=False)
    if commits is not None:
        t = t.merge(pd.read_csv(commits)[["commit", "age_days"]], on="commit", how="left")
        t["w"] = 0.5 ** (t.age_days.fillna(730) / HALF_LIFE)
    else:
        t["w"] = 1.0
    t["fix"] = t.fix.astype(int)
    g = t.groupby(["path", "key"])
    return pd.DataFrame({"edits": g.commit.nunique(), "decayed": g.w.sum(),
                         "fixes": g.apply(lambda x: x.loc[x.fix == 1, "commit"].nunique())})


def load(name):
    root = os.path.join(W, name)
    fns = pd.read_csv(os.path.join(root, "functions.csv"), keep_default_na=False)
    meta = pd.read_csv(os.path.join(root, "files.csv"))
    fns = fns[fns.key != ""].merge(meta[["path", "test", "code_lines", "imports"]], on="path")
    fns = fns[~fns.test.astype(bool)]
    past = edits(os.path.join(root, "pasttouched.csv"), os.path.join(root, "pastcommits.csv"))
    fut = edits(os.path.join(root, "touched.csv")).rename(columns={"edits": "f_edits", "fixes": "f_fixes"})[["f_edits", "f_fixes"]]
    fns = fns.merge(past, left_on=["path", "key"], right_index=True, how="left") \
             .merge(fut, left_on=["path", "key"], right_index=True, how="left")
    fns = fns.fillna({"edits": 0, "decayed": 0, "fixes": 0, "f_edits": 0, "f_fixes": 0})
    fns = fns.drop_duplicates(["path", "key"])
    # file-level interest, as in interest.py: (1 + health deductions) x decayed commits
    fl = pd.read_csv(os.path.join(root, "pastlog.csv"))
    fn_complex = fns.assign(cl=np.where(fns.cognitive > 15, fns.lines, 0)).groupby("path").cl.sum().rename("complex_fn_lines")
    meta = meta.merge(fn_complex, left_on="path", right_index=True, how="left").fillna({"complex_fn_lines": 0}).merge(fl, on="path", how="left").fillna(0)
    meta["bad"] = S.h4_points(meta).sum(axis=1)
    meta["file_interest"] = (1 + meta.bad) * L(meta.decay)
    fns = fns.merge(meta[["path", "file_interest", "bad"]].rename(columns={"bad": "file_bad"}), on="path", how="left")
    return fns


def fn_bad(f):
    """The file deduction for complex code, per function: 1 point per
    doubling of (1 + lines / 25) once cognitive complexity passes 15."""
    return np.where(f.cognitive > 15, L(f.lines / 25), 0.0)


def candidates(f):
    tb = f.lines / 1e9
    bad = fn_bad(f)
    return {
        "lines": f.lines.astype(float),
        "cognitive": f.cognitive + tb,
        "past edits (decayed)": f.decayed + tb,
        "bad x edits": bad * L(f.decayed) + tb,
        "(1 + bad) x edits": (1 + bad) * L(f.decayed) + tb,
        "(1 + cognitive/15) x edits": (1 + f.cognitive / 15) * L(f.decayed) + tb,
        "its file's interest": f.file_interest + tb,
        "file interest, then (1 + bad) x edits": f.file_interest.rank(pct=True) + ((1 + bad) * L(f.decayed)).rank(pct=True) + tb,
    }


def evaluate(f, s):
    s = pd.Series(np.asarray(s, float), index=f.index)
    r = {"rho fixes": S.spearman(s, f.f_fixes), "rho|lines": S.partial(s, f.f_fixes, f.lines),
         "rho edits": S.spearman(s, f.f_edits)}
    o = f.assign(s=s).sort_values("s", ascending=False)
    r["fixes@5% lines"] = o.f_fixes[o.lines.cumsum() <= 0.05 * o.lines.sum()].sum() / max(1, o.f_fixes.sum())
    r["fixes@20% lines"] = o.f_fixes[o.lines.cumsum() <= 0.2 * o.lines.sum()].sum() / max(1, o.f_fixes.sum())
    top = o.head(20)
    r["top20 fixes"] = top.f_fixes.sum() / max(1, o.f_fixes.sum())
    r["top20 lines"] = top.lines.sum() / max(1, o.lines.sum())
    r["top20 hit>=2"] = float((top.f_fixes >= 2).mean())
    return r


names = [n for n in ["Sylius", "django-oscar", "fineract", "hibernate-orm", "librechat", "nopCommerce", "sakai"]
         if os.path.exists(os.path.join(W, n, "pasttouched.csv"))]
data = {n: load(n) for n in names}

print("== Corpus (production functions)")
for n, f in data.items():
    print(f"{n:15s} functions {len(f):7d}  edited before {int((f.edits > 0).sum()):6d}  "
          f"fixed after {int((f.f_fixes > 0).sum()):5d}  fixes after {int(f.f_fixes.sum()):6d}")

rows = []
for n, f in data.items():
    for k, s in candidates(f).items():
        rows.append({"candidate": k, "repo": n, **evaluate(f, s)})
R = pd.DataFrame(rows)
pd.set_option("display.width", 250)
print("\n== Ranking functions against fixes after T0 (mean over repos)")
print(R.groupby("candidate", sort=False).mean(numeric_only=True).round(3).to_string())
print("\n== rho fixes per repo")
print(R.pivot(index="candidate", columns="repo", values="rho fixes").round(2).to_string())

# Inside the hottest files: do 3 functions say where to start?
rows = []
for n, f in data.items():
    hot = f.drop_duplicates("path").nlargest(10, "file_interest").path
    for p in hot:
        g = f[f.path == p]
        if g.f_fixes.sum() == 0 or len(g) < 5:
            continue
        bad = pd.Series(fn_bad(g), index=g.index)
        picks = {"3 by (1 + bad) x edits": ((1 + bad) * L(g.decayed) + g.lines / 1e9).nlargest(3).index,
                 "3 by past edits": (g.decayed + g.lines / 1e9).nlargest(3).index,
                 "3 longest": g.lines.nlargest(3).index,
                 "3 most complex": (g.cognitive + g.lines / 1e9).nlargest(3).index}
        for k, idx in picks.items():
            rows.append({"pick": k, "repo": n, "file": p,
                         "share of file's fn lines": g.loc[idx, "lines"].sum() / g.lines.sum(),
                         "share of file's later fn fixes": g.loc[idx, "f_fixes"].sum() / g.f_fixes.sum()})
H = pd.DataFrame(rows)
print(f"\n== Inside the 10 hottest files per repo ({H.file.nunique()} files with later fixes)")
print(H.groupby("pick", sort=False)[["share of file's fn lines", "share of file's later fn fixes"]].mean().round(3).to_string())

for n in ["hibernate-orm", "sakai", "fineract"]:
    if n not in data:
        continue
    f = data[n]
    print(f"\n{n}: hottest 3 files, top 3 functions by (1 + bad) x edits")
    for p in f.drop_duplicates("path").nlargest(3, "file_interest").path:
        g = f[f.path == p].copy()
        g["interest"] = (1 + fn_bad(g)) * L(g.decayed)
        t = g.nlargest(3, "interest")
        print(f"  {p.rsplit('/', 1)[-1]}  ({int(g.lines.sum())} fn lines, {int(g.f_fixes.sum())} later fn fixes)")
        for _, r in t.iterrows():
            print(f"     {r.key.split('.', 1)[-1][:55]:55s} lines {int(r.lines):4d}  cog {int(r.cognitive):4d}  "
                  f"past edits {int(r.edits):3d}  later fixes {int(r.f_fixes):3d}")
