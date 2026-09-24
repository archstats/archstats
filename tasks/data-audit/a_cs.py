# C# recall/precision of unit_connections (file grain) against the source text.
# Expected: file F names a type T (PascalCase token outside comments/strings) that is
# declared in exactly one other file G, and G's namespace is F's own, an enclosing one,
# or one of F's usings.
import sqlite3, re, sys, os, random, collections
S = os.path.dirname(os.path.abspath(__file__))
db, root = sys.argv[1], sys.argv[2]
c = sqlite3.connect(db)
types = collections.defaultdict(set)   # name -> {(ns, file)}
for uid, name, f in c.execute("select id, name, file from units where kind='type' and file like '%.cs'"):
    ns = uid[: -len(name) - 1] if uid.endswith("." + name) else ""
    types[name].add((ns, f))
edges = set((a, b) for a, b in c.execute("select from_file, to_file from unit_connections where from_file like '%.cs' and to_file like '%.cs' and from_file != to_file"))
strip = re.compile(r'//[^\n]*|/\*.*?\*/|@"(?:[^"]|"")*"|"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])\'', re.S)
tok = re.compile(r"\b[A-Z][A-Za-z0-9_]*\b")
files = [r[0] for r in c.execute("select distinct file from units where file like '%.cs'")]
expected = set(); why = {}
for f in files:
    try: src = open(os.path.join(root, f.lstrip("./") if f.startswith("./") else f), encoding="utf-8", errors="replace").read()
    except Exception: continue
    code = strip.sub(" ", src)
    usings = set(re.findall(r"^\s*using\s+(?:static\s+)?([A-Za-z0-9_.]+)\s*;", code, re.M))
    nsm = re.search(r"^\s*namespace\s+([A-Za-z0-9_.]+)", code, re.M)
    ns = nsm.group(1) if nsm else ""
    visible = set(usings); parts = ns.split(".")
    for i in range(1, len(parts) + 1): visible.add(".".join(parts[:i]))
    body = re.sub(r"^\s*(using|namespace)\b[^\n]*", " ", code, flags=re.M)
    for t in set(tok.findall(body)):
        decl = types.get(t)
        if not decl or len(decl) != 1: continue
        (tns, g), = decl
        if g == f or tns not in visible: continue
        expected.add((f, g)); why[(f, g)] = t
found = expected & edges
print(f"expected {len(expected)} file pairs, recorded {len(found)} ({100*len(found)/max(1,len(expected)):.1f}% recall)")
miss = sorted(expected - edges); random.seed(1)
for p in random.sample(miss, min(8, len(miss))): print("  miss", why[p], p)
extra = edges - expected
print(f"recorded edges not explained by a token: {len(extra)} of {len(edges)}")
for p in random.sample(sorted(extra), min(5, len(extra))): print("  extra", p)
