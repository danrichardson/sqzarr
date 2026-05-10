# Interview Process Improvement Report
**Generated:** 2026-04-02  
**Purpose:** Recommendations to make the Throughline interview process produce documents that are consistent, verifiable, and repeatable — such that any project could be recreated from scratch using only the output documents.  
**Based on:** Deficiencies found in SQZARR (see `deficiency-report.md`)

---

## Framing: What "Spec-Ready" Actually Means

The current interview process treats the output documents as *communication artifacts* — summaries that convey intent to a human reader. That is not enough. For documents to function as a true project backup from which a site could be rebuilt from scratch by any competent agent at any future time, they must function as *executable specifications*: every decision must be recorded, every ambiguity must be resolved, every feature must have a verifiable acceptance criterion.

The root cause of every SQZARR deficiency is the same: **a statement was made in the interview, translated into prose summary, and then re-interpreted by a downstream agent.** Each translation introduced loss. The goal of these recommendations is to eliminate that loss by structuring the interview output so that nothing requires re-interpretation.

---

## Part 1: Interview Stage Reforms

### 1.1 — Make Scope (Stage 4) Non-Skippable

**Problem:** The SQZARR founder skipped Stage 4 and the interviewer accepted it. As a result, the 1.0 scope was never locked, leading to inconsistent half-built features.

**Recommendation:** The interview system must treat Stage 4 as a hard gate. If the user attempts to skip it, the system should respond:

> "I understand you're tired of this. I just need two minutes. Without a scope boundary, the agent will build things you didn't ask for and skip things you did. Give me three things that are NOT in 1.0 and we can finish."

If the user still refuses, the system must insert a visible UNRESOLVED flag at the top of every output document:

```
⚠️ SCOPE UNRESOLVED — Stage 4 was not completed.
The 1.0 definition is inferred, not confirmed. An agent building from
this document should treat all non-core features as OPTIONAL and request
human confirmation before implementing them.
```

This flag must not be suppressable. An agent picking up the document will see it and know to ask.

---

### 1.2 — Add a Dedicated Technical Exclusions Pass

**Problem:** The SQZARR interview captured "skip HEVC and AV1" in Stage 1, but the closing summary generalized it to "configurable criteria (age, bitrate, codec)" — dropping the explicit codec list. The downstream agent only implemented HEVC skip.

**Recommendation:** After Stage 1 closes, the interviewer must run a dedicated extraction pass that asks:

> "Let me verify the exclusion rules. You said files already in HEVC/AV1 should be skipped. Is that list complete? Are there other codecs, formats, file types, or directory patterns that should NEVER be touched, no matter what?"

The output of this pass must be stored as a **Technical Exclusions List** — a verbatim bullet list, not prose — which is injected into every downstream document. No summarization or paraphrasing is permitted.

**Format:**
```
## Technical Exclusions (Non-negotiable — never touch these files)
- Codec: hevc — already HEVC, no re-encode needed
- Codec: av1 — already AV1, already compressed
- Codec: vp9 — already VP9, already compressed
- [any others confirmed by founder]
```

---

### 1.3 — Add a UI Screen Inventory Stage

**Problem:** The current interview never asks "what screens do you expect to see?" The founder described scenarios (*"I pull up the admin panel and I can see status… I want to see why this failed… I want to transcode this file right now"*) but these were never converted into an authoritative screen list. The mockups prompt used a generic screen list, which omitted a Quarantine management screen entirely.

**Recommendation:** Add a mandatory **UI Inventory stage** immediately before the closing summary. The interviewer should say:

> "Before I write the summary, let me verify the screen list. Based on everything you've described, here are the screens I'm expecting. Tell me what's missing or wrong."

Then the interviewer presents a numbered list of screens derived from the transcript and asks the founder to confirm, add, or remove. This list becomes the **authoritative Screen Inventory** and is injected verbatim into the mockups and code_packet prompts.

For each screen, the inventory must include:
1. Screen name and URL path
2. One-line purpose
3. Every interactive control (button, input, selector, toggle) with its action
4. Every data element displayed
5. Edge state behavior: what happens when empty, loading, or an error occurs

