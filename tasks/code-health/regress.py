"""The engine against the experiment: scores the health the engine itself
stored for each T0 tree, before the change (engine.db) and after it
(engine2.db), by the same later fixes the formulas were chosen on.

    python3 regress.py <work-dir>

The new engine should match the prototype formula (h4) it implements, and
beat the old engine's health on the same files.
"""
import os, sqlite3, sys
import numpy as np
import pandas as pd
from score import load, DEV, TEST, evaluate, h4


def engine_health(work, name, db):
    path = os.path.join(work, name, db)
    con = sqlite3.connect(path)
    cols = {r[1] for r in con.execute("pragma table_info(files)")}
    want = ["codesmells__code_health", "complexity__lines__code", "complexity__lines__complex", "modularity__imports__count",
            "codesmells__health__deduction__deep_code"]
    pick = ", ".join(f"{c}" if c in cols else f"null as {c}" for c in want)
    df = pd.read_sql(f"select name as path, {pick} from files", con)
    con.close()
    df["path"] = df.path.str.replace(r"^\./", "", regex=True)
    return df


def mean_eval(repos, names, col):
    per = []
    for n in names:
        d = repos[n]
        d = d[d[col].notna()]
        per.append(evaluate(d, d[col]))
    per = [p for p in per if p["fixed"] >= 20]
    return {k: round(float(np.nanmean([p[k] for p in per])), 3) for k in ("rho", "fixshare", "recall20", "at10", "red")}


if __name__ == "__main__":
    work = sys.argv[1]
    repos = load(work)
    for n in list(repos):
        old = engine_health(work, n, "engine.db")[["path", "codesmells__code_health"]].rename(columns={"codesmells__code_health": "engine_old"})
        new = engine_health(work, n, "engine2.db").rename(columns={"codesmells__code_health": "engine_new"})
        d = repos[n].merge(old, on="path", how="left").merge(new, on="path", how="left")
        d["prototype"] = h4(d)
        repos[n] = d
    allf = pd.concat([r[~r.test] for r in repos.values()])
    print("files with a reading: old engine", int(allf.engine_old.notna().sum()), " new engine", int(allf.engine_new.notna().sum()), "of", len(allf))
    both = allf[allf.engine_new.notna()]
    print("new engine vs prototype formula on the same files: Spearman",
          round(both.engine_new.rank().corr(both.prototype.rank()), 3),
          " mean absolute difference", round(float((both.engine_new - both.prototype).abs().mean()), 3))
    print("inputs, engine vs prototype (Spearman): code lines",
          round(both["complexity__lines__code"].rank().corr(both.code_lines.rank()), 3),
          " complex lines", round(both["complexity__lines__complex"].rank().corr(both.complex_fn_lines.rank()), 3),
          " imports", round(both["modularity__imports__count"].rank().corr(both.imports.rank()), 3))
    for label, names in (("DEV", DEV), ("TEST", TEST)):
        print(f"\n== {label}")
        for col in ("engine_old", "prototype", "engine_new"):
            print(f"  {col:11s}", mean_eval(repos, names, col))
    rows = []
    for n in DEV + TEST:
        d = repos[n]
        d = d[d.engine_new.notna() & d.engine_old.notna()]
        a, b = evaluate(d, d.engine_old), evaluate(d, d.engine_new)
        rows.append({"repo": n[:16], "fixed": a["fixed"], "old rho": a["rho"], "new rho": b["rho"], "old fixshare": a["fixshare"], "new fixshare": b["fixshare"]})
    print()
    print(pd.DataFrame(rows).round(3).to_string(index=False))
