"""Which coupling measure should health use? Compares the import count the
prototype counted with what the engine itself knows about each file, by
scanning the T0 trees with the engine (scan/main.go -> <repo>/engine.db).

    python3 coupling.py <work-dir>

Each candidate replaces the coupling term of the decided formula; its
weight is tuned on the dev repos and judged on the test repos.
"""
import os, sqlite3, sys
import numpy as np
import pandas as pd
from score import load, DEV, TEST, evaluate, spearman
from fit import poisson


def engine_coupling(work, name):
    db = os.path.join(work, name, "engine.db")
    con = sqlite3.connect(db)
    cols = {r[1] for r in con.execute("pragma table_info(files)")}
    pick = lambda c: c if c in cols else "0"
    files = pd.read_sql(f"select name as path, {pick('modularity__import__raw')} as raw, {pick('modularity__component__imports')} as comp from files", con)
    edges = pd.read_sql("select distinct from_file, to_file from unit_connections where from_file != '' and to_file != '' and from_file != to_file", con)
    con.close()
    strip = lambda s: s.str.replace(r"^\./", "", regex=True)
    files["path"] = strip(files.path)
    edges["from_file"], edges["to_file"] = strip(edges.from_file), strip(edges.to_file)
    out = edges.groupby("from_file").to_file.nunique().rename("fan_out")
    inn = edges.groupby("to_file").from_file.nunique().rename("fan_in")
    files = files.merge(out, left_on="path", right_index=True, how="left").merge(inn, left_on="path", right_index=True, how="left")
    return files.fillna(0)


CANDIDATES = {
    "prototype imports": "imports",
    "engine raw imports": "raw",
    "engine component imports": "comp",
    "engine resolved fan-out": "fan_out",
    "engine fan-in": "fan_in",
}


def health(d, col, weight, base):
    return np.maximum(1, 10 - np.log2(np.maximum(d.code_lines, 150) / 150)
                      - weight * np.log2(np.maximum(d[col], base) / base)
                      - np.log2(1 + d.complex_fn_lines / 25))


def mean_eval(repos, names, f):
    per = [evaluate(repos[n], f(repos[n])) for n in names]
    per = [p for p in per if p["fixed"] >= 20]
    return {k: float(np.nanmean([p[k] for p in per])) for k in ("rho", "fixshare", "recall20")}


if __name__ == "__main__":
    work = sys.argv[1]
    repos = load(work)
    for n in list(repos):
        repos[n] = repos[n].merge(engine_coupling(work, n), on="path", how="left").fillna({"raw": 0, "comp": 0, "fan_out": 0, "fan_in": 0})
    allf = pd.concat([r[~r.test] for r in repos.values()])
    print("coverage (share of production files with a nonzero value):")
    print({k: round(float((allf[c] > 0).mean()), 3) for k, c in CANDIDATES.items()})
    print("by language, median:")
    print(allf.groupby("lang")[list(CANDIDATES.values())].median().to_string())

    # What each adds beyond size and complex code, per file: Poisson z.
    rows = []
    for label, names in (("DEV", DEV), ("TEST", TEST)):
        d = allf[allf.repo.isin(names)]
        for k, c in CANDIDATES.items():
            X = np.column_stack([np.log2(np.maximum(d.code_lines, 150) / 150), np.log2(1 + d.complex_fn_lines / 25),
                                 np.log2(1 + d[c]), pd.get_dummies(d.repo).astype(float).values])
            b, se = poisson(X, d.fixes.values.astype(float))
            rows.append({"set": label, "coupling": k, "x per doubling": round(float(np.exp(b[2])), 3), "z": round(float(b[2] / se[2]), 1)})
    print("\nper-file effect beyond size and complex code:")
    print(pd.DataFrame(rows).pivot(index="coupling", columns="set", values=["x per doubling", "z"]).to_string())

    # Weight and threshold tuned on DEV, judged on TEST.
    none = {s: mean_eval(repos, names, lambda d: health(d, "imports", 0, 10)) for s, names in (("dev", DEV), ("test", TEST))}
    print("\nno coupling term:", {s: {k: round(v, 3) for k, v in r.items()} for s, r in none.items()})
    for k, c in CANDIDATES.items():
        best = None
        for base in (3, 5, 10, 20):
            for w in (0.5, 1, 1.5, 2):
                r = mean_eval(repos, DEV, lambda d: health(d, c, w, base))
                score = r["rho"] + r["fixshare"] + r["recall20"]
                if best is None or score > best[0]:
                    best = (score, w, base, r)
        _, w, base, dev = best
        test = mean_eval(repos, TEST, lambda d: health(d, c, w, base))
        print(f"{k:26s} weight {w} past {base:2d}: dev {({m: round(v, 3) for m, v in dev.items()})}  test {({m: round(v, 3) for m, v in test.items()})}")
