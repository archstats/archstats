import sqlite3, sys, os, collections
DBS = [d for d in ["broadleaf", "sakai", "librechat", "gin", "oscar", "nopcommerce", "elepy"] if os.path.exists(os.path.join(os.path.dirname(os.path.abspath(__file__)), os.environ.get("DBDIR", "db"), d + ".db"))]
ROOT = os.path.dirname(os.path.abspath(__file__))
DBDIR = os.environ.get("DBDIR", "db")

def con(name):
    c = sqlite3.connect(f"file:{ROOT}/{DBDIR}/{name}.db?mode=ro", uri=True)
    c.row_factory = sqlite3.Row
    return c

def has(c, table):
    return c.execute("select 1 from sqlite_master where name=?", (table,)).fetchone() is not None

def cols(c, table):
    return [r[1] for r in c.execute(f"pragma table_info(`{table}`)")]

def summary(c):
    return {r["name"]: r["value"] for r in c.execute("select name, value from summary")}

class Report:
    def __init__(self, db):
        self.db, self.rows = db, []
    def check(self, area, what, ok, detail=""):
        self.rows.append((area, what, ok, detail))
        mark = "PASS" if ok is True else ("SKIP" if ok is None else "FAIL")
        print(f"  [{mark}] {area}: {what}" + (f" -- {detail}" if detail else ""))
