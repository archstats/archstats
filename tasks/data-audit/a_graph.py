from lib import *
from collections import deque, defaultdict

def runtime_edges(c):
    k = "kind" in cols(c, "component_connections_direct")
    where = "where kind != 'type_only'" if k else ""
    E = set()
    for r in c.execute(f"select `from`, `to` from component_connections_direct {where}"):
        if r[0] != r[1]:
            E.add((r[0], r[1]))
    return E

def bfs(adj, s):
    d = {s: 0}; q = deque([s])
    while q:
        u = q.popleft()
        for v in adj[u]:
            if v not in d:
                d[v] = d[u] + 1; q.append(v)
    return d

def tarjan(nodes, adj):
    index, low, on, st, out, i = {}, {}, set(), [], [], [0]
    import sys; sys.setrecursionlimit(100000)
    def sc(v):
        index[v] = low[v] = i[0]; i[0] += 1; st.append(v); on.add(v)
        for w in adj[v]:
            if w not in index: sc(w); low[v] = min(low[v], low[w])
            elif w in on: low[v] = min(low[v], index[w])
        if low[v] == index[v]:
            comp = []
            while True:
                w = st.pop(); on.discard(w); comp.append(w)
                if w == v: break
            out.append(frozenset(comp))
    for v in nodes:
        if v not in index: sc(v)
    return out

