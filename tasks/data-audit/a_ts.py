# TS/JS recall on relative named imports, and precision, from source text.
import sqlite3, re, collections, posixpath, random, sys, os
S = os.path.dirname(os.path.abspath(__file__))
db = sys.argv[1]
c = sqlite3.connect(f"{S}/{os.environ.get('DBDIR','db')}/{db}.db")
content = dict(c.execute("select file, content from file_contents where file glob '*.[jt]s' or file glob '*.[jt]sx'"))
files = {r[0] for r in c.execute("select name from files")}
by_file = collections.defaultdict(dict)
for uid, name, f, kind in c.execute("select id, name, file, kind from units"):
    if kind != 'module': by_file[f][name] = uid
edges = collections.defaultdict(set)
for a, b, ff in c.execute("select `from`, `to`, from_file from unit_connections"): edges[ff].add(b)
def resolve(f, spec):
    if not spec.startswith("."): return None
    base = posixpath.normpath(posixpath.join(posixpath.dirname(f), spec))
    for cand in [base] + [base + e for e in (".ts",".tsx",".js",".jsx")] + [base + "/index" + e for e in (".ts",".tsx",".js",".jsx")]:
        if cand in files: return cand
imp = re.compile(r'import\s+(?:type\s+)?(?:(\w+)\s*,?\s*)?(?:\{([^}]*)\})?\s*from\s*[\'"]([^\'"]+)[\'"]')
req = re.compile(r'const\s+\{([^}]*)\}\s*=\s*require\(\s*[\'"]([^\'"]+)[\'"]\s*\)')
stats = collections.Counter(); ex = collections.defaultdict(list)
for f, src in content.items():
    pairs = []
    for m in imp.finditer(src):
        names = [n.strip().split(" as ")[0].replace("type ","").strip() for n in (m.group(2) or "").split(",") if n.strip()]
        pairs.append((m.group(3), names + ([m.group(1)] if m.group(1) else [])))
    for m in req.finditer(src):
        pairs.append((m.group(2), [n.strip().split(":")[0].strip() for n in m.group(1).split(",") if n.strip()]))
    test = "test" if re.search(r'\.(spec|test)\.', f) else "non-test"
    for spec, names in pairs:
        tf = resolve(f, spec)
        if not tf: continue
        for n in names:
            if n in by_file[tf]:
                stats[test + " want"] += 1
                if by_file[tf][n] not in edges[f]:
                    stats[test + " miss"] += 1
                    if len(ex[test]) < 4: ex[test].append((f.split('/')[-1], n))
for k in ("non-test", "test"): print(f"{db} relative named imports ({k}): {stats[k+' miss']}/{stats[k+' want']} missing; e.g. {ex[k]}")
random.seed(4)
name = dict(c.execute("select id, name from units"))
es = c.execute("select `from`, `to`, from_file from unit_connections").fetchall()
strip = re.compile(r'//[^\n]*|/\*.*?\*/', re.S)
sample = random.sample(es, min(600, len(es)))
miss = [e for e in sample if not re.search(r'\b' + re.escape(name[e[1]]) + r'\b', strip.sub(" ", content.get(e[2], "")))]
print(f"{db} precision: {len(sample)-len(miss)}/{len(sample)}; e.g. {miss[:2]}")
