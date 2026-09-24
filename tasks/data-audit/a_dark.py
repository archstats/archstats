import sqlite3, re, collections, sys
db = sys.argv[1]
c = sqlite3.connect(f"{__import__('os').path.dirname(__file__)}/db/{db}.db")
unit_file = dict(c.execute("select id, file from units"))
modules = set(unit_file.values())
E = set()
for a, b in c.execute("select `from`, `to` from unit_connections"):
    fa, fb = unit_file.get(a), unit_file.get(b)
    if fa and fb and fa != fb: E.add((fa, fb))
def report(label, E):
    fin = collections.Counter(b for a, b in E); fout = collections.Counter(a for a, b in E)
    isolated = sum(1 for m in modules if not fin[m] and not fout[m])
    never = sum(1 for m in modules if not fin[m])
    print(f"  {label:44} edges {len(E):6}  isolated (UI 'dark') {isolated:5}  never imported {never:5}  of {len(modules)} modules")
print(f"== {db}")
report("engine edges (what the UI uses)", E)
if db in ("broadleaf", "sakai"):
    content = dict(c.execute("select file, content from file_contents where file like '%.java'"))
    comp_of = dict(c.execute("select name, component from files"))
    by_comp = collections.defaultdict(dict)
    for uid, name, f, comp in c.execute("select id, name, file, component from units where file like '%.java'"):
        by_comp[comp][name] = f
    strip = re.compile(r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\])*"', re.S)
    ann = {}
    for f, src in content.items():
        p = re.search(r'^\s*package\s+([\w.]+)\s*;', src, re.M); m = re.search(r'^\s*(?:public\s+)?@interface\s+(\w+)', src, re.M)
        if p and m: ann[p.group(1) + "." + m.group(1)] = f
    E2 = set(E); extra_mods = set()
    for f, src in content.items():
        toks = set(re.findall(r'\b[A-Z]\w*\b', strip.sub(" ", src)))
        for name, tf in by_comp.get(comp_of.get(f), {}).items():
            if tf != f and name in toks and f in modules: E2.add((f, tf))
        for m in re.finditer(r'^\s*import\s+([\w.]+)\s*;', src, re.M):
            if m.group(1) in ann and f != ann[m.group(1)]:
                E2.add((f, ann[m.group(1)])); extra_mods.add(ann[m.group(1)])
    modules |= extra_mods
    report("+ same-package + annotation references", E2)