def run(db):
    c, R = con(db), Report(db)
    E = runtime_edges(c)
    nodes = {r[0] for r in c.execute("select name from components")}
    adj = defaultdict(set)
    for a, b in E: adj[a].add(b)
    R.check("graph", "every edge endpoint is a known component", all(a in nodes and b in nodes for a, b in E),
            f"{sum(1 for a,b in E if a not in nodes or b not in nodes)} dangling of {len(E)}")

    # Indirect
    dist = {s: bfs(adj, s) for s in nodes}
    pairs = {(s, t): d for s in nodes for t, d in dist[s].items() if t != s}
    ind = defaultdict(list)
    for r in c.execute("select `from`, `to`, shortest_path_length, shortest_path from component_connections_indirect"):
        ind[(r[0], r[1])].append((r[2], r[3]))
    R.check("indirect", "reachable pairs = BFS reachability", set(ind) == set(pairs),
            f"view {len(ind)} pairs, BFS {len(pairs)}; only in view {len(set(ind)-set(pairs))}, only in BFS {len(set(pairs)-set(ind))}")
    off = [(p, ind[p][0][0], pairs[p]) for p in ind if p in pairs and ind[p][0][0] != pairs[p]]
    plus1 = [(p, ind[p][0][0], pairs[p]) for p in ind if p in pairs and ind[p][0][0] == pairs[p] + 1]
    R.check("indirect", "shortest_path_length = hops", len(off) == 0,
            f"{len(off)} of {len(ind)} differ; {len(plus1)} are exactly hops+1 (counts nodes, not edges)")
    multi = sum(1 for v in ind.values() if len(v) > 1)
    R.check("indirect", "one row per pair", multi == 0, f"{multi} pairs have several rows (tied shortest paths); total rows {sum(len(v) for v in ind.values())}")
    badpath = 0
    for p, rows in ind.items():
        for _, sp in rows:
            hop = sp.split(" -> ")
            if hop[0] != p[0] or hop[-1] != p[1] or any((hop[i], hop[i+1]) not in E for i in range(len(hop)-1)):
                badpath += 1
    R.check("indirect", "every shortest_path walks real edges", badpath == 0, f"{badpath} invalid paths")

    # SCC
    sccs = [s for s in tarjan(sorted(nodes), adj) if len(s) > 1]
    if has(c, "component_strongly_connected_groups"):
        g = defaultdict(set)
        for r in c.execute("select `group`, component from component_strongly_connected_groups"): g[r[0]].add(r[1])
        view = {frozenset(v) for v in g.values() if len(v) > 1}
        R.check("scc", "strongly connected groups = Tarjan", view == set(sccs),
                f"view {len(view)} multi-member groups, Tarjan {len(sccs)}; sizes view {sorted(len(x) for x in view)[-3:]} tarjan {sorted(len(x) for x in sccs)[-3:]}")

    # Shortest cycles
    if has(c, "component_cycles_shortest") and c.execute("select count(*) from component_cycles_shortest").fetchone()[0]:
        cyc = defaultdict(list)
        for r in c.execute("select cycle_nr, component, cycle_size, cycle from component_cycles_shortest"):
            cyc[r[0]].append(r)
        invalid = wrongsize = notshortest = 0
        for nr, rows in cyc.items():
            seq = rows[0][3].split(" -> ")
            ring = seq if seq[0] != seq[-1] else seq[:-1]
            edges_ok = all((ring[i], ring[(i+1) % len(ring)]) in E for i in range(len(ring)))
            if not edges_ok: invalid += 1
            if rows[0][2] != len(ring): wrongsize += 1
            # shortest cycle through its first component
            s = ring[0]
            best = min((dist[t].get(s, 10**9) + 1 for t in adj[s] if s in dist[t]), default=None) if False else None
        # shortest cycle length through each component = 1 + min over successors t of dist(t -> s)
        def girth_through(s):
            m = None
            for t in adj[s]:
                d = dist[t].get(s)
                if d is not None and (m is None or d + 1 < m): m = d + 1
            return m
        for nr, rows in cyc.items():
            for r in rows:
                g_ = girth_through(r[1])
                if g_ is not None and r[2] != g_: notshortest += 1
        R.check("cycles", "each reported cycle walks real edges", invalid == 0, f"{invalid} of {len(cyc)} cycles break")
        R.check("cycles", "cycle_size = components in the cycle", wrongsize == 0, f"{wrongsize} of {len(cyc)}")
        R.check("cycles", "cycle_size = shortest cycle through that component", notshortest == 0, f"{notshortest} component rows are not the shortest")
        incyc = {r[0] for r in c.execute("select distinct component from component_cycles_shortest")}
        inscc = set().union(*sccs) if sccs else set()
        R.check("cycles", "components in a cycle = components in a multi-member SCC", incyc == inscc,
                f"cycles {len(incyc)}, SCC members {len(inscc)}; missing from cycles {len(inscc-incyc)}")

    # Furthest
    if has(c, "component_connections_furthest"):
        bad = 0; n = 0
        for r in c.execute("select component, furthest_component, furthest_component_distance from component_connections_furthest"):
            n += 1
            d = dist.get(r[0], {})
            far = max((v for k, v in d.items() if k != r[0]), default=0)
            if r[2] not in (far, far + 1): bad += 1
        sample = c.execute("select component, furthest_component_distance, furthest_component_shortest_path from component_connections_furthest limit 1").fetchone()
        R.check("furthest", "distance = max BFS depth (or +1)", bad == 0, f"{bad} of {n}; sample {tuple(sample) if sample else None}")

    # Coupling metrics on components
    cc = cols(c, "components")
    fan_out = {s: len(adj[s]) for s in nodes}
    fan_in = defaultdict(int)
    for a, b in E: fan_in[b] += 1
    for col, truth, label in (("modularity__coupling__efferent", fan_out, "efferent = distinct runtime successors"),
                              ("modularity__coupling__afferent", fan_in, "afferent = distinct runtime predecessors")):
        if col in cc:
            rows = c.execute(f"select name, `{col}` from components").fetchall()
            bad = [(r[0], r[1], truth.get(r[0], 0)) for r in rows if (r[1] or 0) != truth.get(r[0], 0)]
            R.check("components", col.split("__")[-1] + " " + label, not bad, f"{len(bad)} of {len(rows)} differ; e.g. {bad[:3]}")
    return R

if __name__ == "__main__":
    import sys
    for db in (sys.argv[1:] or DBS):
        if db == "sakai": continue
        print(f"== {db}"); run(db)
