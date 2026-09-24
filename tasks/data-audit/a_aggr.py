from lib import *
import math
def run(db):
    c = con(db); s = summary(c); fc = set(cols(c, "files"))
    print(f"== {db}")
    for k, v in sorted(s.items()):
        if k not in fc: continue
        try: v = float(v)
        except (TypeError, ValueError): print(f"  {k:48} non-numeric {v!r}"); continue
        r = c.execute(f"select sum(`{k}`), max(`{k}`), min(`{k}`), avg(`{k}`), count(`{k}`) from files").fetchone()
        cand = {"sum": r[0], "max": r[1], "min": r[2], "avg(files)": r[3]}
        if k.endswith("__avg"):
            base = k[:-5]
            if base + "__count" in fc:
                w = c.execute(f"select sum(`{k}` * `{base}__count`) / sum(`{base}__count`) from files").fetchone()[0]
                cand["count-weighted avg"] = w
        match = [n for n, x in cand.items() if x is not None and math.isclose(float(x), v, rel_tol=1e-6, abs_tol=1e-9)]
        print(f"  {k:48} {v:>14.4f}  = {', '.join(match) if match else 'NONE of ' + str({n: round(float(x),3) for n, x in cand.items() if x is not None})}")
for db in ("broadleaf", "librechat"):
    run(db)
