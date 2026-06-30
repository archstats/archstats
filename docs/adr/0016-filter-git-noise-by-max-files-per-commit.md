# 0016. Filter Git Noise by Max Files Per Commit

- **Status**: Accepted
- **Date**: 2026-06-14

## Context and Problem Statement

When analyzing repository git history in `archstats`, massive commits such as license header updates, dependency bumps, or codebase-wide automated formatting change the vast majority of the codebase. These commits introduce severe noise into git-based metrics, especially logical coupling (shared commits between files/components) and code churn/ownership analysis.

We need a mechanism to filter out these noisy commits from analysis so that architectural metrics reflect actual development and change patterns, rather than administrative or mechanical noise.

## Decision Drivers

- Reduce logical coupling noise from repository-wide administration/mechanical changes.
- Ensure performant execution by filtering out massive commits early.
- Keep configuration simple and intuitive for users (YAGNI).

## Considered Options

1. **Option 1: Filter raw commits in `Init()` by max file changes (Recommended)**
   Filter out commits from the raw commit list inside `Init()`, before converting them to `PartOfCommit` or doing multi-dimensional indexing/splitting.
2. **Option 2: Filter in `EditResults()` (Post-Init)**
   Filter out commit parts during component/file mapping in the results-editing lifecycle phase.
3. **Option 3: Stream-Level Filtering**
   Discard commits directly inside the git log parser based on the number of fields parsed.

## Decision Outcome

Chosen option: **Option 1 (Filter raw commits in `Init()`)**, because:
- It isolates the filtered commits before they enter the data analysis pipeline (reducing unnecessary memory and struct allocations).
- It provides a simple, clean configuration point using a single cli flag.
- It is highly unit-testable.

### Consequences

- **Good**: Significant reduction in git coupling noise from large updates.
- **Good**: Improved analysis performance for repositories with massive historical commits (e.g. license change commits).
- **Neutral**: Users can customize the threshold with `--git-max-changes-per-commit` or set it to `0` to disable filtering.
- **Bad**: Highly collaborative/massive feature commits that genuinely touch more than the threshold (default: 100) files might also be filtered out unless the threshold is manually increased.

