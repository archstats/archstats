"""Real bug labels for repos that name Jira tickets instead of saying "fix":
fetches the issue type of every ticket their commits mention.

    python3 jira.py <repo> <work-dir>

Writes <work-dir>/<name>/jira.json, ticket -> issue type (a sub-task takes
its parent's type). history.py and past.py then call a commit a fix when a
ticket it names is a Bug, and fall back to the keyword when it names none.
"""
import json, os, re, subprocess, sys, time, urllib.parse, urllib.request

PROJECTS = {
    "hibernate-orm": ("HHH", "https://hibernate.atlassian.net/rest/api/3/search/jql"),
    "sakai": ("SAK", "https://sakaiproject.atlassian.net/rest/api/3/search/jql"),
    "fineract": ("FINERACT", "https://issues.apache.org/jira/rest/api/2/search"),
}


def keys_in(text, prefix):
    return set(re.findall(rf"\b{prefix}-\d+\b", text))


def fetch(url, keys):
    jql = "key in (" + ",".join(sorted(keys)) + ")"
    q = urllib.parse.urlencode({"jql": jql, "fields": "issuetype,parent", "maxResults": 100})
    req = urllib.request.Request(f"{url}?{q}", headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            data = json.load(r)
    except urllib.error.HTTPError as e:
        if e.code == 400 and len(keys) > 1:  # a key that does not exist fails the batch
            half = sorted(keys)
            return fetch(url, half[: len(half) // 2]) | fetch(url, half[len(half) // 2:])
        if e.code == 400:
            return {}
        raise
    out = {}
    for issue in data.get("issues", []):
        f = issue["fields"]
        kind = f["issuetype"]["name"]
        if f["issuetype"].get("subtask") and f.get("parent"):
            kind = f["parent"]["fields"]["issuetype"]["name"]
        out[issue["key"]] = kind
    return out


def main():
    repo, work = sys.argv[1], sys.argv[2]
    name = os.path.basename(os.path.normpath(repo))
    prefix, url = PROJECTS[name]
    log = subprocess.run(["git", "-C", repo, "log", "--no-merges", "--since=2018-01-01", "--format=%s%n%b"],
                         capture_output=True, text=True, errors="replace").stdout
    keys = sorted(keys_in(log, prefix))
    types = {}
    for i in range(0, len(keys), 100):
        types |= fetch(url, keys[i:i + 100])
        time.sleep(0.2)
    path = os.path.join(work, name, "jira.json")
    json.dump(types, open(path, "w"))
    counts = {}
    for t in types.values():
        counts[t] = counts.get(t, 0) + 1
    print(name, len(keys), "keys,", len(types), "found:", dict(sorted(counts.items(), key=lambda kv: -kv[1])[:6]))


if __name__ == "__main__":
    main()
