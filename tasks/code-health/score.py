"""Scores every candidate health formula on the experiment data that
history.py built, and reports how well each predicts the future.

    python3 score.py <work-dir> [explore]

A formula is judged on production files only, per repo, then averaged with
each repo weighted equally:

  rho        Spearman between unhealthiness at T0 and fix commits since.
  fixshare   Spearman with the share of a file's later commits that were
             fixes, files with 3+ commits: quality per change, not activity.
             (What a score knows beyond size is fit.py's question: a partial
             correlation flatters any score with a plateau.)
  recall20   Share of all fixes found by reading the least healthy files
             first until 20% of the code is read. Chance is 20%.
  red/green  Fixes per 1,000 code lines in red files (< 4) over green (>= 8).
  at10       Share of files at a perfect score: the ceiling.
"""
import glob, math, os, sys, zlib
import numpy as np
import pandas as pd

DEV = ["librechat", "nopCommerce", "npo-diaz-recommendations-and-search", "sakai", "django-oscar", "npo-datahub-hubd",
       "hugo", "excalidraw"]
TEST = ["fineract", "hibernate-orm", "Sylius", "BroadleafCommerce", "caddy", "outline", "django-rest-framework"]


# ---------------------------------------------------------------- data

def load(work):
    repos = {}
    for d in sorted(glob.glob(os.path.join(work, "*", "outcomes.csv"))):
        name = os.path.basename(os.path.dirname(d))
        root = os.path.dirname(d)
        files = pd.read_csv(os.path.join(root, "files.csv"))
        out = pd.read_csv(os.path.join(root, "outcomes.csv"))
        fns = pd.read_csv(os.path.join(root, "functions.csv"))
        files = files.merge(out, on="path", how="left").fillna({"commits": 0, "fixes": 0})
        past = os.path.join(root, "past.csv")
        if os.path.exists(past):
            files = files.merge(pd.read_csv(past), on="path", how="left")
        for c in ("past_commits", "past_fixes"):
            files[c] = files[c].fillna(0) if c in files else 0
        files["repo"] = name
        files["code_avg"] = sum(files[f"deep{k}"] for k in range(1, 9)) / files.code_lines.clip(lower=1)
        if "imports" not in files:
            files["imports"] = 0
        files["ext"] = files.path.str.extract(r"(\.[^./]+)$")[0].str.lower()
        files = files.merge(function_features(fns), on="path", how="left")
        for c in FN_COLS:
            files[c] = files[c].fillna(0)
        files["troubled_share"] = (files.troubled_lines / files.code_lines.clip(lower=1)).clip(upper=1)
        repos[name] = files
    return repos


FN_COLS = ["n_funcs", "max_cog", "sum_cog", "max_fn_lines", "max_ast_nest", "max_params",
           "n_bumpy", "complex_fn_lines", "fn_pen_sum", "fn_pen_max", "troubled_lines", "troubled_fns"]


def troubled(f):
    """A function that funcs.py found edits to go wrong in: long, and complex
    or deeply nested. Short functions showed no such effect however written."""
    return (f.lines > 50) & ((f.cognitive > 15) | (f.max_nesting >= 3))


def function_features(fns):
    f = fns.copy()
    f["pen"] = function_penalty(f)
    f["complex_lines"] = np.where(f.cognitive > 15, f.lines, 0)
    f["troubled_lines"] = np.where(troubled(f), f.lines, 0)
    f["troubled"] = troubled(f).astype(int)
    g = f.groupby("path")
    return pd.DataFrame({
        "n_funcs": g.size(),
        "max_cog": g.cognitive.max(),
        "sum_cog": g.cognitive.sum(),
        "max_fn_lines": g.lines.max(),
        "max_ast_nest": g.max_nesting.max(),
        "max_params": g.params.max(),
        "n_bumpy": g.bumps.apply(lambda b: int((b >= 2).sum())),
        "complex_fn_lines": g.complex_lines.sum(),
        "fn_pen_sum": g.pen.sum(),
        "fn_pen_max": g.pen.max(),
        "troubled_lines": g.troubled_lines.sum(),
        "troubled_fns": g.troubled.sum(),
    }).reset_index()


def function_penalty(f):
    """Iteration 2: what one function costs. Each smell is a soft ramp from
    its threshold, in log steps so a function twice as far over costs one
    step more, not twice as much."""
    def over(x, t):
        return np.log2(1 + np.maximum(0, x - t) / t)
    return (1.0 * over(f.cognitive, 15)
            + 0.5 * over(f.lines, 70)
            + 0.5 * np.maximum(0, f.max_nesting - 3)
            + 0.5 * np.maximum(0, f.params - 5)
            + 0.5 * np.maximum(0, f.bumps - 1))


