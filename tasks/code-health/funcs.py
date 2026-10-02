"""Function-level evidence: which static traits of a function, measured at
T0, mark it for later bug fixes, with its length held constant.

    python3 funcs.py <work-dir>

Files make size the whole story, because more code collects more fixes.
Functions let complexity, nesting and parameters be compared between
functions of the same length, and per change: of the edits a function got,
how many were fixes.
"""
import glob, os, sys
import numpy as np
import pandas as pd
from fit import poisson

FEATS = {
    "log2 lines": lambda f: np.log2(f.lines.clip(lower=1)),
    "cognitive": lambda f: np.log2(1 + f.cognitive),
    "cyclomatic": lambda f: np.log2(f.cyclomatic.clip(lower=1)),
    "nesting": lambda f: f.max_nesting.astype(float),
    "params": lambda f: f.params.astype(float),
    "bumps": lambda f: f.bumps.astype(float),
}


def load(work):
    frames = []
    for d in sorted(glob.glob(os.path.join(work, "*", "touched.csv"))):
        root = os.path.dirname(d)
        repo = os.path.basename(root)
        fns = pd.read_csv(os.path.join(root, "functions.csv"), keep_default_na=False)
        files = pd.read_csv(os.path.join(root, "files.csv"))[["path", "lang", "test"]]
        t = pd.read_csv(d, keep_default_na=False)
        if len(t):
            agg = t.groupby(["path", "key"]).agg(changes=("commit", "nunique"),
                                                 fixes=("commit", lambda c: c[t.loc[c.index, "fix"].astype(int) == 1].nunique()))
        else:
            agg = pd.DataFrame(columns=["changes", "fixes"])
        fns = fns[fns.key != ""].merge(files, on="path").merge(agg.reset_index(), on=["path", "key"], how="left")
        fns[["changes", "fixes"]] = fns[["changes", "fixes"]].fillna(0)
        fns["repo"] = repo
        frames.append(fns[~fns.test.astype(bool)])
    return pd.concat(frames, ignore_index=True)


def design(f, cols):
    X = pd.DataFrame({c: FEATS[c](f) for c in cols}, index=f.index)
    return (X - X.mean()) / X.std().replace(0, 1)


def fit(f, cols, offset=None):
    X = design(f, cols)
    dummies = pd.get_dummies(f.repo).astype(float)
    M = np.hstack([X.values, dummies.values])
    y = f.fixes.values.astype(float)
    if offset is not None:  # rate per change: put log(changes) in as a fixed term
        beta, se = poisson_offset(M, y, offset)
    else:
        beta, se = poisson(M, y)
    k = len(cols)
    return pd.DataFrame({"coef/sd": beta[:k], "z": beta[:k] / se[:k]}, index=cols)


def poisson_offset(X, y, off, iters=60, ridge=1e-3):
    beta = np.zeros(X.shape[1])
    for _ in range(iters):
        eta = X @ beta + off
        mu = np.exp(np.clip(eta, -30, 30))
        z = X @ beta + (y - mu) / np.maximum(mu, 1e-9)
        A = X.T @ (X * mu[:, None]) + ridge * np.eye(X.shape[1])
        new = np.linalg.solve(A, X.T @ (mu * z))
        done = np.max(np.abs(new - beta)) < 1e-8
        beta = new
        if done:
            break
    mu = np.exp(X @ beta + off)
    cov = np.linalg.inv(X.T @ (X * mu[:, None]) + ridge * np.eye(X.shape[1]))
    return beta, np.sqrt(np.diag(cov))


def bands(f):
    f = f.copy()
    f["len"] = pd.cut(f.lines, [0, 10, 25, 50, 100, 1e9], labels=["≤10", "11-25", "26-50", "51-100", ">100"])
    f["cog"] = pd.cut(f.cognitive, [-1, 0, 4, 9, 15, 30, 1e9], labels=["0", "1-4", "5-9", "10-15", "16-30", ">30"])
    f["nest"] = pd.cut(f.max_nesting, [-1, 1, 2, 3, 99], labels=["0-1", "2", "3", "4+"])
    f["rate"] = f.fixes / f.groupby("repo").fixes.transform("mean").replace(0, np.nan)
    for col in ("cog", "nest"):
        t = f.pivot_table(index="len", columns=col, values="rate", aggfunc="mean", observed=False)
        n = f.pivot_table(index="len", columns=col, values="rate", aggfunc="size", observed=False)
        print(f"\n== fixes per function (relative to repo mean) by length and {col}; cells under 40 functions blank")
        print(t.where(n >= 40).round(2).to_string())
    busy = f[f.changes >= 2]
    busy = busy.assign(share=busy.fixes / busy.changes)
    for col in ("cog", "nest"):
        t = busy.pivot_table(index="len", columns=col, values="share", aggfunc="mean", observed=False)
        n = busy.pivot_table(index="len", columns=col, values="share", aggfunc="size", observed=False)
        print(f"\n== share of a function's edits that were fixes (2+ edits), by length and {col}; cells under 40 blank")
        print(t.where(n >= 40).round(2).to_string())


if __name__ == "__main__":
    f = load(sys.argv[1])
    print("functions:", len(f), "repos:", f.repo.nunique(), "edited:", int((f.changes > 0).sum()), "fixed:", int((f.fixes > 0).sum()))
    print(f.groupby("repo").agg(functions=("key", "size"), edited=("changes", lambda c: int((c > 0).sum())), fixed=("fixes", lambda c: int((c > 0).sum()))).to_string())
    cols = ["log2 lines", "cognitive", "nesting", "params", "bumps"]
    print("\n== fixes per function, all repos (Poisson, repo baselines)")
    print(fit(f, cols).round(3).to_string())
    print("\n== fixes per change, functions edited at least once (offset log changes)")
    e = f[f.changes > 0]
    print(fit(e, cols, offset=np.log(e.changes.values)).round(3).to_string())
    rows = []
    for repo, g in f.groupby("repo"):
        if (g.fixes > 0).sum() < 30:
            continue
        r = fit(g, cols)["coef/sd"].to_dict()
        ge = g[g.changes > 0]
        rc = fit(ge, cols, offset=np.log(ge.changes.values))["coef/sd"].to_dict()
        rows.append({"repo": repo[:14], **{f"{k}": v for k, v in r.items()}, **{f"per-change {k}": v for k, v in rc.items() if k != "log2 lines"}})
    t = pd.DataFrame(rows).set_index("repo")
    print("\n== per repo (fixes per function | per change)")
    print(t.round(2).to_string())
    print("\npositive in N of", len(t), "repos:", (t > 0).sum().to_dict())
    bands(f)
