# SQZARR Deficiency Report
**Generated:** 2026-04-02  
**Source:** Throughline interview transcript vs. implemented site at http://192.168.29.211:8080/

---

## Executive Summary

The SQZARR implementation is solid and covers most of the transcript's requirements well: Go backend, React frontend, sandstone palette, hardware-agnostic encoding (VAAPI/VideoToolbox/NVENC), file safety (verifier + quarantine), Plex rescan, SSE live updates, disk-space guard, and a working Directories CRUD. However, five material gaps exist between what the transcript requested and what shipped. Additionally, three interview process deficiencies explain *why* those gaps exist.

---

## Part 1: Site Feature Deficiencies

### DEF-1 — AV1 (and other already-compressed codecs) not skipped
**Severity: High**  
**Transcript reference:** Stage 1 — *"Explicit excludes — files already in HEVC/AV1: skip them."*

**What shipped:** `internal/scanner/scanner.go` only checks `strings.EqualFold(probe.codec, "hevc")`. Files already encoded in AV1, VP9, or other modern codecs will be re-encoded unnecessarily.

**Why it happened:** The interviewer captured the AV1 exclusion in Stage 1, but the final summary paragraph only says *"configurable criteria (age, bitrate, codec)"* without listing AV1 explicitly. The code_packet prompt inherited this vagueness, so the agent implemented only the HEVC skip — the one that was most obvious from context.

**Fix:** Add AV1 and VP9 to the skip list in `maybeEnqueue`. Ideally make the excluded-codec list configurable in `sqzarr.toml`.

---

### DEF-2 — No per-file manual transcode trigger in the UI
**Severity: High**  
**Transcript reference:** Stage 2 — *"hey I want to transcode this file right now boom and it just transcoded it then I can see the status and like Yay it's transcoding."*

**What shipped:** The backend has `POST /api/v1/jobs` accepting a `{ path: string }` body, and `api.createJob()` is wired in `lib/api.ts`. But no page renders a UI element to use it. There is no text input or "Add file" button anywhere in the Queue or Dashboard screens.

**Why it happened:** This requirement was expressed conversationally ("boom, transcoded it") rather than as a structured feature in Stage 4. The mockups prompt asked for standard screens (dashboard, queue, settings) and the per-file trigger was never called out as a required control on any specific screen. The agent implemented the API endpoint but skipped the UI because it had no mockup to reference.

**Fix:** Add a "Transcode a file" form to the Queue page — a text input for the absolute path + a submit button that calls `api.createJob(path)`.

---

### DEF-3 — Quarantine has no UI management
**Severity: Medium**  
**Transcript reference:** Stage 3 — *"maybe even have an option to like move things into a temporary location and like if you don't reject the transcode after 10 days then it'll delete that stuff and actually free up the space."*

**What shipped:** The quarantine backend is fully implemented — originals are moved to `{data_dir}/quarantine/` after a successful transcode and auto-deleted after `quarantine_retention_days`. But there is zero UI for this. The admin cannot see what's in quarantine, how many days remain on each file, manually approve (delete early), or reject (restore) a transcoded file.

**Why it happened:** The transcript describes quarantine as a safety mechanism but the discussion of *managing* quarantine from the UI was vague. The mockups prompt listed standard screens and didn't include a "Quarantine" screen. The agent correctly built the backend but had no UI spec to work from.

**Fix:** Add a Quarantine page listing held originals with filename, size, days remaining, and two actions: "Release original" (restore + delete the transcode) and "Confirm & delete" (purge early).

---

### DEF-4 — Settings page is read-only; key parameters aren't configurable in the UI
**Severity: Medium**  
**Transcript reference:** Stage 2 — *"I need to be able to configure directories on my location… when it meets the criteria that I set it's going to do stuff."*  
Stage 3 — *"maybe I have an option in the UI that says yeah dude you can do like five streams at once."*

**What shipped:** The Settings page (`/settings`) shows the active encoder and tells you to edit `sqzarr.toml` to change anything. The following settings exist only in the TOML file with no UI equivalent:
- Worker concurrency (parallel transcode count)
- Disk free pause threshold
- Scan interval
- Quarantine retention days
- Plex URL and token

This contradicts the "clean admin panel where you can configure things" requirement. Directories are configurable in-UI (good), but everything else is not.

**Why it happened:** The code_packet prompt instructs the agent to build a starter packet but doesn't itemize which settings belong in the UI vs. the config file. The architecture prompt correctly placed most config in a TOML file (a reasonable call for a self-hosted service), but this decision was never validated with the founder. The interview didn't resolve the boundary between "UI-configurable" and "file-configurable."

**Fix (minimum):** Expose concurrency and disk-threshold in the Settings UI as editable fields that write back to the running config via a new `PUT /config` endpoint. The scan schedule (cron-style or hours) and Plex settings should follow.

