from lib import *
from collections import defaultdict
def run(db):
    c, R = con(db), Report(db)
    k = "kind" in cols(c, "component_connections_direct")
    where = "where kind != 'type_only' and `from` != `to`" if k else "where `from` != `to`"
    rows = c.execute(f"select `from`, `to`, file from component_connections_direct {where}").fetchall()
    dependents, dependencies = defaultdict(set), defaultdict(set)
    aff_files, eff_files = defaultdict(set), defaultdict(set)
    for a, b, f in rows:
        dependents[b].add(a); dependencies[a].add(b)
        aff_files[b].add(f); eff_files[a].add(f)
    comp = c.execute("select name, modularity__coupling__dependents, modularity__coupling__dependencies, modularity__coupling__afferent, modularity__coupling__efferent, modularity__instability from components").fetchall()
    for idx, truth, label in ((1, dependents, "dependents = distinct components importing it"),
                              (2, dependencies, "dependencies = distinct components it imports"),
                              (3, aff_files, "afferent = distinct external files importing it"),
                              (4, eff_files, "efferent = distinct own files importing out")):
        bad = [(r[0], r[idx], len(truth[r[0]])) for r in comp if (r[idx] or 0) != len(truth[r[0]])]
        R.check("coupling", label, not bad, f"{len(bad)}/{len(comp)} differ; e.g. {bad[:2]}")
    bad = []
    for r in comp:
        ca, ce = len(aff_files[r[0]]), len(eff_files[r[0]])
        want = ce / (ca + ce) if ca + ce else 0
        if abs((r[5] or 0) - want) > 1e-9: bad.append((r[0], r[5], want))
    R.check("coupling", "instability = Ce/(Ca+Ce) on file counts", not bad, f"{len(bad)} differ; e.g. {bad[:2]}")
    return R
if __name__ == "__main__":
    for db in DBS:
        if db == "sakai": continue
        print(f"== {db}"); run(db)