**Example entry:**
```
## Screen: Queue  (/queue)
Purpose: Show in-progress and pending transcode jobs; allow manual file submission.

Controls:
- [Text input] "File path" — accepts absolute filesystem path
- [Button] "Queue File" — calls POST /jobs with the input path; clears input on success; shows inline error on failure
- [Button] "Cancel" (per pending job) — calls DELETE /jobs/{id}; removes job from list
- [Button] "Load more" — pagination

Display:
- Running jobs section: filename, progress bar, percent complete, source size, encoder used
- Pending jobs section: filename, source size, cancel button

States:
- Empty: "Queue is empty" centered message
- Error on submit: inline red text below input
```

---

### 1.4 — Add a Settings Boundary Question

**Problem:** The interview never asked "which settings should be configurable in the UI vs. in a config file?" The agent made a reasonable engineering choice (most settings in TOML), but this contradicted the founder's stated desire for a configurable admin panel.

**Recommendation:** Add a **Settings Boundary** question to Stage 6 (Technical Constraints):

> "I'm going to list every configurable parameter we've discussed. For each one, tell me: should it be changeable in the web UI, or only in the config file? You can also say 'both — file for initial setup, UI for runtime changes.'"

The answer is recorded as a **Settings Boundary Table**:

| Parameter | Default Value | Settable in UI | Settable in Config File | Notes |
|-----------|--------------|----------------|------------------------|-------|
| Worker concurrency | 1 | Yes | Yes | UI shows a number input 1–8 |
| Disk free pause threshold | 50 GB | Yes | Yes | UI shows GB input with warning |
| Scan interval | 6 hours | Yes | Yes | UI shows hours selector |
| Quarantine retention | 10 days | Yes | Yes | UI shows days input |
| Plex base URL | — | Yes | Yes | UI shows text input |
| Plex token | — | Yes | Yes | UI shows password input |
| Admin password hash | — | No | Yes | Hash-only, use CLI tool |

This table is injected into the Settings page mockup and the code_packet. Any setting marked "Yes" in "Settable in UI" must have a corresponding editable control in the Settings page. Any agent failing to implement a UI control for a "Yes" setting has made an error, not a judgment call.

---

### 1.5 — Add a Background Processes Inventory

**Problem:** The interview captured "periodic scan" and "ad hoc scan now" but conflated them. The "I want to schedule this scan instead of waiting till Friday" requirement was partly fulfilled (the scan now button) and partly missed (no schedule visibility or change in UI).

**Recommendation:** Add a **Background Processes** question to Stage 2 or 3:

> "This service runs things in the background on a schedule. Let me list what I think those are: directory scan, transcode worker, quarantine cleanup. For each one, should the UI show when it last ran, when it runs next, and let you change the schedule?"

The answer is recorded as a **Background Processes Table**:

| Process | Default Schedule | UI: Show Last Run | UI: Show Next Run | UI: Change Schedule | UI: Trigger Immediately |
|---------|-----------------|-------------------|-------------------|---------------------|------------------------|
| Directory scan | Every 6 hours | Yes | Yes | Yes (hours input) | Yes — "Scan Now" button |
| Transcode worker | Continuous | No (shown as status) | N/A | No | N/A |
| Quarantine cleanup | Daily | No | Yes | No (config only) | No |

This table is injected into the Dashboard mockup (for "next scan in X hours") and the Settings page mockup (for the interval change control).

---

### 1.6 — Add a Backend-to-UI Completeness Check

**Problem:** Quarantine was fully implemented in the backend but had no UI, because there was no process step that asked "for every backend feature, does it need a UI management screen?"

**Recommendation:** Before closing the interview, the interviewer must run a **Backend-UI Completeness Check**: for every backend feature that produces persistent state, ask:

> "Does the user ever need to inspect, manage, or override [feature name] from the UI?"