# ---------------------------------------------------------------- formulas

def h0_current(df):
    """The engine today (extensions/codesmells/extension.go healthOf)."""
    js = df.ext.isin([".js", ".jsx", ".ts", ".tsx"])
    mt = np.where(js, 6, 4)
    at = np.where(js, 2.5, 1.5)
    size = np.where(df.lines > 500, np.minimum(3, (df.lines - 500) * 0.01), 0)
    mx = np.where(df.ws_max > mt, np.minimum(3, (df.ws_max - mt) * 0.5), 0)
    av = np.where(df.ws_avg > at, np.minimum(3, (df.ws_avg - at) * 1.5), 0)
    return np.maximum(1, 10 - size - mx - av)


def calibrate(all_files):
    """The language's own norm, from every repo's production files: the
    median average indentation of code lines is where a method body
    normally sits."""
    prod = all_files[~all_files.test]
    return prod.groupby("lang").code_avg.median().to_dict()


def soft(d):
    """Deductions to a 1–10 score without a floor that ties the worst."""
    return 1 + 9 * np.exp(-d / 8)


def h1_whitespace(df, base):
    """Iteration 1: today's inputs, made continuous and fair across languages.
    Size from 150 code lines in log steps; average nesting over the
    language's own median; the lines sitting three levels past it."""
    b = df.lang.map(base).fillna(1.5)
    code = df.code_lines.clip(lower=1)
    size = 1.2 * np.log2(np.maximum(code, 150) / 150)
    avg = 2.0 * np.maximum(0, df.code_avg - b - 0.25)
    level = (b.round() + 3).clip(3, 8).astype(int)
    deep = np.choose(level - 3, [df.deep3, df.deep4, df.deep5, df.deep6, df.deep7, df.deep8])
    deep_pen = 1.5 * np.log2(1 + deep / 10)
    return soft(size + avg + deep_pen)


def h2_functions(df):
    """Iteration 2: function-level smells from the syntax tree, plus file size
    and duplication. The worst function counts in full, the rest compressed."""
    code = df.code_lines.clip(lower=1)
    size = 1.0 * np.log2(np.maximum(code, 300) / 300)
    fn = df.fn_pen_max + np.log2(1 + np.maximum(0, df.fn_pen_sum - df.fn_pen_max))
    dup = 2.0 * np.log2(1 + df.dup_lines / 50)
    return soft(size + 1.5 * fn + dup)


def size_points(df):
    """Iteration 3: two points per doubling past 150 code lines. Each
    doubling of a file roughly doubles its future fixes (x2.2 dev, x2.8
    test), so a point is about x1.5 the fix rate, and red (< 4) starts at
    1,200 code lines."""
    return 2.0 * np.log2(np.maximum(df.code_lines, 150) / 150)


def h3_size(df):
    return np.maximum(1, 10 - size_points(df))


def function_points(df):
    """The worst function's cognitive complexity past 15 (Sonar's line), half
    a point per doubling: diagnosis more than prediction."""
    return 0.5 * np.log2(np.maximum(df.max_cog, 15) / 15)


def h3_size_fn(df):
    return np.maximum(1, 10 - size_points(df) - function_points(df))


def h4_points(df):
    """Round 4, the decision: three deductions from the code alone.
    Size: 1 point per doubling past 150 code lines. Imports: 1.5 per doubling
    past 10. Complex code: 1 per doubling of (1 + lines in functions whose
    cognitive complexity passes 15, over 25)."""
    return pd.DataFrame({
        "size": np.log2(np.maximum(df.code_lines, 150) / 150),
        "coupling": 1.5 * np.log2(np.maximum(df.imports, 10) / 10),
        "complex code": np.log2(1 + df.complex_fn_lines / 25),
    }, index=df.index)


def h4(df):
    return np.maximum(1, 10 - h4_points(df).sum(axis=1))


def hotspot(df):
    """Not health: the Hotspots view's size x churn, read before T0."""
    return -np.log2(df.code_lines.clip(lower=1)) * np.log2(df.past_commits + 1) - 1e-6 * df.code_lines


