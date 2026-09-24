from lib import *

def run(db):
    c, R = con(db), Report(db)
    s = summary(c)
    files = c.execute("select count(*) from files").fetchone()[0]
    R.check("summary", "complexity__files = rows in files", s.get("complexity__files") == files, f"{s.get('complexity__files')} vs {files}")
    lines = c.execute("select sum(complexity__lines) from files").fetchone()[0]
    R.check("summary", "complexity__lines = sum(files.complexity__lines)", s.get("complexity__lines") == lines, f"{s.get('complexity__lines')} vs {lines}")
    comps = c.execute("select count(*) from components").fetchone()[0]
    fcomps = c.execute("select count(distinct component) from files where component is not null and component != ''").fetchone()[0]
    R.check("summary", "component_count = rows in components", s.get("component_count") == comps, f"{s.get('component_count')} vs {comps}")
    R.check("summary", "component_count = distinct files.component", s.get("component_count") == fcomps, f"{s.get('component_count')} vs {fcomps}")
    dirs = c.execute("select count(*) from directories").fetchone()[0]
    R.check("summary", "directory_count = rows in directories", s.get("directory_count") == dirs, f"{s.get('directory_count')} vs {dirs}")
    if has(c, "modules"):
        mods = c.execute("select count(*) from modules").fetchone()[0]
        R.check("summary", "module_count = rows in modules", s.get("module_count") == mods, f"{s.get('module_count')} vs {mods}")
    if has(c, "git_commits") and c.execute("select count(*) from git_commits").fetchone()[0]:
        commits = c.execute("select count(distinct commit_hash) from git_commits").fetchone()[0]
        R.check("git", "git__commits__total = distinct commit_hash", s.get("git__commits__total") == commits, f"{s.get('git__commits__total')} vs {commits}")
        by_email = c.execute("select count(distinct lower(author_email)) from git_commits").fetchone()[0]
        by_name = c.execute("select count(distinct author_name) from git_commits").fetchone()[0]
        by_pair = c.execute("select count(*) from (select distinct author_name, author_email from git_commits)").fetchone()[0]
        ga = c.execute("select count(*) from git_authors").fetchone()[0]
        R.check("git", "git__authors__total matches a distinct-author count", s.get("git__authors__total") in (by_email, by_name, by_pair),
                f"summary {s.get('git__authors__total')}; distinct email {by_email}, name {by_name}, name+email {by_pair}; git_authors rows {ga}")
        R.check("git", "git_authors rows = distinct authors", ga in (by_email, by_name, by_pair), f"{ga} rows; email {by_email} name {by_name} pair {by_pair}")
        uf = c.execute("select count(distinct file) from git_commits").fetchone()[0]
        R.check("git", "git__unique_file_changes__total = distinct files in git_commits", s.get("git__unique_file_changes__total") == uf, f"{s.get('git__unique_file_changes__total')} vs {uf}")
        add = c.execute("select sum(file_additions), sum(file_deletions) from git_commits").fetchone()
        R.check("git", "git__additions__total = sum(file_additions)", s.get("git__additions__total") == add[0], f"{s.get('git__additions__total')} vs {add[0]}")
        R.check("git", "git__deletions__total = sum(file_deletions)", s.get("git__deletions__total") == add[1], f"{s.get('git__deletions__total')} vs {add[1]}")
    if has(c, "units"):
        kinds = dict(c.execute("select kind, count(*) from units group by kind").fetchall())
        for k, v in kinds.items():
            R.check("units", f"unit_count__{k} = units where kind={k}", s.get(f"unit_count__{k}") == v, f"{s.get(f'unit_count__{k}')} vs {v}")
    for k in ("java_class", "java_full_class", "git__repository"):
        if k in s:
            R.check("summary", f"'{k}' is meaningful codebase-wide", False if s[k] not in ("", None) else None, f"value {s[k]!r}")
    return R

if __name__ == "__main__":
    for db in DBS:
        print(f"== {db}")
        run(db)