If the answer is yes, add a screen or control to the Screen Inventory. The check must cover:
- Every background process (scan history, next run, trigger)
- Every safety mechanism (quarantine contents, verification failures, disk guard status)
- Every external integration (Plex status, last notification sent)
- Every error condition (failed jobs, skipped files, probe failures)

If the founder says "no, the config file is enough" for a particular item, that decision is recorded explicitly in the Decision Log with the founder's words as justification.

---

## Part 2: Post-Interview Processing Reforms

### 2.1 — Replace the Prose Summary with a Structured Requirements Document

**Problem:** The current closing summary is five paragraphs of prose. Downstream agents re-interpret it, losing specifics. The summary for SQZARR correctly captured codec exclusions in discussion but the prose summary dropped AV1.

**Recommendation:** Replace the prose summary entirely with a **Structured Requirements Document (SRD)** that uses explicit sections and tabular formats. The SRD becomes the single source of truth from which all other documents are derived. It must contain these sections:

**Section 1: Product Identity**
```
Name: SQZARR
Tagline: [confirmed or TBD]
Version: 1.0
Repository: [github URL or TBD]
```

**Section 2: Feature Requirements Table**

Every feature as a row:

| ID | Feature | Description | In 1.0 | Acceptance Criteria |
|----|---------|-------------|--------|---------------------|
| F-01 | Directory scan | Walk configured directories and enqueue qualifying files | Yes | Given a directory with 3 qualifying files, running a scan enqueues exactly those 3 files |
| F-02 | Skip already-HEVC files | Files with video codec hevc must never be enqueued | Yes | A file with codec=hevc is not inserted into jobs table after scan |
| F-03 | Skip already-AV1 files | Files with video codec av1 must never be enqueued | Yes | A file with codec=av1 is not inserted into jobs table after scan |
| ... | ... | ... | ... | ... |

Every feature discussed in the interview must appear in this table. Acceptance criteria must be specific and testable — not "it works" but "given X, Y happens."

**Section 3: Technical Exclusions List** (verbatim, from §1.2)

**Section 4: Screen Inventory** (verbatim, from §1.3)

**Section 5: Settings Boundary Table** (verbatim, from §1.4)

**Section 6: Background Processes Table** (verbatim, from §1.5)

**Section 7: External Integrations**

| Integration | Purpose | Trigger | API Call | Failure Behavior |
|------------|---------|---------|----------|-----------------|
| Plex | Library rescan after transcode | After successful file replacement | GET /library/sections/{id}/refresh?X-Plex-Token={token} | Log warning, continue — never fail the job |
| ffprobe | Probe source file codec/bitrate/duration | Before enqueue | ffprobe -v quiet -print_format json -show_streams -show_format {path} | Log error, skip file |
| ffmpeg | Hardware transcode | During job processing | [encoder-specific args] | Mark job failed, clean up temp file |

**Section 8: Non-Negotiable Constraints**

A verbatim list of hard constraints, attributed to the founder:

```
- NO Docker. No Docker dependency anywhere. (stated 7+ times, emphatic)
- NO AI-style purple/fluorescent color palettes
- NO commit messages referencing AI authorship
- NO paid tiers, paywalls, or access gating
- NO Java, PHP, Electron
- Security review required before public GitHub publish
```

**Section 9: Scope Boundary**

```
IN SCOPE for 1.0:
- [list]

OUT OF SCOPE for 1.0:
- [list, confirmed by founder]

DEFERRED (noted, not decided):
- [list]
```

If Stage 4 was skipped, Section 9 must contain only:
```
⚠️ SCOPE BOUNDARY NOT CONFIRMED — See top-of-document flag.
```

---

### 2.2 — Add a User Journey Extraction Pass

**Problem:** The founder described features as vivid first-person scenarios (*"I pull up Tailscale and I want to see why this failed"*, *"I want to transcode this file right now boom"*). These scenario statements are the richest requirements signal in the whole interview, but they were treated as color commentary rather than feature specifications.

**Recommendation:** After the interview closes, run a dedicated **User Journey Extraction** pass over the full transcript. For every statement in the pattern "I want to X" or "I need to be able to X" or "I can [do something] and then I see Y," extract a **User Journey card**:

```
Journey: Manual file transcode
Trigger: User wants to immediately transcode a specific file
Actor: Admin user
Preconditions: Service is running; file path is within a configured directory
Steps:
  1. User navigates to Queue page
  2. User enters file path in "File path" text input
  3. User clicks "Queue File" button
  4. UI immediately shows new job in Pending section with the filename
  5. If the file is already queued: inline error "job already exists for this path"
  6. If the path is not in a configured directory: inline error "path is not within a configured directory"
Postconditions: Job appears in queue; worker picks it up on next tick
UI Location: Queue page — "Add File" section at top of page
```

Every User Journey card must reference a screen in the Screen Inventory and a feature row in the Feature Requirements Table. Any journey that doesn't map to an existing screen must trigger the creation of a new screen.

---

### 2.3 — Inject the SRD into Every Downstream Prompt

**Problem:** Each prompt currently receives the full raw transcript, which is ~2,500 words of stream-of-consciousness speech. Downstream agents must re-interpret this, leading to different agents making different inferences from the same source.

**Recommendation:** Each downstream prompt must receive:
1. The **Structured Requirements Document** (§2.1) as its primary source
2. The raw transcript only as supplemental context (for tone/voice questions)
3. Explicit prompt-specific checklists (§3.x below)

The SRD is the authoritative source. If the SRD and the transcript conflict, the SRD wins. The SRD was explicitly reviewed and confirmed; the transcript was not.

---

### 2.4 — Add a Cross-Document Consistency Validator

**Problem:** Currently each document is generated independently. It's possible (and did happen) that the architecture document mentions a feature (quarantine management) that the mockups document never sketches, resulting in the code_packet generating the backend but omitting the UI.

**Recommendation:** After all documents are generated, run a **Cross-Document Consistency Check** that verifies:

1. Every feature ID in the Feature Requirements Table appears in at least one section of the architecture document
2. Every screen in the Screen Inventory has a corresponding mockup in the mockups document
3. Every API endpoint implied by the Screen Inventory's controls appears in the code_packet
4. Every setting marked "UI-configurable" in the Settings Boundary Table has a corresponding control in the Settings mockup
5. Every background process in the Background Processes Table has a "next run" or "last run" indicator in at least one mockup
6. Every User Journey card maps to at least one mockup state

Any inconsistency found by this check must be surfaced as a flagged item in the WORKFLOW.md Decision Log before the documents are handed to an implementation agent.

---

## Part 3: Document Structure Reforms

### 3.1 — WORKFLOW.md as the Single Ground Truth

**Problem:** Currently all documents are derived independently from the transcript. They are parallel, not hierarchical, which means they can diverge.

**Recommendation:** The WORKFLOW.md must be designated the **canonical ground truth document**. All other documents (architecture, mockups, code_packet, draft_summary) are views into WORKFLOW.md, not independent specifications. The relationship is:

```
Interview Transcript
       ↓
Structured Requirements Document (SRD)  ← human-reviewable checkpoint
       ↓
WORKFLOW.md (ground truth)  ← single authoritative document
   ├── architecture.md  (derived: "how to build it")
   ├── mockups.md       (derived: "what it looks like")
   ├── code_packet.md   (derived: "starter implementation")
   └── draft_summary.md (derived: "what it is")
```

The founder should review and approve the SRD before any other document is generated. The WORKFLOW.md is the expansion of the SRD into a full implementation contract.

---

### 3.2 — Add a Requirements Traceability Matrix (RTM) to WORKFLOW.md

**Problem:** There is no way to verify that a built feature satisfies a stated requirement, or to find which requirement a built feature is implementing.

**Recommendation:** WORKFLOW.md must include a **Requirements Traceability Matrix** as a standard section. Format:

