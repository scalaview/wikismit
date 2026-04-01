# Wikismit Workflow Skills Specification

**Status:** Draft
**Last updated:** 2026-03-30

---

## 1. Overview

This document formalizes the current wikismit documentation workflow into reusable local skills. The goal is not to replace the repo's existing `.docs` process, but to make that process explicit, repeatable, and harder for future agents to drift away from.

The workflow remains:

1. clarify and design the work
2. write a technical specification in `.docs/spec`
3. decompose the approved spec into epic documents in `.docs/tasks`
4. decompose a selected epic into implementation-ready plans in `.docs/plans`
5. implement later by following the plan documents

The new skills cover only steps 1-4. Implementation remains a separate execution phase.

---

## 2. Problem Statement

Generic planning and brainstorming skills tend to default to non-repo-native artifact locations such as `docs/tech-spec.md`, `docs/epics/`, or `docs/superpowers/plans/`. In wikismit, those outputs are wrong even if the design quality is otherwise acceptable, because the repo already has an established document system under `.docs/`.

Without repo-specific skills, future agents are likely to:

- place specs in the wrong directory
- create epics outside `.docs/tasks`
- create plans outside `.docs/plans`
- blur the boundary between planning documents and implementation work
- miss the repo's existing naming and dependency conventions

The skills defined here exist to enforce the repo's workflow contracts.

---

## 3. Goals

- Encode the wikismit workflow as reusable local skills
- Preserve the established artifact locations:
  - `.docs/spec`
  - `.docs/tasks`
  - `.docs/plans`
- Keep design, epic decomposition, and implementation planning as separate stages
- Make dependencies explicit between specs, epics, and plans
- Reduce future drift toward generic documentation locations

---

## 4. Non-goals

- Replace the existing repo docs with a new planning system
- Combine planning and implementation into one skill
- Force every workflow into a single monolithic skill
- Define the implementation details of a specific future epic in this document

---

## 5. Proposed Skill Set

### 5.1 `wikismit-brainstorm-to-spec`

Purpose:
- adapt `superpowers:brainstorming` to wikismit's repo conventions
- use "using-git-worktrees" to isolate implementation work in `.worktrees`
- ensure approved design output lands in `.docs/spec`

Scope:
- requirement clarification
- approach comparison
- design approval
- writing a repo-native tech spec

Out of scope:
- epic writing
- plan writing
- implementation

### 5.2 `wikismit-spec-to-epics`

Purpose:
- convert an approved spec into dependency-aware epic documents in `.docs/tasks`

Scope:
- epic boundaries
- dependency declaration
- acceptance criteria and subtask framing

Out of scope:
- plan writing
- code implementation

### 5.3 `wikismit-epic-to-plan`

Purpose:
- convert a chosen epic into detailed implementation-ready plan documents in `.docs/plans`

Scope:
- requirements breakdown
- plan index
- ordered plan slices
- detailed implementation steps and testcases

Out of scope:
- actual coding
- direct source-file edits beyond plan artifacts

---

## 6. Artifact Contracts

### 6.1 Specs

- location: `.docs/spec/`
- source of truth for problem framing, design, goals, and boundaries
- written only after brainstorming and design approval

### 6.2 Epics

- location: `.docs/tasks/`
- derived from an approved spec
- can depend on one another
- should be narrow enough to plan independently

### 6.3 Plans

- location: `.docs/plans/`
- use git worktree to isolate implementation work. location: `.worktrees`
- derived from one specific epic
- should include detailed implementation steps and testcases
- should be specific enough that implementation can be executed later without rediscovering scope

### 6.4 Implementation

- not part of these skills
- begins only after plan artifacts exist
- follows `.docs/plans/` step by step

---

## 7. Workflow Rules

1. Do not write tasks before the spec exists.
2. Do not write plans before the target epic exists.
3. Do not start implementation from the skill-writing phase.
4. Use repo-local `.docs` paths instead of generic documentation folders.
5. Match existing wikismit document shapes whenever possible instead of inventing new templates.

---

## 8. Validation Expectations

The skill set is considered valid when:

- each skill triggers on its intended stage
- each skill points to the correct `.docs` destination
- the planning boundary is preserved
- generic fallback locations are overridden by repo-specific guidance

Baseline validation for this work used pressure scenarios that showed generic behavior without repo-specific skills:

- spec requests defaulted to generic `docs/tech-spec.md` style paths
- epic requests defaulted to generic `docs/epics/` style paths
- plan requests defaulted to generic `docs/superpowers/plans/` style paths

The new skills must redirect those behaviors to `.docs/spec`, `.docs/tasks`, and `.docs/plans` respectively.

---

## 9. Next Steps

1. Use the new skills as the repo-local workflow entry points.
2. Create or refine epic documents under `.docs/tasks` from this spec if further rollout work is needed.
3. For any chosen epic, create plan artifacts in `.docs/plans` before implementation starts.
