# SQZARR — Interview Process Improvement Report
**Date:** 2026-04-02
**Original deliverables:** c:\src\_project-sqzarr\docs\downloaded-prompts\
**Improved config_manifest:** c:\src\_project-sqzarr\docs\sqzarr-config-manifest.md
**Original deficiency report:** c:\src\_project-sqzarr\docs\deficiency-report.md

---

## What the Improved Process Fixed

### Deficiencies caught by config_manifest

| Deficiency | Original outcome | Improved process outcome | How it was caught |
|---|---|---|---|
| Stage 4 abandoned — 1.0 scope never locked | Agent inferred scope from conversational cues; produced quarantine backend without quarantine UI; settings page reads TOML only | Improvement #6: when founder refuses/avoids scoping, interviewer offers out-of-scope checklist. config_manifest Scope section flags: "OPEN_ITEM: Stage 4 abandoned — no exclusions formally logged." | The structured 10-item Stage 7 checklist (Improvement #2) also forces scope confirmation before `[INTERVIEW_CONFIRMED]` can be issued. Founder either answers the scope question or leaves an explicit OPEN_ITEM — the silence-equals-done path is closed. |
| AV1/VP9 not excluded from scan | Only HEVC was skipped; AV1 files would be re-encoded unnecessarily | config_manifest Scope section lists "Skip files already encoded in HEVC or AV1 (stated in Stage 1)" as an explicit in-scope item. ASSUMPTION #1 logs the VP9 gap. | Improvement #7 (ambiguity flagging) would produce: "ASSUMPTION: VP9 exclusion was not addressed. Treating as include (will be transcoded). Founder should confirm." This surfaces the gap for review rather than letting the agent silently implement only the most obvious case. |
| Per-file manual transcode trigger had no UI | API endpoint built (`POST /api/v1/jobs`); no UI control existed | The conversational requirement ("boom, transcode this file right now") would now survive into the config_manifest as an explicit in-scope UI feature: "Manual per-file transcode trigger from UI." | Sequential chaining (Improvement #11): the mockups prompt receives the config_manifest as context, sees "Manual per-file transcode trigger from UI" as an explicit line item, and must produce a mockup for it. The UI omission happened because the mockups prompt only saw the raw transcript, not a structured feature list. |
| Settings page read-only; key parameters not configurable in UI | Concurrency, disk threshold, scan interval, Plex URL — all TOML-only | config_manifest Open Items flags: "Settings UI vs. config file boundary — it was never resolved which settings live in the web UI vs. a config file." | Improvement #7 would log: "ASSUMPTION: concurrency, disk threshold, scan interval are file-configurable only. Founder said 'configure directories… when it meets the criteria that I set' — this implies UI configuration. Requires clarification." |
| Quarantine UI missing | Backend quarantine fully implemented; no way to view, approve, or reject from web panel | config_manifest Scope section includes quarantine description as in-scope feature. OPEN_ITEM #3 explicitly flags: "Quarantine UI — backend quarantine described but no UI for viewing/managing quarantined files was specified." | The structured config_manifest forces the mockups prompt to either include a Quarantine screen or explicitly mark it OPEN_ITEM. The old process had no mechanism to catch that the backend spec and the UI spec were mismatched. |
| Closing summary never confirmed | Transcript ends with "Does this capture it?" — no explicit founder confirmation of Stage 1–7 summary | Improvement #2: Stage 7 produces structured 10-item checklist and waits for YES. Improvement #1: `[INTERVIEW_CONFIRMED]` token required before artifacts generate. | In the SQZARR transcript, the founder also did not explicitly say "yes" to the closing summary. The same gap as Erik's transcript — interview completion was inferential, not confirmed. |
| Conversational requirements lost specificity in summary | "configurable criteria (age, bitrate, codec)" generalized away AV1; "boom, transcode it" never became a UI spec | config_manifest In-1.0 Scope list converts every conversational statement into a discrete, concrete line item. Sequential chaining delivers this list to all downstream prompts. | Improvement #11 (sequential chaining) is the mechanical fix here. When the mockups prompt receives "Manual per-file transcode trigger from UI — Queue page, text input + submit button" as context, it cannot miss it. When the code_packet receives the architecture, it cannot diverge. |

---

### OPEN_ITEM flags generated

From the config_manifest, the following gaps remain even after the improved interview:

1. **1.0 scope boundary** — Stage 4 was abandoned. No exclusion list exists. The agent must infer 1.0 scope from stated features plus conservative defaults.
2. **Additional excluded codecs beyond HEVC/AV1** — VP9 and others were not addressed. Should the skip list be user-configurable from the UI, or hardcoded?
3. **Quarantine UI** — Backend quarantine behavior was described (move, wait, auto-delete after N days). The UI for viewing, approving, and rejecting quarantined files was never specified.
4. **Settings UI vs. config file boundary** — Founder wants to "configure things" from the admin panel but the boundary between UI-settable and file-settable was never drawn. Specifically unresolved: concurrency, disk threshold, scan interval, Plex URL/token, quarantine retention days.
5. **Scan schedule UI** — "Schedule this scan instead of waiting till Friday" implies a recurring schedule editor, but this was conflated with the ad-hoc "Scan Now" trigger. Whether the UI should include a schedule editor (vs. just a manual trigger) was not confirmed.
6. **Project name/tagline** — "SQZARR" confirmed. Tagline was not resolved. Founder offered "save space R Us" and "transcode your shit and it works" as jokes and said "I don't know, you figure it out."
7. **CT 340 rebuild** — Founder said "tear that shit down." Whether to rebuild at CT 340 or a new CT number was not specified.
8. **Mac Mini migration timeline** — Whether macOS support is a 1.0 requirement or a post-launch consideration was not locked.
9. **Security review process** — Required before GitHub publish; no reviewer, checklist, or tooling was named.

---

### ASSUMPTION flags generated

The following judgment calls were previously made silently and are now visible for founder review:

1. **ASSUMPTION: VP9 not in the skip list.** Founder said "HEVC/AV1: skip them." VP9 was not mentioned. The closing summary generalized to "configurable criteria (codec)" without listing VP9. The improved process logs this gap explicitly rather than letting the agent silently implement only HEVC skip.
2. **ASSUMPTION: Output codec is HEVC.** Founder never explicitly said "transcode to HEVC." This was inferred from the Tdarr replacement context and the existing ffmpeg command. It is almost certainly correct but was never stated as a direct requirement.
3. **ASSUMPTION: Single transcode at a time is the 1.0 default; concurrency is configurable.** Founder said "process one file at a time for now but have the option to do it in a queue." The word "now" was interpreted as "1.0 default" rather than "never allow parallelism."
4. **ASSUMPTION: Admin password is optional.** Founder said "maybe have an option for a password." The word "maybe" was interpreted as optional feature. If the founder considers a password mandatory for any public-facing panel, this assumption is wrong.
5. **ASSUMPTION: Plex integration is optional.** Founder said the service "won't give a shit if you're running Plex or not." This was interpreted as "make Plex opt-in via config, not required." If the founder uses Plex for all media management, they may expect Plex notification to be on by default.
6. **ASSUMPTION: ffmpeg is the transcoding engine.** Founder said "I don't care if you use ffmpeg or not" but required hardware transcoding on Intel VAAPI, Apple Silicon VideoToolbox, and Nvidia NVENC. ffmpeg is the only practical multi-platform engine covering all three. This was treated as the correct implied choice.
7. **ASSUMPTION: "Runs natively" means no containerization whatsoever.** Docker hatred was interpreted as a blanket prohibition on all container-based deployment, including Podman, LXC application containers, etc. (Note: Proxmox LXC as the host environment is distinct from running the app inside a Docker container — the former is acceptable, the latter is not.)

---

### Contradictions the consistency_audit would have caught

Based on the original artifacts (project summary, architecture, code starter packet), the `consistency_audit` deliverable would have flagged the following contradictions:

**Codec exclusion list — summary vs. code conflict:**
- Stage 1 transcript: "Explicit excludes — files already in HEVC/AV1: skip them."
- Closing summary: "configurable criteria (age, bitrate, codec)" — AV1 not listed
- scanner.go: `strings.EqualFold(probe.codec, "hevc")` — only HEVC checked, AV1 not checked
- consistency_audit output: "CONTRADICTION: Transcript explicitly names AV1 as a skip target. Closing summary generalized to 'codec' without listing AV1. Implementation skips only HEVC. AV1 files will be re-transcoded. Fix: add AV1 to the skip list in maybeEnqueue."

**Per-file manual trigger — spec vs. UI conflict:**
- Transcript Stage 2: "I want to transcode this file right now boom"
- architecture/code_packet: `POST /api/v1/jobs` endpoint exists and is implemented
- UI: no input or button exists on any screen to call this endpoint
- consistency_audit output: "CONTRADICTION: Manual file transcode trigger is required per transcript. Backend endpoint exists. No UI element exists to invoke it. The Queue page has no 'Add file' control. This feature is half-built."

**Settings configurability — spec vs. implementation conflict:**
- Transcript Stage 2: "I need to be able to configure directories… when it meets the criteria that I set"
- project-summary: "clean admin panel where you can configure things"
- Settings page: reads sqzarr.toml, shows active encoder, says "edit sqzarr.toml to change settings"
- consistency_audit output: "CONTRADICTION: Transcript and project summary both require in-UI configuration. Settings page is read-only except for Directories. Concurrency, disk threshold, scan interval, and Plex settings are not configurable from the UI."

**Quarantine — backend vs. UI completeness conflict:**
- Transcript Stage 3: "move into temporary location… if you don't reject the transcode after 10 days then it'll delete"
- Backend: quarantine folder implemented, auto-deletion after retention_days implemented
- UI: no quarantine screen, no way to view held files, no approve/reject controls
- consistency_audit output: "CONTRADICTION: Quarantine backend is complete. Quarantine UI is absent. The transcript implies the user can 'reject' a transcode (restore original), which requires a UI action. No such action exists."

---

## What the Improved Process Would NOT Have Fixed

**DEF-1 (AV1 skip) — partially.** The improved interview would have flagged the VP9 gap via ASSUMPTION logging and potentially surfaced AV1/VP9 in the Stage 3 external dependency question. But if the founder's answer is still "HEVC and AV1," VP9 would still be an inferred exclusion. The consistency_audit would catch the implementation gap (HEVC only vs. HEVC+AV1 specified), but cannot add VP9 to the list unless the founder names it.

**DEF-3 (Quarantine UI) — partially.** The improved process would have flagged the quarantine UI as an OPEN_ITEM and the consistency_audit would have caught the backend/UI mismatch. However, the founder's description of quarantine was still vague about *what the UI should show*. Even a well-formed interview might not produce a complete quarantine screen specification without a more structured mockup pass.

**DEF-4 (Settings read-only) — the root cause is architectural.** The decision to put most config in sqzarr.toml was a reasonable call for a self-hosted service. The improved interview would have flagged the UI/config boundary as an OPEN_ITEM, but the founder's preference (everything in UI vs. professional TOML for services) was genuinely ambiguous. This gap requires an explicit design conversation, not just better intake.

**Stage 4 boredom** — The founder explicitly said "I'm kind of getting bored with this interview so let's just move on." Improvement #8 (shortening offramp) addresses this by offering to accelerate the interview, but a founder who refuses to scope is a founder who refuses to scope. The improved process can offer an out-of-scope checklist (Improvement #6) and log explicit OPEN_ITEMs, but it cannot force a disengaged founder to complete scope definition. The SQZARR interview had a fundamentally harder founder than Erik's — more opinionated, more tangential, more likely to redirect. The improvements help at the margins but the root issue is process adherence, not process design.

**Deployment and infrastructure details** — CT number for rebuild, Mac Mini migration timeline, security review tooling — these are operational details that fall outside the interview's purpose. The config_manifest correctly flags them as OPEN_ITEMs, but no interview improvement resolves them; they require follow-up conversations or build-time decisions.

---

## Net Assessment

The SQZARR project started from a stronger foundation than Erik's — most of the core features shipped correctly, and the implementation quality was high. The gaps that existed were primarily cases where conversational requirements didn't survive the trip from transcript to mockup to implementation. With the improved process, three of the five material gaps would have been meaningfully addressed. Sequential chaining (Improvement #11) would have delivered the config_manifest's explicit feature list — including "manual per-file transcode trigger from UI" and "quarantine management UI" — directly into the mockups prompt, making it essentially impossible for those controls to be omitted because there was no mockup to reference. The out-of-scope checklist (Improvement #6) would have forced a scope conversation even when the founder was disengaged, producing at minimum an OPEN_ITEM rather than a silent non-answer. The consistency_audit (Improvement #10) would have caught all four backend/UI mismatches before the project was declared done. The two gaps that remain hardest to fix — settings configurability and founder disengagement at Stage 4 — are architectural and human problems respectively, and no interview improvement fully solves either. Overall, the improved process would have elevated SQZARR from "solid but with material UI gaps" to "complete and internally consistent on first delivery" — a meaningful difference in rework cost and founder confidence.