| Req ID | Transcript Quote | Feature Description | Acceptance Criterion | Implemented In | Test |
|--------|-----------------|---------------------|---------------------|----------------|------|
| F-02 | "Explicit excludes — files already in HEVC/AV1: skip them." | Skip HEVC files in scanner | Scan of directory containing HEVC file does not insert a job row | `internal/scanner/scanner.go:maybeEnqueue` | `TestScannerSkipsHEVC` |
| F-03 | Same as F-02 | Skip AV1 files in scanner | Scan of directory containing AV1 file does not insert a job row | `internal/scanner/scanner.go:maybeEnqueue` | `TestScannerSkipsAV1` |
| F-11 | "hey I want to transcode this file right now boom" | Manual per-file transcode from Queue UI | User enters path, clicks Queue File, job appears in Pending section | `frontend/src/pages/Queue.tsx` | `TestQueueManualSubmit` (E2E) |

This matrix is populated by the code_packet agent during generation, and updated by any implementation agent during the build. At any point, the count of "Implemented In" cells that are non-empty versus blank gives an immediate completion percentage.

---

### 3.3 — Add a Rebuild-from-Scratch Specification to WORKFLOW.md

**Problem:** The documents as currently structured describe what to build, but not how to verify that it's working. A developer receiving only the documents cannot verify a correct implementation without also reading the code.

**Recommendation:** WORKFLOW.md must include a dedicated **Rebuild from Scratch** section containing everything needed to recreate the project with no prior knowledge:

#### 3.3.1 — Environment & Dependencies

```
## Runtime Dependencies
- Go 1.22+ (backend)
- Node 20+ with pnpm (frontend build)
- ffmpeg 6.0+ with hevc_vaapi / hevc_videotoolbox / hevc_nvenc encoders
- ffprobe (bundled with ffmpeg)
- SQLite 3.x (embedded via go-sqlite3)
- Linux kernel with /dev/dri/renderD128 (for VAAPI) OR macOS (for VideoToolbox)

## Build Steps
1. cd frontend && pnpm install && pnpm build
   - Output: frontend/dist/
2. go build -o sqzarr ./cmd/sqzarr
   - Embeds frontend/dist via go:embed
3. Copy sqzarr binary to /usr/local/bin/
4. Copy sqzarr.toml.example to /etc/sqzarr/sqzarr.toml and edit
5. Run: sqzarr serve
```

Every environment variable and config key must have:
- Name
- Type (string, int, bool)
- Default value
- Description
- Whether it's required or optional
- Example value

#### 3.3.2 — Database Schema with Semantics

The schema is not enough. Each table must be documented with:
- Purpose of the table
- Row lifecycle (when created, when updated, when deleted)
- Key relationships
- Sample row (realistic, not NULL everywhere)

Example:
```
## Table: jobs
Purpose: One row per transcode job. Created by the scanner or by manual API call.
         Updated as the worker processes it. Never deleted — forms the permanent history.

Columns:
- id INTEGER PRIMARY KEY
- directory_id INTEGER NULL  -- NULL for manually-submitted jobs
- source_path TEXT           -- absolute path to the source file
- source_size INTEGER        -- bytes, at time of enqueue
- source_codec TEXT          -- e.g. "h264", "mpeg2video"
- source_duration REAL       -- seconds, from ffprobe
- source_bitrate INTEGER     -- bits per second
- status TEXT                -- "pending" | "running" | "done" | "failed" | "cancelled" | "skipped"
- progress REAL              -- 0.0 to 1.0, updated during transcoding
- bytes_saved INTEGER NULL   -- set on done; NULL if not yet complete or if output was larger
- encoder_used TEXT NULL     -- e.g. "hevc_vaapi", set when job starts
- error_message TEXT NULL    -- set on failure
- priority INTEGER DEFAULT 0 -- higher = picked first; manual jobs use priority=1
- started_at DATETIME NULL
- finished_at DATETIME NULL
- created_at DATETIME

Sample row:
  id=42, directory_id=1, source_path="/media/TV/Show.S01E01.mkv",
  source_size=8589934592, source_codec="h264", source_duration=2580.5,
  source_bitrate=26500000, status="done", progress=1.0,
  bytes_saved=6291456000, encoder_used="hevc_vaapi", error_message=NULL,
  priority=0, started_at="2026-04-01T02:15:00Z", finished_at="2026-04-01T02:32:00Z",
  created_at="2026-04-01T02:00:00Z"
```

