# Strict recall: every supertype marker (extends/implements/base list, captured by the
# engine itself) that names a type declared in the SAME component should be an edge.
from lib import con, has
from collections import defaultdict
import sys
def run(db):
    c = con(db)
    if not has(c, "unit_markers"): print(f"== {db}: no markers"); return
    units = c.execute("select id, name, component, file from units").fetchall()
    by_comp = defaultdict(lambda: defaultdict(list))
    for uid, name, comp, f in units: by_comp[comp][name].append(uid)
    comp_of = {u[0]: u[2] for u in units}
    edges = {(a, b) for a, b in c.execute("select `from`, `to` from unit_connections")}
    same = cross = same_miss = amb = 0; ex = []
    for u, key in c.execute("select unit, key from unit_markers where source='supertype'"):
        simple = key.split("<")[0].split(".")[-1].strip()
        cands = [t for t in by_comp[comp_of.get(u)].get(simple, []) if t != u]
        if not cands: continue
        if len(cands) > 1: amb += 1
        same += 1
        if not any((u, t) in edges for t in cands):
            same_miss += 1
            if len(ex) < 3: ex.append((u.split(".")[-1], simple))
    print(f"== {db}: supertypes naming a same-component type: {same}; with no edge: {same_miss} ({same_miss/max(same,1):.0%}); ambiguous names {amb}; e.g. {ex}")
for db in sys.argv[1:]: run(db)
