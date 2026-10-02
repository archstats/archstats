# Code health: what to score, and how

Decided 2026-10-02 after four rounds of measurement and a check against the published research. The scripts and how to rerun them are at the end.

## The decision

A file's health is read from its code alone: no git, no history. It is 10 minus three deductions, never below 1.

| Deduction | Rule | Examples |
|---|---|---|
| **Complex code** | 1 point per doubling of (1 + L ÷ 25), where L is the lines inside functions whose cognitive complexity passes 15 | 25 lines → 1, 75 → 2, 175 → 3, 375 → 4 |
| **Coupling** | 1.5 points per doubling of imports past 10 | 20 imports → 1.5, 40 → 3 |
| **Size** | 1 point per doubling of code lines past 150 (lines that are not blank or comment) | 300 → 1, 600 → 2, 1,200 → 3 |

**Worked example.** A 600-line file with 20 imports and one 75-line complex function:
- size −2, coupling −1.5, complex code −2;
- score **4.5**, yellow.
- The file names the complex function, so the reader knows what to fix.

**Bands are unchanged:** green 8–10, yellow 4–8, red below 4. Over the 28,000 production files tested, that comes to:

| Band | Share of files | Fixes per file, vs green | Edits that were bug fixes |
|---|---|---|---|
| Green | 88% | 1× | 27% |
| Yellow | 8.6% | 5.9× | 33% |
| Red | 3.3% | 19× | 39% |

**What changes from today:**
1. **Indentation deductions go away.** Both average and max nesting go, and with them the per-language thresholds. Beyond size, they predicted *fewer* fixes in every round.
2. **Complexity is measured per function.** It uses cognitive complexity from the syntax tree, Sonar's rules, at Sonar's threshold of 15.
3. **Coupling is new.** It's the file's count of import statements, standard library and third-party included. The resolved in-project fan-out was tested and did worse (see Implemented).
4. **Size counts from 150 code lines, with no cap.** Today it counts from 500 lines and stops at 800.
5. **Only code is scored.** Files a language pack parses get the full reading. Other programming languages (Rust, Ruby, C/C++, Scala and others) get an indentation-based fallback (see Implemented). Markup, data, stylesheets, SQL and shell get none.
6. **Component health stays the line-weighted mean of its files,** with the worst file shown beside it.
7. **Every deduction is stored,** so the score can always be explained.

## What each deduction means

These are the measured effects that set the weights. "Per file" is how many more fixes land in the file. "Per edit" is how much likelier each change to it is to be a bug fix: that's the quality of the code, separate from how much there is or how busy it is.

| Per doubling of… | Per file | Per edit |
|---|---|---|
| Code lines past 150 | ×1.52 (z 26) | ×0.94 |
| Imports past 10 | ×1.72 (z 34) | ×1.01 |
| Complex code, (1 + L ÷ 25) | ×1.14 (z 15) | **×1.08** (z 11) |

All three effects held, with the same sign, in both the dev and the test repos.

- **Complex code is the quality deduction.** It is the only trait that makes every change to the code riskier.
- **Coupling and size are exposure.** More dependencies and more code attract more fixes, but the code isn't worse per line. That matches Koru's finding that small modules have more defects per line.
- **Complex code is weighted above what its per-file effect alone would earn.** A grid search on the dev repos (below) found that weighting improves both the risk ranking and the per-edit measure. It also makes the score say something about how the code is written, not just how much of it there is.

## How it was tested

### Setup

- **Score the past, judge by the future.** Every formula scored the code as it stood two years before HEAD (longer for quiet repos). It was judged by the bug-fix commits that touched each file afterwards. Renames were followed, and commits touching more than 30 files were ignored as sweeps.
- **Real bug labels.** For the three repos that write Jira keys instead of "fix" (hibernate-orm, fineract, sakai), each commit is labelled by its ticket's real issue type from their public Jira. That's 13,000 tickets. In sakai, 86% of tickets are Bugs that the keyword method had missed.
- **Corpus: 15 repos, 28,000 production files, 218,000 functions.**
  - Java: sakai, fineract, hibernate-orm, BroadleafCommerce
  - C#: nopCommerce
  - PHP: Sylius
  - Python: django-oscar, django-rest-framework, npo-diaz
  - TypeScript: librechat, excalidraw, outline
  - Go: hugo, caddy, npo-datahub-hubd