#### 3.3.3 — Complete API Surface

Every endpoint must be documented with:
- Method + path
- Authentication required (yes/no)
- Request body (JSON schema)
- Success response (status code + body)
- Error responses (each possible error code + when it occurs)
- Side effects (what state changes)

Example:
```
POST /api/v1/jobs
Auth: Required (Bearer token) if auth is configured; none otherwise
Purpose: Manually enqueue a specific file for transcoding

Request body:
  { "path": "/absolute/path/to/file.mkv" }

Success: 201 Created
  { "ID": 42, "SourcePath": "...", "Status": "pending", ... }

Errors:
  400 Bad Request — body missing or path is empty
  400 Bad Request — path is outside all configured directories
  409 Conflict    — job already exists for this path (any non-terminal status)
  500 Internal Server Error — database failure

Side effects:
  - Inserts row into jobs table with status=pending, priority=1
  - Worker picks up the job within 5 seconds
```

#### 3.3.4 — Smoke Test Checklist

A step-by-step verification procedure that can be run by anyone after a fresh install:

```
## Smoke Test Checklist (run after fresh install, in order)

[ ] 1. Service starts: `sqzarr serve` exits with no error; logs show "worker started"
[ ] 2. Dashboard loads: browser opens http://localhost:8080, sees Dashboard page, no console errors
[ ] 3. Encoder detected: Dashboard shows active encoder (should match hardware)
[ ] 4. Add directory: Navigate to Directories, add "/tmp/sqzarr-test", save, appears in list
[ ] 5. Codec detection works: Create a test file: `ffmpeg -i [h264 source] -t 30 -c copy /tmp/sqzarr-test/test.mkv`
[ ] 6. Manual scan triggers: Click "Scan Now" on Dashboard, check logs for "queued" entry
[ ] 7. Job appears in queue: Navigate to Queue, see test.mkv in Pending
[ ] 8. Job runs: Wait ≤60 seconds, see job move from Pending to Running with progress bar
[ ] 9. Transcode completes: Job appears in History as "done" with bytes_saved > 0
[ ] 10. Plex notified (if configured): Check Plex logs for library refresh event
[ ] 11. Manual file submission: On Queue page, enter "/tmp/sqzarr-test/another.mkv", click Queue File, verify it appears
[ ] 12. Cancel works: Add a file to queue, click Cancel before it starts, verify it shows as "cancelled" in History
[ ] 13. Retry works: Click Retry on a cancelled job, verify it re-enters Pending
[ ] 14. Disk guard: Set pause_threshold_gb higher than actual free space in config, restart, verify queue shows as Paused
[ ] 15. Auth (if configured): Log out, verify protected routes redirect to /login
[ ] 16. HEVC file skipped: Encode a test HEVC file, add to directory, scan, verify it does NOT appear in Queue
[ ] 17. AV1 file skipped: Encode a test AV1 file, add to directory, scan, verify it does NOT appear in Queue
[ ] 18. Quarantine: After a successful transcode, verify original moved to {data_dir}/quarantine/ (if quarantine_enabled=true)
```

---

### 3.4 — Add a Decision Log as a Standard Section

**Problem:** The SQZARR WORKFLOW.md has a Decision Log but it's sparse. Most architectural decisions (why TOML over in-database config, why SQLite, why SSE over WebSockets) are either undocumented or inferred.

**Recommendation:** Every non-obvious decision must be logged in this format:

```
| Decision | Chosen Option | Rejected Options | Reason | Decided By |
|----------|--------------|------------------|--------|------------|
| Config storage | sqzarr.toml file | Database, env vars | Self-hosted tools benefit from human-readable config files; restoring from backup means restoring one file | Architecture agent |
| Realtime updates | SSE (EventSource) | WebSockets, polling | SSE is unidirectional (server→client), simpler to implement, works over HTTP/1.1, no auth complexity | Architecture agent |
| Skip-codec list | Hard-coded: hevc, av1, vp9 | User-configurable | [NOT DECIDED — founder said "HEVC/AV1" but configurability was not asked; assumed hard-coded. Flag for review.] | ⚠️ Inferred |
```

