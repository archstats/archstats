import sqlite3, sys, os
S = os.path.dirname(os.path.abspath(__file__))
before_dir, after_dir = sys.argv[1], sys.argv[2]
def q(db, sql):
    try: return sqlite3.connect(db).execute(sql).fetchone()[0]
    except Exception as e: return f"ERR {str(e)[:40]}"
checks = [
 (".git files", "select count(*) from files where name like '.git/%' or name like '%/.git/%'"),
 ("React components (summary)", "select coalesce(sum(value),0) from summary where name like '%react__components'"),
 ("health = 0 (below scale)", "select count(*) from files where codesmells__code_health = 0"),
 ("health NULL (not scored)", "select count(*) from files where codesmells__code_health is null"),
 ("age 0 with commits", "select count(*) from files where git__age_in_days = 0 and git__commits__total > 0"),
 ("files with no git history", "select count(*) from files where coalesce(git__commits__total,0) = 0"),
 ("java_class in summary", "select count(*) from summary where name in ('java_class','java_full_class') and value is not null and value != ''"),
 ("git commits (summary)", "select value from summary where name='git__commits__total'"),
 ("contributors (summary)", "select value from summary where name='git__authors__total'"),
 ("git_authors rows", "select count(*) from git_authors"),
 ("deleted-file component ''", "select count(*) from git_commits where component = ''"),
 ("indirect rows / pairs", "select count(*) || ' / ' || (select count(*) from (select distinct `from`,`to` from component_connections_indirect)) from component_connections_indirect"),
 ("min indirect length", "select min(shortest_path_length) from component_connections_indirect"),
 ("go rule status", "select status from rules where rule like '%go__internal%'"),
 ("units", "select count(*) from units"),
 ("module units", "select count(*) from units where kind='module'"),
 ("unit edges", "select count(*) from unit_connections"),
 ("dir lines = all files", "select sum(case when d.complexity__lines = (select sum(f.complexity__lines) from files f where f.name like d.name || '/%') then 1 else 0 end) || '/' || count(*) from directories d"),
]
dbs = sorted(f[:-3] for f in os.listdir(os.path.join(S, after_dir)) if f.endswith(".db"))
for name in dbs:
    b, a = os.path.join(S, before_dir, name + ".db"), os.path.join(S, after_dir, name + ".db")
    print(f"== {name}")
    for label, sql in checks:
        print(f"   {label:30} {str(q(b, sql)) if os.path.exists(b) else '-':>22}  ->  {q(a, sql)}")