FORMULAS = {"h0 current": lambda df, base: h0_current(df),
            "h1 whitespace": h1_whitespace,
            "h2 functions": lambda df, base: h2_functions(df),
            "h3 size": lambda df, base: h3_size(df),
            "h3 size+fn": lambda df, base: h3_size_fn(df),
            "h4 decided": lambda df, base: h4(df),
            "ref hotspot": lambda df, base: hotspot(df)}


# ---------------------------------------------------------------- metrics

def spearman(a, b):
    a, b = pd.Series(a).rank(), pd.Series(b).rank()
    if a.std() == 0 or b.std() == 0:
        return float("nan")
    return float(np.corrcoef(a, b)[0, 1])


def partial(x, y, z):
    """Spearman of x and y with z held constant."""
    rxy, rxz, ryz = spearman(x, y), spearman(x, z), spearman(y, z)
    if any(math.isnan(v) for v in (rxy, rxz, ryz)) or abs(rxz) >= 1 or abs(ryz) >= 1:
        return float("nan")
    return (rxy - rxz * ryz) / math.sqrt((1 - rxz ** 2) * (1 - ryz ** 2))


def tiebreak(paths):
    return paths.map(lambda p: zlib.crc32(p.encode()))


def evaluate(df, score):
    d = df.assign(score=score)
    d = d[~d.test]
    bad = -d.score
    res = {"n": len(d), "fixed": int((d.fixes > 0).sum()), "rho": spearman(bad, d.fixes)}
    res["rho|size"] = partial(bad, d.fixes, d.code_lines)
    busy = d.commits >= 3
    res["fixshare"] = spearman(bad[busy], (d.fixes / d.commits.clip(lower=1))[busy])
    order = d.assign(tb=tiebreak(d.path)).sort_values(["score", "tb"])
    budget = 0.2 * order.code_lines.sum()
    seen = order.code_lines.cumsum() <= budget
    res["recall20"] = order.fixes[seen].sum() / max(1, order.fixes.sum())
    red, green = d[d.score < 4], d[d.score >= 8]
    dens = lambda g: 1000 * g.fixes.sum() / max(1, g.code_lines.sum())
    res["red/green"] = dens(red) / dens(green) if dens(green) > 0 and len(red) else float("nan")
    res["at10"] = float((d.score >= 9.95).mean())
    res["red"] = float((d.score < 4).mean())
    return res


def report(repos, names, base, label):
    rows = []
    for fname, f in FORMULAS.items():
        per = [evaluate(repos[n], f(repos[n], base)) for n in names if n in repos]
        per = [p for p in per if p["fixed"] >= 20]
        avg = {k: np.nanmean([p[k] for p in per]) for k in ["rho", "fixshare", "recall20", "red/green", "at10", "red"]}
        rows.append({"formula": fname, "repos": len(per), **avg})
    print(f"\n== {label}")
    print(pd.DataFrame(rows).round(3).to_string(index=False))


def per_repo(repos, names, base):
    rows = []
    for n in names:
        if n not in repos:
            continue
        for fname, f in FORMULAS.items():
            r = evaluate(repos[n], f(repos[n], base))
            rows.append({"repo": n[:14], "formula": fname, **{k: r[k] for k in ["n", "fixed", "rho", "fixshare", "recall20", "at10"]}})
    print(pd.DataFrame(rows).round(3).to_string(index=False))


def explore(repos, names):
    """Which raw measurement knows something beyond size, repo by repo."""
    feats = ["code_lines", "ws_avg", "code_avg", "ws_max", "deep4", "deep5", "dup_lines", "n_funcs", "max_cog", "sum_cog",
             "max_fn_lines", "max_ast_nest", "max_params", "n_bumpy", "complex_fn_lines", "fn_pen_max", "fn_pen_sum", "comment_lines",
             "troubled_lines", "troubled_share", "imports"]
    rows = []
    for n in names:
        if n not in repos:
            continue
        d = repos[n][~repos[n].test]
        row = {"repo": n[:12]}
        for f in feats:
            row[f] = spearman(d[f], d.fixes) if f == "code_lines" else partial(d[f], d.fixes, d.code_lines)
        rows.append(row)
    t = pd.DataFrame(rows).set_index("repo").T
    t["mean"] = t.mean(axis=1)
    print("\n== Spearman with future fixes, size partialled out (code_lines: raw)")
    print(t.round(3).to_string())


