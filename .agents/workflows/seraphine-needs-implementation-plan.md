---
description: Formulate a technical implementation plan before coding
---

# 🛠️ The `seraphine-needs-implementation-plan` Label Workflow

When a sub-issue is labeled with `seraphine-needs-implementation-plan`, the AI assistant (**Beerkellar**) is triggered to formulate a concrete, technical design and step-by-step implementation blueprint before any code changes are made.

## 🔄 Workflow Lifecycle

```mermaid
graph TD
    A[Sub-Issue Labeled seraphine-needs-implementation-plan] --> B[1. Read Context & Parent PRD]
    B --> C[2. Perform Pre-Flight Codebase Analysis]
    C --> D[3. Interactive Grilling Session /grill-me]
    D --> E[4. Technical Implementation Plan Formulation]
    E --> F[5. Post Plan & Create Breakdown Sub-Issue]
```

---

## 📋 Phase Guidelines

### 1. Read Context & Parent PRD
Beerkellar reads the sub-issue description, extracts the reference link to the parent issue, and retrieves the approved Product Requirements Document (PRD) posted as a comment on the parent issue. This grounds all architectural decisions in the defined scope and user requirements.

### 2. Perform Pre-Flight Codebase Analysis
Before asking any grilling questions, Beerkellar must perform a comprehensive scan of the codebase to gain a complete understanding of relevant schemas, services, packages, and CLI commands.
* **Scan Areas:**
  - Protocol Buffers: Files under `/proto/` that define the data model and gRPC services.
  - Backend: Directory structure and Go code under `/server/` and `/beerkellar_cli/`.
  - Database & Storage: Check `db.go` and `pstore` usage.
* **Action:** Present a concise summary of the "Pre-Flight Analysis" to the user, highlighting existing structures that will be affected by the plan.

### 3. Interactive Grilling Session (`/grill-me`)
Beerkellar initiates a technical grilling session with the developer/user. The session proceeds Socratically—**asking exactly one highly targeted question at a time** (never group or ask multiple questions at once)—and suggests sensible, best-practice defaults to resolve technical design ambiguities.
* **Precondition:** Before starting the session, the agent must ensure a thorough understanding of the codebase, the bug, and the context from previous phases.
* **Mandatory Probing Areas:**
  1. **Data Persistence & Schema:** Do we need new `.proto` messages/fields, or changes to how data is serialized and stored via `pstore`?
  2. **API Boundaries & gRPC Contracts:** Are new gRPC service definitions, RPC methods, or custom request/response models needed?
  3. **Backend Logic & Concurrency:** What Go packages and logic are affected? Are there async queue / Untappd rate-limiting implications (using `processqueue`)?
  4. **CLI Design:** Are new CLI command verbs or flags needed in `beerkellar_cli`?
  5. **Error Handling & Fault Tolerance:** How are API failures or corrupt inputs handled?

### 4. Technical Implementation Plan Formulation
Once a shared understanding of technical details is reached, Beerkellar compiles the blueprint.
* **Format:** The Implementation Plan must adhere to **Option A** structure:
  1. **Proposed Architecture / System Design:** A high-level overview of backend and CLI components.
  2. **Schema & Protocol Buffer Changes:** Specific `.proto` modifications (field numbers, types, message structures).
  3. **Backend (Go) Implementation Details:** File paths, package design, gRPC server methods, database/pstore changes, and logic modifications.
  4. **CLI / Interface Updates:** Commands, flags, and console output details.
  5. **Testing Strategy:** Plan for backend Go tests (`go test -v ./...`), integration tests under `/integration/`, and manual verification steps.

### 5. Post Plan & Create Breakdown Sub-Issue
Beerkellar posts the finalized implementation plan to the sub-issue using premium markdown formatting (collapsible `<details>` blocks, interactive task lists `- [ ]`, Mermaid diagrams, and direct file path links).
* **Action:**
  1. Remove the `seraphine-needs-implementation-plan` label from the current `[Implementation Plan]` issue.
  2. Programmatically create a new sub-issue:
     - **Sub-Issue Title:** `[Breakdown] <Parent Issue Title>`
     - **Sub-Issue Label:** `seraphine-break-down-issue`
     - **Sub-Issue Description:** A link referencing the `[Implementation Plan]` issue and instructing the agent to begin the issue breakdown.
