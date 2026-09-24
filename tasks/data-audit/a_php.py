# PHP recall/precision of unit_connections (file grain) against source text, resolving
# names the way PHP does: fully qualified, through a use alias, else the file's namespace.
import sqlite3, re, sys, os, random, collections
db, root = sys.argv[1], sys.argv[2]
c = sqlite3.connect(db)
where = {}
for uid, f in c.execute("select id, file from units where kind='type' and file like '%.php'"):
    where[uid.lower()] = f
edges = set(c.execute("select from_file, to_file from unit_connections where from_file like '%.php' and to_file like '%.php' and from_file != to_file"))
strip = re.compile(r"//[^\n]*|#(?!\[)[^\n]*|/\*.*?\*/|'(?:\\.|[^'\\])*'|\"(?:\\.|[^\"\\])*\"", re.S)
name = re.compile(r"(?<![\w$>:\\])\\?[A-Z_a-z][A-Za-z0-9_]*(?:\\[A-Za-z_][A-Za-z0-9_]*)*")
kw = {"self","static","parent","true","false","null","array","string","int","float","bool","void","mixed","callable","iterable","object","never","new","return","function","class","use","namespace"}
expected = set(); why = {}
files = [r[0] for r in c.execute("select distinct file from units where file like '%.php'")]
for f in files:
    try: src = open(os.path.join(root, f[2:] if f.startswith("./") else f), encoding="utf-8", errors="replace").read()
    except Exception: continue
    code = strip.sub(" ", src)
    m = re.search(r"^\s*namespace\s+([\w\\]+)\s*[;{]", code, re.M); ns = m.group(1) if m else ""
    alias = {}
    for u in re.finditer(r"^\s*use\s+(?!function\b|const\b)([^;]+);", code, re.M):
        body = u.group(1).strip()
        if "{" in body:
            pre, rest = body.split("{", 1); pre = pre.strip().lstrip("\\")
            parts = [pre + p.strip() for p in rest.rstrip("}").split(",") if p.strip()]
        else: parts = [body.lstrip("\\")]
        for p in parts:
            mm = re.match(r"([\w\\]+)(?:\s+as\s+(\w+))?$", p.strip())
            if mm: alias[(mm.group(2) or mm.group(1).split("\\")[-1]).lower()] = mm.group(1)
    body = re.sub(r"^\s*(use|namespace)\b[^\n]*", " ", code, flags=re.M)
    # only names in class positions: before ::, after new/extends/implements/instanceof/catch, type hints
    for t in set(name.findall(body)):
        if t.lower() in kw: continue
        if t.startswith("\\"): full = t[1:]
        else:
            first, _, rest = t.partition("\\")
            if first.lower() in alias: full = alias[first.lower()] + ("\\" + rest if rest else "")
            else: full = (ns + "\\" + t) if ns else t
        g = where.get(full.lower())
        if g and g != f: expected.add((f, g)); why[(f, g)] = t
found = expected & edges
print(f"expected {len(expected)} file pairs, recorded {len(found)} ({100*len(found)/max(1,len(expected)):.1f}% recall)")
random.seed(1)
for p in random.sample(sorted(expected - edges), min(8, len(expected - edges))): print("  miss", why[p], p)
extra = sorted(edges - expected); print(f"recorded but not explained: {len(extra)} of {len(edges)}")
for p in random.sample(extra, min(5, len(extra))): print("  extra", p)