def parity(repos, base):
    """A score should not be lower for a language for being that language."""
    allf = pd.concat(repos.values())
    allf = allf[~allf.test]
    rows = []
    for fname, f in FORMULAS.items():
        s = pd.Series(f(allf, base), index=allf.index)
        for lang, g in allf.groupby("lang"):
            rows.append({"formula": fname, "lang": lang, "n": len(g), "median": s[g.index].median(), "p10": s[g.index].quantile(0.1),
                         "share_red": (s[g.index] < 4).mean()})
    print("\n== score by language (production files)")
    print(pd.DataFrame(rows).round(2).pivot(index="lang", columns="formula", values=["median", "p10"]).to_string())


def bands(repos, names, base):
    """Fix rate per file in each band, relative to green, per formula."""
    rows = []
    for fname in ("h0 current", "h3 size+fn", "h4 decided"):
        rel = []
        for n in names:
            d = repos[n][~repos[n].test].copy()
            d["s"] = FORMULAS[fname](d, base)
            d["band"] = np.select([d.s >= 8, d.s >= 4], ["green", "yellow"], "red")
            g = d.groupby("band").fixes.mean()
            if g.get("green", 0) > 0:
                edits = d.groupby("band").apply(lambda x: x.fixes.sum() / max(1, x.commits.sum()), include_groups=False)
                rel.append({b: g.get(b, np.nan) / g["green"] for b in ("green", "yellow", "red")} | {f"share {b}": (d.band == b).mean() for b in ("yellow", "red")}
                           | {f"edits {b}": edits.get(b, np.nan) for b in ("green", "yellow", "red")})
        r = pd.DataFrame(rel)
        rows.append({"formula": fname, "yellow x": r.yellow.median(), "red x": r.red.median(), "yellow %": 100 * r["share yellow"].mean(), "red %": 100 * r["share red"].mean(),
                     "fix share of edits g/y/r": "/".join(f"{r[f'edits {b}'].median():.2f}" for b in ("green", "yellow", "red"))})
    print("\n== fixes per file by band, relative to green (median over repos)")
    print(pd.DataFrame(rows).round(2).to_string(index=False))
    allf = pd.concat([repos[n][~repos[n].test] for n in names])
    print("lines vs code lines, mean Spearman with fixes:",
          round(np.mean([spearman(repos[n][~repos[n].test].lines, repos[n][~repos[n].test].fixes) for n in names]), 3),
          round(np.mean([spearman(repos[n][~repos[n].test].code_lines, repos[n][~repos[n].test].fixes) for n in names]), 3))


def rollups(repos, names, base):
    """Directory health from file health, judged by the directory's fixes."""
    rows = []
    for fname in ("h0 current", "h3 size+fn", "h4 decided"):
        f = FORMULAS[fname]
        res = {k: [] for k in ("line-weighted mean", "plain mean", "worst file", "share of code < 6")}
        for n in names:
            d = repos[n][~repos[n].test].copy()
            d["score"] = f(d, base)
            d["dir"] = d.path.str.rsplit("/", n=1).str[0]
            d["w"] = d.score * d.code_lines
            d["weak"] = np.where(d.score < 6, d.code_lines, 0)
            g = d.groupby("dir").agg(files=("path", "size"), code=("code_lines", "sum"), fixes=("fixes", "sum"),
                                    w=("w", "sum"), mean=("score", "mean"), worst=("score", "min"), weak=("weak", "sum"))
            g = g[g.files >= 3]
            if (g.fixes > 0).sum() < 10:
                continue
            for k, bad in (("line-weighted mean", -(g.w / g.code)), ("plain mean", -g["mean"]), ("worst file", -g.worst), ("share of code < 6", g.weak / g.code)):
                res[k].append((spearman(bad, g.fixes), partial(bad, g.fixes, g.code)))
        for k, v in res.items():
            rows.append({"formula": fname, "rollup": k, "rho": np.nanmean([a for a, _ in v])})
    print("\n== directory rollups (directories with 3+ files)")
    print(pd.DataFrame(rows).round(3).to_string(index=False))


if __name__ == "__main__":
    repos = load(sys.argv[1])
    base = calibrate(pd.concat(repos.values()))
    print("language base (median avg indentation):", {k: round(v, 2) for k, v in base.items()})
    if len(sys.argv) > 2 and sys.argv[2] == "explore":
        explore(repos, DEV + TEST)
        sys.exit()
    report(repos, DEV, base, "DEV")
    report(repos, TEST, base, "TEST")
    if len(sys.argv) > 2 and sys.argv[2] == "full":
        per_repo(repos, DEV + TEST, base)
        parity(repos, base)
    rollups(repos, DEV + TEST, base)
    bands(repos, DEV + TEST, base)
