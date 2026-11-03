"""Composite-score experiment: are health, hotspot, complexity and size four
signals or one? And does any combination beat its parts at finding where
trouble lands next (future fix commits, future commits)?

    python3 analyze.py <work-dir>
"""
import os, sys
import numpy as np
import pandas as pd

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import score as S

W = sys.argv[1]
import sqlite3
repos = {k: v[~v.test].copy() for k, v in S.load(W).items()}
repos = {k: v for k, v in repos.items() if (v.fixes > 0).sum() >= 20}


def scc_sizes(edges):
    """File -> size of its strongly connected group (Tarjan, iterative)."""
    adj = {}
    for a, b in edges:
        adj.setdefault(a, []).append(b)
        adj.setdefault(b, [])
    index, low, on, stack, out, n = {}, {}, set(), [], {}, 0
    for root in adj:
        if root in index:
            continue
        work = [(root, 0)]
        while work:
            v, i = work.pop()
            if i == 0:
                index[v] = low[v] = n; n += 1; stack.append(v); on.add(v)
            nbrs = adj[v]
            if i < len(nbrs):
                work.append((v, i + 1))
                w = nbrs[i]
                if w not in index:
                    work.append((w, 0))
                elif w in on:
                    low[v] = min(low[v], index[w])
                continue
            if low[v] == index[v]:
                comp = []
                while True:
                    w = stack.pop(); on.discard(w); comp.append(w)
                    if w == v:
                        break
                for w in comp:
                    out[w] = len(comp)
            if work:
                u = work[-1][0]
                low[u] = min(low[u], low[v])
    return out


def structure(name):
    db = os.path.join(W, name, "engine.db")
    if not os.path.exists(db):
        return None, None
    con = sqlite3.connect(db)
    strip = lambda s: s.str.replace(r"^\./", "", regex=True)
    e = pd.read_sql("select distinct from_file, to_file from unit_connections where from_file != '' and to_file != '' and from_file != to_file", con)
    e["from_file"], e["to_file"] = strip(e.from_file), strip(e.to_file)
    u = pd.read_sql("select file, component, used_by_components, page_rank from units where file != ''", con)
    u["file"] = strip(u.file)
    comps = pd.read_sql("select name as component, modularity__coupling__afferent as afferent, modularity__coupling__efferent as efferent, cycles__short__count as cycles, graph__page_rank as pagerank, graph__betweenness as betweenness from components", con)
    try:
        g = pd.read_sql("select component, group_size from component_strongly_connected_groups", con)
        comps = comps.merge(g, on="component", how="left")
    except Exception:
        comps["group_size"] = 1
    con.close()
    f = pd.DataFrame({"fan_in": e.groupby("to_file").from_file.nunique()})
    f["fan_out"] = e.groupby("from_file").to_file.nunique()
    sc = scc_sizes(list(zip(e.from_file, e.to_file)))
    f["scc"] = pd.Series(sc)
    gu = u.groupby("file")
    f = f.join(pd.DataFrame({"used_by_components": gu.used_by_components.max(), "component": gu.component.agg(lambda c: c.mode().iat[0])}), how="outer")
    return f.fillna({"fan_in": 0, "fan_out": 0, "scc": 1, "used_by_components": 0}), comps


STRUCT, COMPS = {}, {}
for n in list(repos):
    f, c = structure(n)
    if f is not None:
        repos[n] = repos[n].merge(f, left_on="path", right_index=True, how="left").fillna({"fan_in": 0, "fan_out": 0, "scc": 1, "used_by_components": 0})
        COMPS[n] = c


def pct(x):
    return pd.Series(x).rank(pct=True).values


def features(d):
    f = pd.DataFrame(index=d.index)
    f["size"] = d.code_lines
    f["complexity"] = d.complex_fn_lines + d.max_cog / 1000  # tie-break on worst function
    f["unhealth"] = 10 - S.h4(d)
    f["churn"] = d.past_commits
    f["hotspot"] = np.log2(d.code_lines.clip(lower=1)) * np.log2(d.past_commits + 1)
    f["past_fixes"] = d.past_fixes
    if "fan_in" in d:
        f["fan_in"] = d.fan_in
        f["cross_comp_users"] = d.used_by_components
        f["in_cycle"] = (d.scc > 1).astype(float)
    return f


