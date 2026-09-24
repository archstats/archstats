from lib import con, cols, DBS
import math, os
from collections import defaultdict
def health(lines, mx, avg, ext):
    h = 10.0
    if lines > 500: h -= min(3.0, (lines - 500) * 0.01)
    mt, at = (6, 2.5) if ext in (".js", ".jsx", ".ts", ".tsx") else (4, 1.5)
    if mx > mt: h -= min(3.0, (mx - mt) * 0.5)
    if avg > at: h -= min(3.0, (avg - at) * 1.5)
    return max(1.0, h)
def run(db):
    c = con(db); fc = set(cols(c, "files"))
    if "codesmells__code_health" not in fc: print(f"== {db}: no codesmells"); return
    has_git = "git__commits__total" in fc
    rows = c.execute(f"""select name, component, directory, complexity__lines, complexity__indentation__max, complexity__indentation__avg,
        complexity__indentation__volatility, {'git__commits__total' if has_git else '0'}, codesmells__code_health, codesmells__hotspot_score,
        codesmells__bumpy_road, codesmells__static_complexity_score from files""").fetchall()
    scored = [r for r in rows if r[8] is not None]
    raw = {r[0]: math.log2((r[7] or 0) + 1) * (r[3] or 0) for r in scored}
    mx = max(raw.values()) if raw else 0
    bad = defaultdict(list)
    for r in scored:
        ext = os.path.splitext(r[0])[1].lower()
        if abs(health(r[3] or 0, r[4] or 0, r[5] or 0, ext) - r[8]) > 1e-6: bad["health"].append(r[0])
        want_h = (raw[r[0]] * 100 / mx) if (r[7] or 0) > 0 and mx > 0 else 0
        if abs(want_h - (r[9] or 0)) > 1e-6: bad["hotspot"].append(r[0])
        if abs(((r[6] or 0) / r[3] if r[3] else 0) - (r[10] or 0)) > 1e-6: bad["bumpy"].append(r[0])
        if abs((r[3] or 0) * (1 + min(8.0, r[5] or 0)) - (r[11] or 0)) > 1e-6: bad["static"].append(r[0])
    print(f"== {db}: {len(scored)}/{len(rows)} files scored; per-file mismatches: " + ", ".join(f"{k} {len(bad[k])}" for k in ("health", "hotspot", "bumpy", "static")))
    # roll-ups
    for table, key in (("components", 1), ("directories", 2)):
        tc = set(cols(c, table))
        if "codesmells__code_health" not in tc: continue
        groups = defaultdict(list)
        for r in scored:
            if table == "components":
                groups[r[key]].append(r)
            else:
                # A directory holds its subdirectories' files too.
                parts = r[0].split("/")[:-1]
                for i in range(1, len(parts) + 1): groups["/".join(parts[:i])].append(r)
        view = {n: (h, hs, b, s) for n, h, hs, b, s in c.execute(f"select name, codesmells__code_health, codesmells__hotspot_score, codesmells__bumpy_road, codesmells__static_complexity_score from {table}")}
        tally = defaultdict(lambda: defaultdict(int))
        for g, fs in groups.items():
            if g not in view: continue
            v = view[g]; L = [max(1, f[3] or 0) for f in fs]
            cand = {
              "health": {"line-weighted": sum(f[8] * l for f, l in zip(fs, L)) / sum(L), "mean": sum(f[8] for f in fs) / len(fs)},
              "hotspot": {"max": max(f[9] for f in fs), "mean": sum(f[9] for f in fs) / len(fs)},
              "bumpy": {"line-weighted": sum(f[10] * l for f, l in zip(fs, L)) / sum(L), "mean": sum(f[10] for f in fs) / len(fs)},
              "static": {"sum": sum(f[11] or 0 for f in fs)},
            }
            for i, m in enumerate(("health", "hotspot", "bumpy", "static")):
                hit = [k for k, x in cand[m].items() if v[i] is not None and math.isclose(v[i], x, rel_tol=1e-6, abs_tol=1e-9)]
                tally[m][hit[0] if hit else "NONE"] += 1
        print(f"   {table:11} " + "; ".join(f"{m}: {dict(t)}" for m, t in tally.items()))
if __name__ == "__main__":
    for db in ("broadleaf", "librechat", "oscar", "gin", "nopcommerce", "elepy", "exposed"):
        run(db)
