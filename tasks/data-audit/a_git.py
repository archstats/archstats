from lib import *

def run(db):
    c, R = con(db), Report(db)
    if not has(c, "git_commits") or not c.execute("select count(*) from git_commits").fetchone()[0]:
        R.check("git", "git data present", None, "no git_commits"); return R
    s = summary(c)
    cur = "file in (select name from files)"
    q = lambda sql: c.execute(sql).fetchone()[0]
    R.check("git", "commits__total = distinct commits touching current files", s["git__commits__total"] == q(f"select count(distinct commit_hash) from git_commits where {cur}"),
            f"{s['git__commits__total']} vs {q(f'select count(distinct commit_hash) from git_commits where {cur}')}")
    for key, col in (("name", "author_name"), ("email", "lower(author_email)")):
        v = q(f"select count(distinct {col}) from git_commits where {cur}")
        R.check("git", f"authors__total = distinct {key} over current files", s["git__authors__total"] == v, f"{s['git__authors__total']} vs {v}")
    v = q(f"select count(distinct file) from git_commits where {cur}")
    R.check("git", "unique_file_changes__total = current files with commits", s["git__unique_file_changes__total"] == v, f"{s['git__unique_file_changes__total']} vs {v}")
    a, d = c.execute(f"select sum(file_additions), sum(file_deletions) from git_commits where {cur}").fetchone()
    R.check("git", "additions__total over current files", s["git__additions__total"] == a, f"{s['git__additions__total']} vs {a}")
    R.check("git", "deletions__total over current files", s["git__deletions__total"] == d, f"{s['git__deletions__total']} vs {d}")
    # Per-file: files.git__commits__total vs git_commits
    if "git__commits__total" in cols(c, "files"):
        bad = c.execute(f"""select f.name, f.git__commits__total, count(distinct g.commit_hash) n
            from files f left join git_commits g on g.file = f.name
            group by f.name having coalesce(f.git__commits__total,0) != n limit 5""").fetchall()
        tot = q("select count(*) from files")
        nbad = q(f"""select count(*) from (select f.name from files f left join git_commits g on g.file=f.name
            group by f.name having coalesce(f.git__commits__total,0) != count(distinct g.commit_hash))""")
        R.check("git", "files.git__commits__total = commits to that file", nbad == 0, f"{nbad}/{tot} differ; e.g. {[tuple(r) for r in bad[:3]]}")
    # Reference date for the last_N windows
    last = q("select max(commit_time) from git_commits")
    scanned = q("select max(timestamp) from files")
    R.check("git", "reference point for last_N windows", None, f"newest commit {last}; scanned {scanned}; last_30 commits={s.get('git__commits__last_30_days')} last_180={s.get('git__commits__last_180_days')}")
    return R

if __name__ == "__main__":
    for db in DBS:
        print(f"== {db}"); run(db)
