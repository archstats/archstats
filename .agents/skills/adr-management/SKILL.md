---
name: adr-management
description: Guidelines and tooling instructions for creating, updating, and referencing Architecture Decision Records (ADRs) in the archstats repository.
---

# ADR Management Skill

This skill enables AI agents and developers to systematically manage and query architectural decisions in the `archstats` codebase.

## When to Use

Use this skill whenever:
1. You are tasked with making significant architectural changes or refactoring core components.
2. The user requests new design decisions to be documented as ADRs.
3. You need to verify past decisions to align new implementation plans with established repository paradigms.

## Tooling & Automation

Always utilize the provided automated bash script in the repository to create new ADRs. Do NOT write numbered ADR files manually, as this risks numbering conflicts or missing indexing.

### Create a New ADR

To create a new ADR, execute:

```bash
./scripts/new-adr.sh "Title of Your Decision"
```

This script automatically:
1. Calculates the next sequential four-digit identifier (e.g., `0016`).
2. Generates a kebab-cased file name: `docs/adr/XXXX-title-of-your-decision.md`.
3. Substitutes template placeholders with the proper title, ISO date, and sets the initial status to `Proposed`.
4. Appends the new record row directly to [docs/adr/README.md](file:///Users/ryansusana/workspace-manager/workspaces/personal/archstats/docs/adr/README.md)'s tabular index.

---

## Directory Structure & Registry

- **Index & Guide**: [docs/adr/README.md](file:///Users/ryansusana/workspace-manager/workspaces/personal/archstats/docs/adr/README.md)
- **ADR Template**: [docs/adr/template.md](file:///Users/ryansusana/workspace-manager/workspaces/personal/archstats/docs/adr/template.md) (standard Michael Nygard format)
- **Decisions Folder**: [docs/adr/](file:///Users/ryansusana/workspace-manager/workspaces/personal/archstats/docs/adr/)

---

## Operating Guidelines for AI Agents

1. **Alignment Verification**: Before drafting any implementation plan for a new feature, scan through the existing ADR list in `docs/adr/README.md` to ensure your planned designs do not violate previously accepted decisions (e.g., respect the snippet-based analysis model in `0004` and avoid tight coupling in `0005`).
2. **Status Lifecycles**:
   - Newly generated ADRs start as `Proposed`.
   - Once approved by the user, their status should be updated to `Accepted`.
   - If a new decision supersedes an older one, modify the old ADR status to `Superseded` and link the new ADR in its context.