Any decision marked "Inferred" or "⚠️" must be resolved before the document set is considered spec-ready.

---

## Part 4: Per-Prompt Specific Changes

Each prompt template must be updated with the following changes:

### 4.1 — `dan-architecture.md` prompt additions

Add these explicit instructions:

```
For every screen in the Screen Inventory (from the SRD), include a section
describing its API dependencies. Every control action listed must map to a
specific API endpoint.

For every backend feature in the Feature Requirements Table, state explicitly
whether it requires a corresponding UI management screen. Do not assume the
answer — if the SRD is ambiguous, flag it in the Decision Log.

Include a Settings Boundary section that matches the Settings Boundary Table
from the SRD exactly. Any setting marked "UI-configurable" must be served by
a documented API endpoint (e.g., PUT /config or equivalent).

For every background process in the Background Processes Table, include:
- How the UI knows the process ran (SSE event, polling, or stored state)
- What endpoint provides "last run" and "next run" data
- What endpoint triggers it ad hoc (if applicable)
```

---

### 4.2 — `dan-mockups.md` prompt additions

Replace the generic "Screens to cover" section with:

```
You MUST produce a mockup for EVERY screen in the Screen Inventory (from the SRD).
Add no screens that are not in the inventory. Omit no screens that are.

For each screen, include EVERY control listed in that screen's inventory entry.
Do not add controls not listed. Do not omit controls that are listed.

For every control, show its:
- Default state
- Disabled state (if applicable)
- Error state (inline error message, what triggers it)
- Loading/pending state (if it makes an async call)
- Success confirmation (if applicable)

For every screen marked as having a "next run" or "last run" indicator in the
Background Processes Table, include that indicator in the mockup.
```

---

### 4.3 — `dan-code_packet.md` prompt additions

```
You MUST implement every control in the Screen Inventory. For each control:
- If it calls an API endpoint, that endpoint must exist in the backend
- If it's an editable setting marked "UI-configurable" in the Settings Boundary
  Table, it must persist when saved (call an API, not just update local state)

You MUST implement every exclusion rule in the Technical Exclusions List.
Do not implement only the most obvious one. Implement ALL of them. If you are
unsure of the ffprobe codec name for an exclusion, use a comment and flag it.

For every background process in the Background Processes Table:
- If "UI: Show Next Run" is Yes, include the data endpoint and the UI element
- If "UI: Change Schedule" is Yes, include both the API endpoint (PUT /config
  or equivalent) and the UI control

Populate the "Implemented In" column of the Requirements Traceability Matrix
as you write each piece of code. Every feature with a "Yes" in the In 1.0
column must have a non-empty Implementation pointer before you submit.
```

---

### 4.4 — `dan-workflow_md.md` prompt additions

```
The WORKFLOW.md you produce is the GROUND TRUTH document. Every other document
is derived from it. It must contain:

1. The complete Feature Requirements Table from the SRD (do not summarize)
2. The complete Screen Inventory from the SRD (do not summarize)
3. The Settings Boundary Table from the SRD (do not summarize)
4. The Background Processes Table from the SRD (do not summarize)
5. A Requirements Traceability Matrix (Req ID, transcript quote, acceptance
   criterion, implementation pointer — leave pointer blank for agent to fill)
6. A Rebuild from Scratch section (environment, dependencies, build steps,
   config reference, API surface, smoke test checklist)
7. A Decision Log (every non-obvious decision; flag inferred decisions with ⚠️)

Every phase in the Phase Plan must have a Definition of Done that includes:
- A specific acceptance criterion for every feature added in that phase
- A reference to the smoke test step(s) that verify it
- Explicit statement of what is NOT done yet (so the next phase's agent knows
  where to start without re-reading the whole document)
```

---

## Part 5: The "Rebuild from Scratch" Standard