---

### DEF-5 — No scan schedule UI
**Severity: Low-Medium**  
**Transcript reference:** Stage 2 — *"I want to schedule this scan now instead of waiting till Friday or whatever."*

**What shipped:** `interval_hours = 6` is hardcoded in `sqzarr.toml.example`. There is no UI to see when the next scan will run or to change the frequency. "Scan Now" exists but "run at 2am every night" does not.

**Why it happened:** The transcript request was for *ad-hoc* scheduling ("I want to schedule this scan now instead of waiting") which the interviewer correctly captured as a trigger feature. But the "change the recurring schedule from the UI" implication was not extracted as a distinct requirement.

**Fix:** Add a "Next scan in X hours" indicator to the Dashboard. Add an interval selector to Settings. A stretch goal would be a cron expression field.

---

## Part 2: Interview Process Deficiencies

These explain *why* the above gaps exist — problems in how the interview translated into prompts, not just what the agent built.

---

### PROC-1 — Stage 4 (1.0 Scope) was abandoned without resolution
**What happened:** The founder said *"I'm kind of getting bored with this interview so let's just move on with it"* and the Stage 4 questions about "three things NOT in 1.0" were never answered. The interviewer accepted this and moved on.

**Impact:** The 1.0 scope was never locked. No exclusions were formally logged. The agent had to infer what to build with no "out of scope" guardrails, which produced a reasonable first pass but included some items (e.g., quarantine backend without quarantine UI) in an inconsistent half-done state.

**Recommendation:** The interview system should treat Stage 4 as non-optional for a "spec-ready" output. If the user skips it, the output document should explicitly flag `## SCOPE UNRESOLVED — 1.0 definition not confirmed` rather than silently proceeding.

---

### PROC-2 — The final summary lost specificity on codec exclusions
**What happened:** The interviewer correctly heard "HEVC/AV1 skip" in Stage 1, but the closing summary paragraph generalized to *"configurable criteria (age, bitrate, codec)"* without listing the specific excluded codecs. The code_packet prompt inherits this generalization.

**Impact:** The agent implemented HEVC skip (the most obvious case) but missed AV1 — a direct line-item from the transcript.

**Recommendation:** The summary section of the throughline interview should be structured as an explicit requirements list, not prose. Each stated feature/exclusion should survive verbatim into the summary so downstream agents don't have to re-interpret.

---

### PROC-3 — Conversational requirements ("boom, transcode it") weren't formalized into UI specs
**What happened:** Several requirements were stated as vivid scenarios in the founder's voice (*"hey I want to transcode this file right now boom"*, *"maybe I have an option in the UI that says five streams at once"*). The interviewer logged these in summary prose but never converted them into structured feature requirements with a specified UI location.

**Impact:** The mockups prompt listed standard screens (dashboard, queue, settings) and the agents produced those screens correctly — but had no mockup for a "manual file enqueue" control or a concurrency slider, so those controls were omitted entirely.

**Recommendation:** The interview post-processing should include a "UI features extraction" pass that converts conversational descriptions into a flat list of explicit UI elements with their screen location. Example: *"Queue page: text input + button to manually enqueue a file by path."* This list should be injected into both the mockups and code_packet prompts.

---

## Summary Table

| # | Gap | Severity | Root Cause |
|---|-----|----------|------------|
| DEF-1 | AV1 (and VP9) not excluded from scan | High | Summary prose lost specificity (PROC-2) |
| DEF-2 | No per-file manual transcode in UI | High | Conversational req not formalized (PROC-3) |
| DEF-3 | Quarantine has no UI management | Medium | No mockup spec for quarantine screen |
| DEF-4 | Settings page is read-only | Medium | UI vs. config boundary never resolved (PROC-1) |
| DEF-5 | No scan schedule UI | Low-Med | Partial capture — trigger vs. schedule conflated |
| PROC-1 | Stage 4 abandoned, 1.0 scope undefined | Process | Interview didn't enforce Stage 4 completion |
| PROC-2 | Summary generalized specific codec exclusions | Process | Closing summary too abstract |
| PROC-3 | Conversational requirements not formalized to UI specs | Process | No "UI features extraction" pass in post-processing |

---

## What Is Working Well (Don't Change)

- Stone/sandstone palette is correct and clean — not purple, not AI-slop
- SSE live updates on the dashboard work well
- Hardware encoder detection (VAAPI → VideoToolbox → NVENC → software) is exactly right
- Directory CRUD with per-directory criteria is solid
- Verifier (size + duration check before replacement) matches the transcript precisely
- Plex rescan after replacement is implemented correctly
- JWT auth with bcrypt password hash is the right approach
- History page with expandable error details and retry is good UX