def candidates(d, f):
    c = {}
    c["size"] = f["size"]
    c["complexity"] = f["complexity"]
    c["unhealth (h4)"] = f["unhealth"] + d.code_lines / 1e7
    c["hotspot (size x churn)"] = f["hotspot"]
    c["churn"] = f["churn"] + d.code_lines / 1e7
    c["past fixes"] = f["past_fixes"] + d.past_commits / 1e4
    # The proposal: four equally weighted percentile ranks.
    c["4-way rank mean"] = (pct(f["size"]) + pct(f["complexity"]) + pct(f["unhealth"]) + pct(f["hotspot"])) / 4
    # Two orthogonal axes: how bad the code is x how often it is paid for.
    c["unhealth x churn"] = f["unhealth"] * np.log2(f["churn"] + 1) + f["unhealth"] / 100
    c["unhealth x churn (geo-rank)"] = np.sqrt(pct(f["unhealth"]) * pct(f["churn"]))
    c["complexity x churn"] = np.log2(1 + d.complex_fn_lines / 25) * np.log2(f["churn"] + 1) + f["churn"] / 1e4
    c["unhealth x churn + past fixes"] = pct(c["unhealth x churn"]) + pct(f["past_fixes"]) / 2
    if "fan_in" in f:
        c["fan-in"] = f["fan_in"] + d.code_lines / 1e7
        c["cross-component users"] = f["cross_comp_users"] + d.code_lines / 1e7
        c["in a file cycle"] = f["in_cycle"] + d.code_lines / 1e7
        c["unhealth x churn + fan-in"] = pct(c["unhealth x churn"]) + pct(f["fan_in"]) / 2
    return c


def evaluate(d, score):
    s = pd.Series(np.asarray(score, dtype=float), index=d.index)
    r = {}
    r["rho fixes"] = S.spearman(s, d.fixes)
    r["rho changes"] = S.spearman(s, d.commits)
    busy = d.commits >= 3
    r["fixshare"] = S.spearman(s[busy], (d.fixes / d.commits.clip(lower=1))[busy])
    order = d.assign(s=s, tb=S.tiebreak(d.path)).sort_values(["s", "tb"], ascending=[False, True])
    budget = 0.2 * order.code_lines.sum()
    seen = order.code_lines.cumsum() <= budget
    r["fixes@20% lines"] = order.fixes[seen].sum() / max(1, order.fixes.sum())
    top = order.head(20)
    r["top20 fixes share"] = top.fixes.sum() / max(1, order.fixes.sum())
    r["top20 lines share"] = top.code_lines.sum() / max(1, order.code_lines.sum())
    r["top20 hit (>=3 fixes)"] = float((top.fixes >= 3).mean())
    return r


def table(rows):
    return pd.DataFrame(rows).groupby("candidate", sort=False).mean(numeric_only=True).round(3)


# ---------------------------------------------------------------- 1. redundancy
print("== Corpus")
for n, d in repos.items():
    print(f"{n:22s} files {len(d):6d}  fix-touched {(d.fixes > 0).sum():5d}  fixes {int(d.fixes.sum()):6d}")

mats = []
for n, d in repos.items():
    f = features(d).drop(columns=["in_cycle"], errors="ignore")
    mats.append(f.rank().corr())
print("\n== Spearman between the inputs, mean over repos")
print(pd.concat(mats).groupby(level=0).mean().loc[mats[0].index, mats[0].columns].round(2).to_string())

# What each input knows beyond size: partial Spearman with future fixes.
rows = []
for n, d in repos.items():
    f = features(d)
    for k in [k for k in ["complexity", "unhealth", "churn", "hotspot", "past_fixes", "fan_in", "cross_comp_users", "in_cycle"] if k in f]:
        rows.append({"input": k, "repo": n, "rho|size": S.partial(f[k], d.fixes, d.code_lines)})
print("\n== Spearman with future fixes, size held constant (mean over repos)")
print(pd.DataFrame(rows).groupby("input")["rho|size"].agg(["mean", "min", "max"]).round(3).to_string())

# ---------------------------------------------------------------- 2. files
rows = []
for n, d in repos.items():
    f = features(d)
    for k, s in candidates(d, f).items():
        rows.append({"candidate": k, "repo": n, **evaluate(d, s)})
sz = []
for n, d in repos.items():
    f = features(d); c = candidates(d, f)
    top = lambda x: set(pd.Series(np.asarray(x, float), index=d.path.values).sort_values(ascending=False).head(20).index)
    for k, v in c.items():
        sz.append({"candidate": k, "rho with size": S.spearman(v, d.code_lines),
                   "top20 shared with size": len(top(v) & top(c["size"])) / 20,
                   "top20 shared with 4-way": len(top(v) & top(c["4-way rank mean"])) / 20})
