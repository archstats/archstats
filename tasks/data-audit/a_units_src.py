# Precision / recall of unit_connections against the source text itself.
from lib import *
import re, random
from collections import defaultdict
random.seed(7)

JAVA_IMPORT = re.compile(r'^\s*import\s+(static\s+)?([\w.]+)(\.\*)?\s*;', re.M)

def run(db):
    c, R = con(db), Report(db)
    if not has(c, "unit_connections"):
        R.check("units", "unit_connections present", None); return R
    content = {r[0]: r[1] for r in c.execute("select file, content from file_contents")}
    unit_file = {r[0]: r[1] for r in c.execute("select id, file from units")}
    unit_name = {r[0]: r[1] for r in c.execute("select id, name from units")}
    edges = c.execute("select `from`, `to`, from_file, to_file from unit_connections").fetchall()
    # Precision: the target's simple name appears in the source file.
    sample = random.sample(edges, min(600, len(edges)))
    miss = []
    for a, b, ff, tf in sample:
        src = content.get(ff)
        if src is None: continue
        nm = unit_name.get(b) or b.split(".")[-1].split("#")[-1]
        if not re.search(r'\b' + re.escape(nm) + r'\b', src): miss.append((ff, nm))
    R.check("units", "precision: target name appears in the source file", not miss, f"{len(miss)}/{len(sample)} sampled edges have no mention; e.g. {miss[:3]}")
    # file consistency
    bad = [(a, ff, unit_file.get(a)) for a, b, ff, tf in edges if a in unit_file and unit_file[a] != ff]
    R.check("units", "from_file = the file the unit is declared in", not bad, f"{len(bad)} differ; e.g. {bad[:2]}")
    bad = [(b, tf, unit_file.get(b)) for a, b, ff, tf in edges if b in unit_file and unit_file[b] != tf]
    R.check("units", "to_file = the file the target is declared in", not bad, f"{len(bad)} differ; e.g. {bad[:2]}")
    dangling = sum(1 for a, b, *_ in edges if a not in unit_file or b not in unit_file)
    R.check("units", "both ends are declared units", dangling == 0, f"{dangling} of {len(edges)}")
    # Recall for Java: explicit single-type imports of an internal type should produce an edge.
    java = [f for f in content if f.endswith(".java")]
    if java:
        fq = {}  # fully-qualified -> unit id
        for uid in unit_file:
            if "." in uid and "#" not in uid and "/" not in uid: fq[uid] = uid
        file_units = defaultdict(set)
        for uid, f in unit_file.items(): file_units[f].add(uid)
        have = defaultdict(set)
        for a, b, ff, tf in edges: have[ff].add(b)
        want = missing = 0; ex = []
        for f in java:
            for m in JAVA_IMPORT.finditer(content[f]):
                if m.group(1) or m.group(3): continue
                target = m.group(2)
                if target in fq and fq[target] not in file_units[f]:
                    want += 1
                    if fq[target] not in have[f]:
                        missing += 1
                        if len(ex) < 3: ex.append((f.split('/')[-1], target))
        R.check("units", "recall: every import of an internal type is an edge", missing == 0, f"{missing}/{want} imports have no edge; e.g. {ex}")
    return R
if __name__ == "__main__":
    import sys
    for db in (sys.argv[1:] or ["broadleaf", "librechat", "gin", "oscar"]):
        print(f"== {db}"); run(db)
