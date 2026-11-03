# Debt interest: bad code × churn

Decided 2026-11-02. The question was: one score for where architects should look first. The proposal was a composite of code health, hotspot score, complexity and size. The scripts are in `interest/`, and how to rerun them is at the end.

## The decision

**debt interest = (1 + health deductions) × log2(1 + recent commits)**

- **Health deductions:** code health's own (size, coupling, complex code; deep code for a file read by its indentation). They are summed without health's floor of 1, so two very large files still differ.
- **Recent commits:** each commit counts `0.5 ^ (age in days ÷ 182)`, so 1 at the newest commit scanned and ¼ a year before it. The commit that added the file is left out, and so are sweeps (commits touching more than 100 files).
- **Test files:** none. They keep their health reading, but a test changes with the code it tests, and the evaluation scored production files only.
- **Groups:** a component's or directory's interest is the sum of its files'. Their recent commits count each commit once.
- **Stored as:** `codesmells__debt_interest` and `git__commits__recent` (revision 21).

No commit messages are read. Fix labels were only the yardstick.

**How it reads:**
- Bad code nobody touches costs nothing yet.
- Healthy code that changes often costs a little.
- Bad code that keeps changing costs most.

## Why not the composite

### The four inputs are two signals

Spearman between the inputs, the mean over 7 repos:

| | size | complexity | unhealth | churn | hotspot |
|---|---|---|---|---|---|
| size | 1 | 0.71 | 0.63 | 0.41 | 0.48 |
| hotspot | | | | 0.99 | 1 |

- Health already holds size and complexity.
- Hotspot is lines × log commits, and ranks almost exactly like churn.
- An equal-weight average of the four ranks 0.93 with size alone.

### It barely beats size

Production files, scored at T0, judged by later fixes:

| | Rank vs later fixes | Fixes per 20% of lines read | Fix share per edit |
|---|---|---|---|
| size | 0.314 | 0.199 (chance) | 0.068 |
| health | 0.316 | 0.220 | 0.130 |
| churn | 0.299 | 0.289 | 0.067 |
| 4-way composite | 0.322 | 0.254 | 0.165 |
| bad × churn | 0.327 | 0.255 | 0.120 |

### Structure at file level adds nothing

Fan-in, the number of other components using a file, and file cycles each add about 0 beyond size.

### At component level, health stops adding and churn starts

What each component signal adds beyond size (partial Spearman):

| | Beyond size |
|---|---|
| churn sum | 0.24 |
| bad × churn, summed | 0.16 |
| betweenness | 0.14 |
| in a component cycle | 0.10 |
| line-weighted health | 0.04 |

## Choosing the variant

There were 19 variants (`interest/interest.py`). Every one ranked later fixes within ±0.015 of the others, so the formula's details matter little.

| | Rank | Beyond size | Fixes per 20% of lines |
|---|---|---|---|
| bad × commits (2 years) | 0.318 | 0.060 | 0.255 |
| bad × recent-weighted commits | 0.319 | 0.060 | 0.253 |
| **(1 + bad) × recent-weighted commits** | 0.303 | **0.111** | **0.268** |
| bad × bug-fix commits only | 0.317 | 0.036 | 0.230 |
| complex-code deduction only × churn | 0.315 | 0.016 | 0.251 |

- **The 1** lets a healthy but busy file into the ranking. That doubles what the score knows beyond size.
- **The decay** beat a fixed two-year count on 5 of 7 repos. A 3-month window was worse.
- **Bug-fix commits as the churn input** did worse, so no message parsing is needed.

### New files

Files under 6 months old at T0 are 17% of files and take 20% of later fixes, at 8.4 fixes per 1,000 lines against 5.0 for files over 2 years old. New code really is more fix-prone. Debt interest ranks equally well within each age band (0.26–0.31). `interest/fileage.py` tested how to treat the commits made while a file was being written:

| | Rank | Top-100 share of fixes | Share of new files in top 100 |
|---|---|---|---|
| all commits | 0.298 | 0.437 | 21% |
| **without the adding commit** | **0.308** | 0.435 | 17% |
| without the first 30 days | 0.315 | 0.452 | 16% |
| without the first 90 days | 0.302 | 0.435 | 15% |

- **Leaving out the adding commit** was equal or better on 6 of 7 repos. It is also the simplest rule: that commit is not a change to existing code.
- **Leaving out the first 30 days** won on average, but most of that win came from one repo (librechat).
- **The engine finds the adding commit** as the file's oldest commit when that commit deleted nothing. A history cut short starts with a change instead.

### Function level: not worth it

`interest/funcinterest.py` mapped every edit before and after T0 onto the T0 functions: 110,000 edits.

- **Across all functions,** (1 + complex-code points) × edits found more fixes per line read than function length (0.273 vs 0.164).
- **Inside the hottest files,** the 3 functions it picks cover 29% of the file's later fixes with 26% of its function lines. The 3 longest functions cover 30% with 31%.

So it doesn't narrow a hot file down better than what the file page already shows. Debt interest stays at file level.

## How it was tested

- The same setup as `DECISION.md`: score at T0, two years before HEAD, and judge by the fix commits since. Commits touching more than 30 files were ignored. Production files only.
- Seven repos, from the full clones that were available: Sylius, django-oscar, fineract, hibernate-orm, librechat, nopCommerce and sakai.
  - hibernate-orm, fineract and sakai are labelled by their Jira issue types.
  - BroadleafCommerce and elepy had under 20 fix-touched files and were left out.
- Structure came from engine scans of the T0 trees (`unit_connections`, `components`).
- **Engine check:** on a django-oscar scan at HEAD, `git__commits__recent` matched a recomputation straight from `git log` on 1,379 of 1,393 files. The 14 differences are moved files, and one file deleted in 2019 and re-created in 2026, whose history the engine rightly starts at the re-creation.

## Limits

- **Seven repos and one time split.** Differences under about 0.02 in rank are noise.
- **Fixes are a proxy.** Change cost and coordination load aren't measured.
- **Test files have no interest.** They were never judged, and before they were left out a test folder was django-oscar's second-highest component.
- **The half-life is not tuned.** Six months was chosen, not fitted.

## Reproduce

Run these after the `DECISION.md` steps (`history.py`, `past.py`, `jira.py`, featurize) for each repo, with `$WORK` the work directory:

```bash
python3 tasks/code-health/interest/pastlog.py <repo> "$WORK"        # dated churn before T0
python3 tasks/code-health/interest/pastchanges.py <repo> "$WORK"    # past changes in featurize's shape
"$WORK/featurize" touched <repo> "$WORK/<name>/changes.csv" "$WORK/<name>/touched.csv"
"$WORK/featurize" touched <repo> "$WORK/<name>/pastchanges.csv" "$WORK/<name>/pasttouched.csv"
"$WORK/appscan" "$WORK/<name>/tree" "$WORK/<name>/engine.db"        # structure, for composite.py

python3 tasks/code-health/interest/composite.py "$WORK"             # redundancy, composite, structure, components
python3 tasks/code-health/interest/interest.py "$WORK"              # the 19 variants
python3 tasks/code-health/interest/fileage.py "$WORK"               # new files vs old
python3 tasks/code-health/interest/funcinterest.py "$WORK"          # function level
```
