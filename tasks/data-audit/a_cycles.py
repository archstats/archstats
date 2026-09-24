from lib import *
from a_graph import runtime_edges, bfs
from collections import defaultdict
def run(db):
    c, R = con(db), Report(db)
    if not has(c, "component_cycles_shortest") or not c.execute("select count(*) from component_cycles_shortest").fetchone()[0]:
        R.check("cycles", "present", None); return R
    E = runtime_edges(c); adj = defaultdict(set)
    for a, b in E: adj[a].add(b)
    nodes = {r[0] for r in c.execute("select name from components")}
    dist = {s: bfs(adj, s) for s in nodes}
    girth = {}
    for s in nodes:
        m = min((dist[t][s] + 1 for t in adj[s] if s in dist[t]), default=None)
        if m: girth[s] = m
    best = defaultdict(lambda: 10**9)
    for r in c.execute("select component, cycle_size from component_cycles_shortest"):
        best[r[0]] = min(best[r[0]], r[1])
    miss = [(s, g, best.get(s)) for s, g in girth.items() if best.get(s) != g]
    R.check("cycles", "each knotted component's shortest cycle is in the set", not miss, f"{len(miss)}/{len(girth)} not; e.g. {miss[:3]}")
    # component metrics
    per = defaultdict(list)
    for r in c.execute("select cycle_nr, component, cycle_size from component_cycles_shortest"): per[r[1]].append(r[2])
    comp = c.execute("select name, cycles__short__count, cycles__short__avg, cycles__short__max from components").fetchall()
    bad = [(r[0], r[1], len(per[r[0]])) for r in comp if (r[1] or 0) != len(per[r[0]])]
    R.check("cycles", "components.cycles__short__count = cycles it is in", not bad, f"{len(bad)} differ; e.g. {bad[:2]}")
    bad = [(r[0], r[3], max(per[r[0]], default=0)) for r in comp if (r[3] or 0) != max(per[r[0]], default=0)]
    R.check("cycles", "cycles__short__max = largest of those", not bad, f"{len(bad)} differ; e.g. {bad[:2]}")
    # git_component_cycles_shortest_shared_commits covers the same cycles?
    if has(c, "git_component_cycles_shortest_shared_commits"):
        g = {r[0] for r in c.execute("select cycle from git_component_cycles_shortest_shared_commits")}
        v = {r[0] for r in c.execute("select distinct cycle from component_cycles_shortest")}
        R.check("cycles", "git shared-commit cycles = the cycle set", g == v, f"git {len(g)} vs view {len(v)}; only-git {len(g-v)} only-view {len(v-g)}")
    return R
if __name__ == "__main__":
    for db in DBS:
        if db == "sakai": continue
        print(f"== {db}"); run(db)