The goal is that the WORKFLOW.md (and documents derived from it) must function as a complete, self-contained specification. Specifically: **a developer who has never seen the codebase, who only has the WORKFLOW.md and the sqzarr.toml.example, must be able to recreate the project to a functionally identical result.**

To verify this is true, apply the following test after generating documents:

**The Burn Test:** Delete the codebase. Give only the WORKFLOW.md and the SRD to a fresh agent. Ask it to build the project. If the result has any of the deficiencies listed in `deficiency-report.md`, the WORKFLOW.md is incomplete. Repeat until the Burn Test passes.

For the Burn Test to pass, the WORKFLOW.md must satisfy:

| Check | What's Needed |
|-------|---------------|
| No ambiguous exclusion lists | Technical Exclusions List is verbatim, not prose |
| No missing UI controls | Every control in Screen Inventory; all screen inventory entries complete |
| No read-only settings page surprises | Settings Boundary Table complete; all "UI-configurable" settings in Screen Inventory |
| No background process visibility gaps | Background Processes Table complete; Dashboard/Settings mockups show schedule indicators |
| No scope ambiguity | Scope Boundary section confirmed; or UNRESOLVED flag clearly visible |
| No inferred decisions | Decision Log marks every assumption; none left unmarked |
| No untestable acceptance criteria | Every RTM row has a specific, binary acceptance criterion |
| No missing smoke tests | Smoke Test Checklist covers every feature in Feature Requirements Table |

---

## Part 6: Recommended Document Set Structure

After these changes, the output of a completed interview should be:

```
/interview-output/
├── 00-srd.md                  ← Structured Requirements Document (human-reviewed gate)
├── WORKFLOW.md                ← Ground truth; contains RTM, rebuild spec, decision log
├── architecture.md            ← Technical design derived from WORKFLOW.md
├── mockups.md                 ← UI mockups for every screen in Screen Inventory
├── code_packet.md             ← Starter implementation with RTM pointers
└── draft_summary.md           ← Plain-language 1-pager for sharing
```

The **00-srd.md** is new and is the key addition. It is:
- Generated first, from the interview transcript
- Human-reviewed before any other document is produced
- The authoritative input for all other documents
- Small enough (~500 lines) to review in under 30 minutes

If the founder approves the SRD, all downstream documents can be generated and trusted. If the founder rejects or amends the SRD, documents are regenerated from the corrected SRD before any code is written.

This single gate — the founder-approved SRD — prevents every category of deficiency found in SQZARR.

---

## Summary: Changes Mapped to Deficiencies Fixed

| Recommendation | Deficiency Fixed |
|----------------|-----------------|
| 1.2 Technical Exclusions Pass | DEF-1: AV1 not excluded |
| 2.2 User Journey Extraction Pass | DEF-2: No manual transcode UI |
| 1.3 UI Screen Inventory Stage | DEF-2, DEF-3: No quarantine UI, no manual transcode UI |
| 1.6 Backend-UI Completeness Check | DEF-3: Quarantine has no UI |
| 1.4 Settings Boundary Question | DEF-4: Settings page is read-only |
| 1.5 Background Processes Inventory | DEF-5: No scan schedule UI |
| 1.1 Stage 4 Non-Skippable | PROC-1: Scope undefined |
| 2.1 Structured Requirements Document | PROC-2: Summary lost specificity |
| 2.2 User Journey Extraction | PROC-3: Conversational reqs not formalized |
| 2.3 Inject SRD into prompts | PROC-2, PROC-3: Multiple agents re-interpreting transcript |
| 2.4 Cross-Document Consistency Validator | All deficiencies: catches mismatches before handoff |
| 3.2 Requirements Traceability Matrix | All deficiencies: verifiable completeness at any time |
| 3.3 Rebuild-from-Scratch Spec | All deficiencies: site is recreatable without reading the code |
| 3.4 Decision Log with Inferred flags | PROC-2, PROC-3: Assumptions surfaced, not buried |
| 5.0 Burn Test standard | All deficiencies: post-generation verification |