print("\n== How much is each candidate just size?")
print(pd.DataFrame(sz).groupby("candidate", sort=False).mean().round(2).to_string())
print("\n== File ranking against the next two years (mean over repos)")
print(table(rows).to_string())
rr = pd.DataFrame(rows)
print("\n== rho fixes per repo")
print(rr.pivot(index="candidate", columns="repo", values="rho fixes").round(2).to_string())


# ---------------------------------------------------------------- 3. directories
def dir_level(d):
    f = features(d)
    d = d.assign(dir=d.path.str.rsplit("/", n=1).str[0], **{k: f[k] for k in f})
    d["interest"] = f["unhealth"] * np.log2(f["churn"] + 1)
    d["w_unhealth"] = f["unhealth"] * d.code_lines
    g = d.groupby("dir")
    a = pd.DataFrame({
        "files": g.size(), "code_lines": g.code_lines.sum(),
        "fixes": g.fixes.sum(), "commits": g.commits.sum(),
        "size": g.code_lines.sum(),
        "unhealth mean (lines)": g.w_unhealth.sum() / g.code_lines.sum().clip(lower=1),
        "unhealth worst": g.unhealth.max(),
        "hotspot max (today)": g.hotspot.max(),
        "churn sum": g.churn.sum(),
        "past fixes sum": g.past_fixes.sum(),
        "interest sum": g.interest.sum(),
        "interest max": g.interest.max(),
    })
    return a[a.files >= 2]


rows = []
for n, d in repos.items():
    a = dir_level(d).reset_index().rename(columns={"dir": "path"})
    for k in ["size", "unhealth mean (lines)", "unhealth worst", "hotspot max (today)", "churn sum",
              "past fixes sum", "interest sum", "interest max"]:
        s = a[k] + a.code_lines / 1e9
        a2 = a.assign(test=False)
        rows.append({"candidate": k, "repo": n, **evaluate(a2, s)})
print("\n== Directory ranking (dirs with 2+ production files), mean over repos")
print(table(rows)[["rho fixes", "rho changes", "fixes@20% lines", "top20 fixes share", "top20 lines share"]].to_string())

# ---------------------------------------------------------------- 4. concentration
print("\n== How concentrated are future fixes?")
for n, d in repos.items():
    fx = d.fixes.sort_values(ascending=False)
    a = dir_level(d).fixes.sort_values(ascending=False)
    print(f"{n:22s} top 1% files {fx.head(max(1, len(fx)//100)).sum()/max(1,fx.sum()):.0%}  "
          f"top 5% files {fx.head(max(1, len(fx)//20)).sum()/max(1,fx.sum()):.0%}  "
          f"top 5% dirs {a.head(max(1, len(a)//20)).sum()/max(1,a.sum()):.0%}")


# ---------------------------------------------------------------- 5. engine components
rows = []
for n, d in repos.items():
    if n not in COMPS or "component" not in d:
        continue
    f = features(d)
    d = d.assign(**{k: f[k] for k in ["unhealth", "churn", "past_fixes"]})
    d["interest"] = f["unhealth"] * np.log2(f["churn"] + 1)
    d["w_unhealth"] = f["unhealth"] * d.code_lines
    g = d.dropna(subset=["component"]).groupby("component")
    a = pd.DataFrame({"files": g.size(), "code_lines": g.code_lines.sum(), "fixes": g.fixes.sum(), "commits": g.commits.sum(),
                      "unhealth mean": g.w_unhealth.sum() / g.code_lines.sum().clip(lower=1), "unhealth worst": g.unhealth.max(),
                      "interest sum": g.interest.sum(), "churn sum": g.churn.sum()}).reset_index()
    a = a.merge(COMPS[n], on="component", how="left").fillna(0)
    a = a[a.files >= 2].rename(columns={"component": "path"}).assign(test=False)
    pr = lambda x: a[x].rank(pct=True)
    cands = {"size": a.code_lines, "unhealth mean": a["unhealth mean"], "unhealth worst": a["unhealth worst"],
             "interest sum": a["interest sum"], "churn sum": a["churn sum"],
             "afferent (dependents)": a.afferent, "page rank": a.pagerank, "betweenness": a.betweenness,
             "short cycles": a.cycles, "in a component cycle": (a.group_size > 1).astype(float),
             "interest + afferent": pr("interest sum") + pr("afferent"),
             "interest + cycles": pr("interest sum") + pr("cycles")}
    for k, v in cands.items():
        rows.append({"candidate": k, "repo": n, "partial|size": S.partial(v, a.fixes, a.code_lines), **evaluate(a, v + a.code_lines / 1e12)})
print("\n== Engine components (2+ production files), mean over repos")
print(table(rows)[["rho fixes", "partial|size", "rho changes", "fixes@20% lines", "top20 fixes share", "top20 lines share"]].to_string())
