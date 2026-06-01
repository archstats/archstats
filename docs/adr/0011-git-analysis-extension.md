# 0011. Git Analysis Extension

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

Integrating repository revision history (git commits, author counts, age, churn) provides massive value for architectural insights (identifying hot spots or unstable files). However, reading git logs requires shell executions or external libraries. Forcing a hard dependency on Git in the core analyzer would increase build sizes, external system requirements, and slow down simple static analysis runs.

## Decision Drivers

- **Modularity**: The core static analyzer must remain fast and light, with zero external dependency requirements.
- **Opt-In Capability**: Users should only pay the performance/system cost of git mining when explicitly requested.
- **Microkernel alignment**: Keep core logic focused strictly on code structure, not repository state.

## Considered Options

- **Built-in Core Git Mining**: Integrated directly into the main pipeline. Slows down basic runs and complicates air-gapped or non-git environment builds.
- **Opt-in Git Extension**: Implement all git commit and author harvesting as an isolated extension.

## Decision Outcome

Chosen option: **Opt-in Git Extension** (implemented in `extensions/git/`).

The git collector is structured entirely as an extension. It registers file-results editors and results editors to augment the collected view columns with git metrics (e.g. `AgeInDays`, `CommitCount`, `Churn`) by executing lightweight git log commands only when activated.

### Consequences

- **Good**: Minimal core analysis engine footprint.
- **Good**: Safe execution in environments without `git` installed (the extension will gracefully stay disabled or report clean errors).
- **Bad**: Requires separate extension registration/wiring.