- **Weights chosen on 8 repos, confirmed on 7 never used to tune:** fineract, hibernate-orm, Sylius, Broadleaf, caddy, outline, django-rest-framework.

### Measures

- **Ranking:** Spearman correlation between unhealthiness and later fixes.
- **Fix share:** the same correlation against the share of a file's later commits that were fixes, for files with 3+ commits. This is quality per change.
- **Fixes per line read:** the share of fixes found by reading the least healthy files first until 20% of the code is read. Chance is 20%.

## Results

| | Ranking, dev | Ranking, test | Fix share, dev | Fix share, test | Fixes per line read, dev | Fixes per line read, test |
|---|---|---|---|---|---|---|
| Today's score | 0.268 | 0.347 | 0.065 | 0.088 | 18.2% | 22.1% |
| **Decided** | **0.363** | **0.432** | **0.077** | **0.134** | **21.7%** | **23.6%** |

- **Per repo:** better ranking on 13 of 15 repos, level on 2 (nopCommerce, caddy).
- **Fixes per line read:** the first health score to beat chance on both dev and test. Every earlier formula, today's included, did worse than random on dev.
- **Components** (correlation with a directory's fixes): today's line-weighted mean 0.39, decided 0.44. Worst file is better still at 0.48, but one file decides it and it jumps around, so it's shown next to the mean rather than replacing it.

## How the four rounds got here

1. **Smoothing today's inputs and calibrating them per language didn't help** (0.219 against 0.215). Plain file size beat every formula. Today's average nesting also counts comment lines, so a licence header drags a small file's reading toward zero.
2. **A model with every function smell,** fitted on 10 repos with keyword labels, predicted held-out repos no better than size alone (0.324 against 0.328). Average nesting pointed the wrong way in 6 of 7 languages. The best explainable option was size plus worst-function complexity, and it did beat today's score on 9 of 10 repos. But it amounted to "health = line count", and that wasn't good enough.
3. **Real Jira labels and five more repos** (hugo, caddy, excalidraw, outline, django-rest-framework) roughly doubled to tripled the bug signal in the Jira repos (hibernate-orm 1,488 fix-touched files, up from 911; sakai 1,175, up from 341). Two code-only signals now held up beyond size on held-out repos: imports (z ≈ 9 in both dev and test) and lines in complex functions.
4. **Function-level attribution.** Every later diff was mapped onto the T0 functions it edited: 25,000 edited functions, 9,000 of them fixed. This finally separated complexity from size:

   | Function length | Complex (cognitive > 15 or nesting ≥ 3): share of edits that were fixes | Simple |
   |---|---|---|
   | 50–100 lines | 41% (1,254 functions) | 29% (916) |
   | 100–200 lines | 43% (735) | 28% (168) |
   | 200+ lines | 48% (311) | 12% (34) |

   - **Long but simple functions are not worse:** 0.93× per edit, worse in only 3 of 11 repos.
   - **Long and complex functions are worse:** 1.64× per edit pooled, worse in 9 of 11 repos.
   - **Length alone is not a quality problem; complexity is.** That's why the complex-code deduction counts lines in complex functions, and the size deduction stays small.

The weights came from a grid search on the dev repos. The best region was flat: coupling 1–1.5× size, complex code 0.3–1× size. The decision takes 1.5 and 1, the best combined result, at a scale where about 88% of files are green and 3% red. CodeScene's field data has 89% healthy and 1% alert.

## What the research says

**Agrees with this decision:**
- **Size confounds most code metrics.** After controlling for class size, the classic object-oriented metrics stopped predicting faults ([El Emam et al. 2001](https://neverworkintheory.org/2011/07/07/the-confounding-effect-of-class-size-on-the-validity-of-object-oriented-metrics.html)). Across projects, only the size-like component of metrics is consistent ([Gil & Lalouche](https://www.cs.technion.ac.il/events/2017/2684/)). This is why every claim here is tested beyond size.
- **Cognitive complexity is validated for understandability.** A meta-analysis of 24,000 human ratings of 427 snippets found it tracks comprehension time and perceived difficulty ([Muñoz Barón, Wyrich & Wagner, ESEM 2020](https://arxiv.org/abs/2007.12520)). The threshold of 15 is Sonar's, raised from 10 because 10 was noisy ([SonarSource](https://community.sonarsource.com/t/s3776-reason-for-the-current-default-value-of-15/127103)).
- **Long, complex code is where smells hurt.** 17,350 validated smells in 30 projects: the long/complex ones are both the most common and linked to change- and fault-proneness ([Palomba et al. 2018](https://link.springer.com/10.1007/s10664-017-9535-z)).
- **Coupling is only partly explained by size.** Size mediates CBO but only moderates fan-out ([Tahir & Xiao 2021](https://arxiv.org/abs/2106.04687)).
- **Small modules have more defects per line** ([Koru et al., theory of relative defect-proneness](https://swag.uwaterloo.ca/publications/replicating-and-re-evaluating-the-theory-of-relative-defect-proneness.html)). That's why a size-heavy score can do worse than random when effort is counted (Mende & Koschke, as summarized in [this 2025 review of effort-aware metrics](https://arxiv.org/abs/2504.19181)), and why the size weight stays modest.

**Explains what I changed in the method:**
- **Bug labels are noisy.** 33.8% of "bug" reports in five projects were not bugs ([Herzig, Just & Zeller, ICSE 2013](https://www.st.cs.uni-saarland.de/publications/files/herzig-icse-2013.pdf)). Hence the real Jira issue types.
- **Rule-by-rule smell counts barely predict faults.** Across 33 Apache projects, SonarQube issues had a significant but small effect on changes and none on faults ([Lenarduzzi et al.](https://arxiv.org/abs/1908.11590)). Hence few deductions, each tested.

**Disagrees, and why this decision goes the other way:**
- **Indentation as a complexity proxy.** It does correlate with McCabe and Halstead ([Hindle, Godfrey & Holt 2008](https://swag.uwaterloo.ca/publications/reading-beside-the-lines-indentation-as-a-proxy-for-complexity-metric.html)). But correlating with complexity is not the same as predicting trouble beyond size, and here it didn't, in four rounds.
- **CodeScene's "15× fewer defects".** It compares defects *per file* across bands, with no control for size or change rate ([Tornhill & Borg 2022](https://arxiv.org/abs/2203.04374)). Any size-heavy score earns ratios like that: today's score gets 27× here. Size is also one of CodeScene's own factors ([CodeScene docs](https://docs.enterprise.codescene.io/versions/6.3.3/guides/technical/code-health.html)). Their per-edit time result is the stronger claim, and this decision's per-edit gradient (27% → 33% → 39% of edits being fixes, green to red) is the same kind of evidence.

## Rejected

- **Indentation, average or max.** No signal beyond size in any round, and negative in most languages.
- **Duplication, comment count, parameter count, bumpy road.** Their sign flipped between repo sets, or they went the wrong way. They can be shown on a file as findings, but they don't deduct points.
- **Size only** (round 3). It predicts nearly as well but says nothing about how the code is written.
- **Worst file as the component number.** It's slightly more predictive but volatile; it's shown, not used.
- **Git-based signals in health.** Churn predicts more than any static trait. But health must be readable from the code itself, so churn stays in the Hotspots view.

## Limits

- **Fixes are a proxy.** They don't capture time lost to hard code. CodeScene's time result suggests complexity costs more than its fix counts show.
- **C# `using` names namespaces, not types,** so C# coupling reads low. (The prototype also missed CommonJS `require()`; the engine counts it.)
- **The complexity counts are close to Sonar's, not identical.** Recursion and labelled jumps aren't counted. Hand-checked samples match in Java, Go, Python, TypeScript and PHP.
- **Functions are matched by name and parameter count.** Functions renamed after T0 lose their later edits.
- **Test files were not judged.** The formula applies to them unchanged until there's evidence.
- **Kotlin, Swift, Dart and Objective-C** got function tables in the implementation, checked by hand-scored tests but not against outcomes: none of the 15 repos is written in them.

## Implemented (2026-10-02, branch code-health-v2, analysis revision 11)

### What went in

- **Measured once per file, on the tree the pack already parsed.** `extensions/treesitter/common/complexity.go` walks it: one walker, one table of node kinds per language. There are 11 languages, each with hand-scored tests: Java, Go, Python, JS/TS (Vue and Svelte script blocks included), PHP, C#, Kotlin, Swift, Dart and Objective-C. Template markup in Vue and Svelte is never read.
- **New per-file stats:** `complexity__lines__code`, `complexity__lines__complex`, `complexity__functions`, `complexity__functions__complex`, `complexity__cognitive__max`, `modularity__imports__count`.
- **A `functions` view:** file, name, lines, cognitive complexity, nesting and parameters for every function outside vendored and generated code.
- **Health in `extensions/codesmells/health.go`,** with the new deductions `__coupling`, `__complex_code` and `__deep_code` (the fallback's). The indentation deductions and thresholds are gone.
- **Components** are weighted by code lines, with `codesmells__code_health__worst_file` beside the mean.

### Coupling: validated, and it changed the plan

The plan was to use the engine's resolved in-project fan-out. On the test repos it did worse than leaving coupling out:

| Coupling measure | Test ranking |
|---|---|
| No coupling term | 0.412 |
| Resolved in-project fan-out | 0.396 |
| Fan-in | 0.392 |
| Import statements, standard library and third-party included | 0.419–0.431 |

So the walker counts import statements itself, the same way in every language, and CommonJS `require()` counts too. The engine's older raw-import stats agree, but are missing for Java and JavaScript. The script is `coupling.py`.

### The indentation fallback: chosen the same way

The fallback covers languages no pack parses (Rust, Ruby, C/C++, Scala and others). Candidates were scored on the 15 repos as if they had no parser:

| Fallback candidate | Ranking, dev / test | Fix share, dev / test |
|---|---|---|
| **Size (non-blank lines past 180) + lines indented 3+ levels** | **0.331 / 0.396** | **0.081 / 0.104** |
| Size only | 0.345 / 0.419 | 0.040 / 0.083 |
| Size + today's average nesting | 0.292 / 0.387 | 0.052 / 0.093 |
| Today's score | 0.268 / 0.347 | 0.065 / 0.088 |

- The chosen fallback beats today's score on 5 of 6 measures. The exception is test fixes-per-line-read, 21.4% against 22.1%.
- Size only ranks slightly higher but is much worse on fix share, i.e. it says least about how the code is written.
- None of them reaches the parsed score. This is a stand-in, not an equal.
- Markup, data, stylesheets, SQL and shell get no reading.

### Regression: the engine against the experiment

Every two-year-old tree was scanned with the old and new engine and judged by the same later fixes (`regress.py`):

| | Ranking, dev | Ranking, test | Fix share, dev | Fix share, test | Fixes per line read, dev | Fixes per line read, test |
|---|---|---|---|---|---|---|
| Old engine | 0.264 | 0.347 | 0.058 | 0.088 | 18.3% | 22.1% |
| Prototype formula | 0.363 | 0.432 | 0.077 | 0.134 | 21.7% | 23.6% |
| **New engine** | **0.370** | **0.432** | **0.084** | **0.134** | **21.9%** | **23.6%** |

- **The engine reproduces the prototype** score for score (Spearman 1.0). Its inputs match too: code lines 1.0, complex lines 1.0, imports 0.997.
- **It's slightly better on dev** because it also counts `require()`. LibreChat goes from 0.235 to 0.251.
- **It ranks better than the old engine on all 15 repos.** On nopCommerce the gain is only 0.002.
- **All 37 Go packages' tests pass.**

### Still to do

- **archstats-ui** reads the removed deductions: `HealthBreakdown.vue` and the file list in `views/components/[name]/inside.vue`. Revision 11 snapshots need the new breakdown (size, coupling, complex code with the functions behind it; deep code for fallback files), while revision 10 and older keep the old one.
- **The fallback** has only been validated by simulation. None of the 15 repos is in a fallback language.

## Reproduce

```bash
python3 tasks/code-health/jira.py <repo> "$WORK"                     # only hibernate-orm, fineract, sakai
tasks/code-health/run_all.sh "$WORK" <repo>[:years] ...              # T0 tree, featurize, future + past changes, function attribution
python3 tasks/code-health/score.py "$WORK" full                      # every formula h0–h4, per repo, by language, rollups, bands
python3 tasks/code-health/score.py "$WORK" explore                   # each raw measurement against fixes, size held out
python3 tasks/code-health/fit.py "$WORK"                             # what each file measurement is worth, dev vs test
python3 tasks/code-health/funcs.py "$WORK"                           # function-level: per function and per edit
go build -o "$WORK/appscan" ./tasks/code-health/scan                 # the real engine, as the app scans
"$WORK/appscan" "$WORK/<repo>/tree" "$WORK/<repo>/engine2.db"        # per repo, after a change
python3 tasks/code-health/coupling.py "$WORK"                        # coupling candidates (needs engine.db)
python3 tasks/code-health/regress.py "$WORK"                         # old engine vs new engine vs prototype
```

`score.py` keeps every formula tried (h0–h4) so a later change can be compared with all of them.
