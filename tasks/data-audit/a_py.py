# Python recall on absolute and relative `from x import Y`, and precision.
import sqlite3, re, collections, random, sys, os, posixpath
S = os.path.dirname(os.path.abspath(__file__))
db = sys.argv[1]
c = sqlite3.connect(f"{S}/{os.environ.get('DBDIR','db')}/{db}.db")
content = dict(c.execute("select file, content from file_contents where file like '%.py'"))
files = set(content)
by_file = collections.defaultdict(dict)
for uid, name, f, kind in c.execute("select id, name, file, kind from units"):
    if kind != 'module': by_file[f].setdefault(name, uid)
edges = collections.defaultdict(set)
for a, b, ff in c.execute("select `from`, `to`, from_file from unit_connections"): edges[ff].add(b)
def mod_to_file(f, mod):
    if mod.startswith("."):
        dots = len(mod) - len(mod.lstrip(".")); rest = mod.lstrip(".").replace(".", "/")
        base = posixpath.dirname(f)
        for _ in range(dots - 1): base = posixpath.dirname(base)
        cand = posixpath.join(base, rest) if rest else base
        for p in (cand + ".py", cand + "/__init__.py"):
            if p in files: return p
        return None
    base = mod.replace(".", "/")
    for p in ("src/" + base + ".py", base + ".py", "src/" + base + "/__init__.py", base + "/__init__.py"):
        if p in files: return p
stats = collections.Counter(); ex = []
for f, src in content.items():
    for m in re.finditer(r'^from\s+([\w.]+)\s+import\s+\(?([\w,\s]+)\)?', src, re.M):
        tf = mod_to_file(f, m.group(1))
        if not tf: continue
        kind = "relative" if m.group(1).startswith(".") else "absolute"
        for name in [n.strip().split(" as ")[0] for n in m.group(2).split(",") if n.strip()]:
            if name in by_file[tf]:
                stats[kind + " want"] += 1
                if by_file[tf][name] not in edges[f]:
                    stats[kind + " miss"] += 1
                    if len(ex) < 4: ex.append((f.split('/')[-1], name))
for k in ("absolute", "relative"): print(f"{db} {k} from-imports of a declared unit: {stats[k+' miss']}/{stats[k+' want']} missing")
print("   e.g.", ex)
random.seed(9); name = dict(c.execute("select id, name from units"))
es = c.execute("select `from`, `to`, from_file from unit_connections").fetchall()
sample = random.sample(es, min(600, len(es)))
miss = [e for e in sample if not re.search(r'\b' + re.escape(name[e[1]]) + r'\b', re.sub(r'#[^\n]*', ' ', content.get(e[2], "")))]
print(f"{db} precision: {len(sample)-len(miss)}/{len(sample)}; e.g. {miss[:2]}")
