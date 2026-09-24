# Kotlin recall/precision of unit_connections (file grain) against source text.
# Expected: file F names a type T declared in exactly one other file G whose package is F's
# own, explicitly imported (pkg.T), or star-imported (pkg.*).
import sqlite3, re, sys, os, random, collections
db, root = sys.argv[1], sys.argv[2]
c = sqlite3.connect(db)
decl = collections.defaultdict(set)
for uid, name, f in c.execute("select id, name, file from units where kind='type' and file like '%.kt'"):
    pkg = uid[: -len(name) - 1] if uid.endswith("." + name) else ""
    decl[name].add((pkg, f))
edges = set(c.execute("select from_file, to_file from unit_connections where from_file like '%.kt' and to_file like '%.kt' and from_file != to_file"))
strip = re.compile(r'//[^\n]*|/\*.*?\*/|""".*?"""|"(?:\\.|[^"\\\n])*"', re.S)
tok = re.compile(r"\b[A-Z][A-Za-z0-9_]*\b")
expected = set(); why = {}
files = [r[0] for r in c.execute("select distinct file from units where file like '%.kt'")]
for f in files:
    try: src = open(os.path.join(root, f[2:] if f.startswith("./") else f), encoding="utf-8", errors="replace").read()
    except Exception: continue
    code = strip.sub(" ", src)
    pm = re.search(r"^\s*package\s+([\w.]+)", code, re.M); pkg = pm.group(1) if pm else ""
    explicit = {}; star = set()
    for m in re.finditer(r"^\s*import\s+([\w.]+)(\.\*)?(?:\s+as\s+(\w+))?", code, re.M):
        if m.group(2): star.add(m.group(1))
        else:
            p, _, n = m.group(1).rpartition("."); explicit[m.group(3) or n] = (p, n)
    body = re.sub(r"^\s*(import|package)\b[^\n]*", " ", code, flags=re.M)
    for t in set(tok.findall(body)):
        if t in explicit:
            p, n = explicit[t]; cands = [(pp, g) for pp, g in decl.get(n, ()) if pp == p]
        else:
            cands = [(pp, g) for pp, g in decl.get(t, ()) if pp == pkg or pp in star]
        if len(cands) != 1: continue
        g = cands[0][1]
        if g == f: continue
        expected.add((f, g)); why[(f, g)] = t
found = expected & edges
print(f"expected {len(expected)} file pairs, recorded {len(found)} ({100*len(found)/max(1,len(expected)):.1f}% recall)")
random.seed(1); miss = sorted(expected - edges)
for p in random.sample(miss, min(8, len(miss))): print("  miss", why[p], p)
extra = sorted(edges - expected); print(f"recorded but not explained: {len(extra)} of {len(edges)}")
for p in random.sample(extra, min(6, len(extra))): print("  extra", p)
