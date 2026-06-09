---
description: GitHub Issue Processing Workflow entry point
---

# Notes Management System - GitHub Issue Processing Workflow

This document serves as the entry point and index for Beerkellar's issue-processing workflows. It outlines the general rules and lists the specific workflow files for each stage in the issue lifecycle.

---

## 🚫 Critical General Rules
1. **Scope Adherence**: The agent should only address the labeled issue, and it must stop once the issue is unlabeled.
2. **Termination Rule**: **The agent should not proceed to the next label.** Once you have removed a label from the bug (or a PR is merged), you should stop execution immediately. Do not trigger or begin processing the next stage or label in the same run.

---

## 🏷️ Workflow Stages & Labels

When an issue is labeled, refer to the corresponding workflow document under `.agents/workflows/` for detailed step-by-step instructions:

1. **Requirements gathering**
   - **Label**: `beerkellar-needs-requirements` (or variant `beerkellar-need-requirements`)
   - **Workflow Guideline**: [beerkellar-needs-requirements.md](file:///workspaces/beerkellar/.agents/workflows/beerkellar-needs-requirements.md)

2. **Technical implementation plan formulation**
   - **Label**: `beerkellar-needs-implementation-plan`
   - **Workflow Guideline**: [beerkellar-needs-implementation-plan.md](file:///workspaces/beerkellar/.agents/workflows/beerkellar-needs-implementation-plan.md)

3. **Issue breakdown**
   - **Label**: `beerkellar-break-down-issue`
   - **Workflow Guideline**: [beerkellar-break-down-issue.md](file:///workspaces/beerkellar/.agents/workflows/beerkellar-break-down-issue.md)

4. **Component implementation**
   - **Label**: `beerkellar-ready-to-implement`
   - **Workflow Guideline**: [beerkellar-ready-to-implement.md](file:///workspaces/beerkellar/.agents/workflows/beerkellar-ready-to-implement.md)

5. **Bug triage and resolution**
   - **Label**: `beerkellar-bug`
   - **Workflow Guideline**: [beerkellar-bug.md](file:///workspaces/beerkellar/.agents/workflows/beerkellar-bug.md)

---

## 🛠️ Summary of Expected Label State Transitions

| Phase | Parent Issue Label(s) | Sub-Issue Title & Label(s) |
| :--- | :--- | :--- |
| **Requirements Gathering** | `beerkellar-needs-requirements` | *None (Not yet created)* |
| **Requirements Approved** | *(Label Removed)* | `[Implementation Plan] <Title>` labeled with `beerkellar-needs-implementation-plan` |
| **Implementation Plan Drafting** | *None* | `[Implementation Plan] <Title>` labeled with `beerkellar-needs-implementation-plan` |
| **Implementation Plan Approved** | *None* | **Implementation Plan:** Label removed (remains Open).<br>**Breakdown Sub-Issue:** `[Breakdown] <Title>` labeled with `beerkellar-break-down-issue` |
| **Issue Breakdown** | *None* | **Breakdown Issue:** `beerkellar-break-down-issue` removed (remains Open).<br>**Child Sub-Issues:** `[Sub-Issue] <Action>` labeled with `beerkellar-ready-to-implement` |
| **Implementation** | *None* | **Breakdown Issue:** Closed when all child sub-issues are closed (cascading to close Implementation Plan and Parent issues).<br>**Child Sub-Issues:** Labeled with `beerkellar-ready-to-implement`. Closed programmatically via PR submission. |
| **Bug Triage (Simple)** | `beerkellar-bug` | *None (Direct fix implemented and PR submitted)* |
| **Bug Triage (Complex/Failed)** | `beerkellar-bug` (Removed) | New issue labeled with `beerkellar-needs-requirements` to initiate requirements gathering |
