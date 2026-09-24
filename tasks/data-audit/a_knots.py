import sqlite3, re, collections, os
S = os.path.dirname(os.path.abspath(__file__))
c = sqlite3.connect(f"{S}/db/broadleaf.db")
unit_file = dict(c.execute("select id, file from units"))
modules = set(unit_file.values())
E = set()
for a, b in c.execute("select `from`, `to` from unit_connections"):
    fa, fb = unit_file.get(a), unit_file.get(b)
    if fa and fb and fa != fb: E.add((fa, fb))
content = dict(c.execute("select file, content from file_contents where file like '%.java'"))
comp_of = dict(c.execute("select name, component from files"))
by_comp = collections.defaultdict(dict)
for uid, name, f, comp in c.execute("select id, name, file, component from units where file like '%.java'"): by_comp[comp][name] = f
strip = re.compile(r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\])*"', re.S)
E2 = set(E)
for f, src in content.items():
    if f not in modules: continue
    toks = set(re.findall(r'\b[A-Z]\w*\b', strip.sub(" ", src)))
    for name, tf in by_comp.get(comp_of.get(f), {}).items():
        if tf != f and name in toks: E2.add((f, tf))
def knots(E): return {tuple(sorted(p)) for p in E if (p[1], p[0]) in E}
k1, k2 = knots(E), knots(E2)
print(f"module pairs that import each other: engine graph {len(k1)}, with same-package references {len(k2)}")
print("examples only visible with same-package refs:", [tuple(os.path.basename(x) for x in p) for p in sorted(k2 - k1)[:4]])
