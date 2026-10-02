"""What is each measurement worth? A Poisson regression of future fix
commits on log-scaled measurements, with one intercept per repo so a busy
project's base rate is not read as code quality. Fitted on the dev repos,
refitted on the test repos to see whether the weights hold.

    python3 fit.py <work-dir>

Not the score: the score must stay a sum of deductions a reader can follow.
This only says which deductions earn their place and roughly how much.
"""
import sys
import numpy as np
import pandas as pd
from score import load, calibrate, DEV, TEST, spearman, partial


def features(df, base):
    b = df.lang.map(base).fillna(1.5)
    level = (b.round() + 3).clip(3, 8).astype(int)
    deep = np.choose(level - 3, [df.deep3, df.deep4, df.deep5, df.deep6, df.deep7, df.deep8])
    return pd.DataFrame({
        "log2 code lines": np.log2(df.code_lines.clip(lower=1)),
        "avg nest excess": np.maximum(0, df.code_avg - b),
        "deep lines": np.log1p(deep),
        "complex fn lines": np.log1p(df.complex_fn_lines),
        "max cognitive": np.log1p(df.max_cog),
        "max fn lines": np.log1p(df.max_fn_lines),
        "bumpy fns": np.log1p(df.n_bumpy),
        "max params>4": np.log1p(np.maximum(0, df.max_params - 4)),
        "dup lines": np.log1p(df.dup_lines),
        "functions": np.log1p(df.n_funcs),
        "comment lines": np.log1p(df.comment_lines),
        "imports": np.log1p(df.imports),
        "troubled share": df.troubled_share,
        "troubled lines": np.log1p(df.troubled_lines),
    }, index=df.index)


def poisson(X, y, iters=50, ridge=1e-3):
    beta = np.zeros(X.shape[1])
    for _ in range(iters):
        mu = np.exp(np.clip(X @ beta, -30, 30))
        W = mu
        z = X @ beta + (y - mu) / np.maximum(mu, 1e-9)
        A = X.T @ (X * W[:, None]) + ridge * np.eye(X.shape[1])
        new = np.linalg.solve(A, X.T @ (W * z))
        if np.max(np.abs(new - beta)) < 1e-8:
            beta = new
            break
        beta = new
    mu = np.exp(X @ beta)
    cov = np.linalg.inv(X.T @ (X * mu[:, None]) + ridge * np.eye(X.shape[1]))
    return beta, np.sqrt(np.diag(cov))


def fit(repos, names, base, cols=None):
    frames = []
    for n in names:
        d = repos[n][~repos[n].test]
        f = features(d, base)
        if cols:
            f = f[cols]
        f["fixes"] = d.fixes.values
        f["repo"] = n
        frames.append(f)
    all_ = pd.concat(frames)
    feat = [c for c in all_.columns if c not in ("fixes", "repo")]
    mean, std = all_[feat].mean(), all_[feat].std().replace(0, 1)
    Xf = ((all_[feat] - mean) / std).values
    dummies = pd.get_dummies(all_.repo).astype(float).values
    beta, se = poisson(np.hstack([Xf, dummies]), all_.fixes.values.astype(float))
    k = len(feat)
    return pd.DataFrame({"coef/sd": beta[:k], "z": beta[:k] / se[:k]}, index=feat), beta[:k], mean, std


def judge(repos, names, base, beta, mean, std, cols=None):
    rows = []
    for n in names:
        d = repos[n][~repos[n].test]
        f = features(d, base)
        if cols:
            f = f[cols]
        risk = ((f - mean) / std).values @ beta
        busy = d.commits >= 3
        busy = busy.values
        rows.append({"repo": n[:14], "rho": spearman(risk, d.fixes), "rho|size": partial(risk, d.fixes, d.code_lines),
                     "fixshare": spearman(risk[busy], (d.fixes / d.commits.clip(lower=1))[busy])})
    t = pd.DataFrame(rows)
    return t, t[["rho", "rho|size", "fixshare"]].mean()


if __name__ == "__main__":
    repos = load(sys.argv[1])
    base = calibrate(pd.concat(repos.values()))
    for label, names in (("DEV", DEV), ("TEST", TEST)):
        table, *_ = fit(repos, names, base)
        print(f"\n== Poisson fit on {label} (standardized coefficients, z)")
        print(table.round(3).to_string())
    _, beta, mean, std = fit(repos, DEV, base)
    t, m = judge(repos, TEST, base, beta, mean, std)
    print("\n== DEV-fitted model judged on TEST\n", t.round(3).to_string(index=False), "\nmean", m.round(3).to_dict())
    _, beta, mean, std = fit(repos, DEV, base, ["log2 code lines"])
    t, m = judge(repos, TEST, base, beta, mean, std, ["log2 code lines"])
    print("size alone on TEST: mean", m.round(3).to_dict())
