# SQZARR — UI Mockups

**Design language**: Sandstone palette. Warm stone tones, amber accent. Clean, calm, utilitarian with just enough personality to feel crafted. No purple. No emoji clusters. Think "a tool made by someone who cares."

**Color tokens** (Tailwind):
- Background: `bg-stone-50` (light) / `bg-stone-900` (dark)
- Surface: `bg-white` / `bg-stone-800`
- Border: `border-stone-200` / `border-stone-700`
- Text primary: `text-stone-900` / `text-stone-50`
- Text muted: `text-stone-500`
- Accent (active/running): `text-amber-600` / `bg-amber-500`
- Success: `text-green-600`
- Error: `text-red-600`
- Nav active: `bg-stone-100` border-l-2 `border-amber-500`

---

## Screen 1: Login

**Purpose**: Optional gate if admin password is configured. Skipped entirely if no password is set.

### ASCII Wireframe

```
┌─────────────────────────────────────────────────────┐
│                                                     │
│                                                     │
│              ┌─────────────────────┐                │
│              │  ▓ SQZARR            │                │
│              │  Media Transcoder   │                │
│              └─────────────────────┘                │
│                                                     │
│              ┌─────────────────────┐                │
│              │ Password            │                │
│              │ [________________] │                │
│              │                     │                │
│              │ [   Sign In   ]     │                │
│              └─────────────────────┘                │
│                                                     │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### Key Interactive Elements
- Password input (type=password, autofocus)
- Sign In button (submits on Enter)

### Tailwind Component Spec
```
<div class="min-h-screen bg-stone-50 flex items-center justify-center">
  <div class="w-full max-w-sm space-y-6 p-8">
    <div class="text-center">
      <h1 class="text-2xl font-semibold text-stone-900 tracking-tight">SQZARR</h1>
      <p class="text-sm text-stone-500 mt-1">Media Transcoder</p>
    </div>
    <form class="space-y-4">
      <Input type="password" placeholder="Password" class="w-full" />
      <Button class="w-full bg-stone-800 hover:bg-stone-700 text-white">Sign In</Button>
    </form>
  </div>
</div>
```

### States
- **Default**: empty form
- **Loading**: button shows spinner, disabled
- **Error**: red border on input, "Incorrect password" below input

---

## Screen 2: Dashboard (Main Home)

**Purpose**: Land here after login. Status at a glance — is it running, what's happening, how much space has it saved. First 30 seconds should make the user say "yep, it's working."

### ASCII Wireframe

```
┌──────────────────────────────────────────────────────────────────┐
│  ▓ SQZARR                                         [● Running]     │
├──────────┬───────────────────────────────────────────────────────┤
│          │                                                        │
│  Nav     │  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│          │  │  127 GB   │  │  843     │  │  12      │           │
│ Dashboard│  │  Saved    │  │  Done    │  │  Failed  │           │
│          │  └──────────┘  └──────────┘  └──────────┘           │
│ Queue    │                                                        │
│   (3)    │  ── Currently Running ─────────────────────────────── │
│          │                                                        │
│ History  │  /media/tv/Breaking.Bad.S01E01.mkv                   │
│          │  [████████████████░░░░░░░░░░░░]  62%                 │
│ Dirs     │  9.2 GB → ~3.4 GB  ·  Intel VAAPI  ·  12m elapsed   │
│          │                                                        │
│ Settings │  ── Up Next (3 jobs) ────────────────────────────────│
│          │                                                        │
│          │  Breaking.Bad.S01E02.mkv     8.8 GB  ·  pending      │
│          │  Breaking.Bad.S01E03.mkv     9.1 GB  ·  pending      │
│          │  The.Wire.S01E01.mkv         6.2 GB  ·  pending      │
│          │                                                        │
│          │  ── Disk Space ──────────────────────────────────── │
│          │  /media  [████████████████████░░░░░]  78% used       │
│          │  Free: 892 GB  ·  Pause threshold: 50 GB             │
│          │                                                        │
│          │  Last scan: 2 hours ago · Next scan: in 4 hours      │
│          │  [Scan Now]                                            │
└──────────┴───────────────────────────────────────────────────────┘
```

### Key Interactive Elements
- Stat cards (clickable, navigate to History with filter)
- Progress bar (live-updating via WebSocket)
- "Up Next" list (click file to go to job detail)
- Disk space bar (turns amber at 80%, red at 90%)
- "Scan Now" button (triggers immediate scan)
- Global status pill (green = running, yellow = paused, gray = idle)

### Tailwind Component Spec
```
Layout: sidebar nav (w-48, hidden on mobile → bottom tab bar)
Main content: p-6, max-w-4xl

Stat cards:
  <div class="grid grid-cols-3 gap-4">
    <div class="bg-white rounded-lg border border-stone-200 p-4">
      <div class="text-2xl font-bold text-stone-900">127 GB</div>
      <div class="text-sm text-stone-500">Saved</div>
    </div>
  </div>

Progress bar:
  <div class="h-2 bg-stone-100 rounded-full overflow-hidden">
    <div class="h-full bg-amber-500 transition-all duration-500" style="width: 62%" />
  </div>

Disk space bar:
  Normal: bg-green-500
  Warning (>80%): bg-amber-500  
  Critical (>90%): bg-red-500
```

### States
- **Active transcode**: shows current job + progress
- **Idle (queue empty)**: "Nothing queued — last scan found no candidates" with Scan Now CTA
- **Paused (disk warning)**: amber banner at top: "Paused: free space below 50 GB threshold"
- **Error**: red banner for any daemon error

---

## Screen 3: Job Queue

**Purpose**: See what's pending, cancel jobs, manually enqueue a file.

### ASCII Wireframe

```
┌──────────────────────────────────────────────────────────────────┐
│  Queue                                     [+ Add File]  [⏸ Pause]│
├──────────┬───────────────────────────────────────────────────────┤
│          │                                                        │
│  Nav     │  ┌─ Running ────────────────────────────────────────┐ │
│          │  │ Breaking.Bad.S01E01.mkv                [Cancel]  │ │
│ Dashboard│  │ 9.2 GB · Intel VAAPI                             │ │
│          │  │ [████████████████████░░░░]  78%  · 4m remaining  │ │
│ Queue ●  │  └──────────────────────────────────────────────────┘ │
│          │                                                        │
│ History  │  ┌─ Pending (2) ────────────────────────────────────┐ │
│          │  │                                                    │ │
│ Dirs     │  │  Breaking.Bad.S01E02.mkv   8.8 GB  h264  [×]    │ │
│          │  │  Breaking.Bad.S01E03.mkv   9.1 GB  h264  [×]    │ │
│ Settings │  │                                                    │ │
│          │  └──────────────────────────────────────────────────┘ │
│          │                                                        │
│          │  ┌─ Add File Manually ──────────────────────────────┐ │
│          │  │ Path: [/media/tv/________________]  [Enqueue]    │ │
│          │  └──────────────────────────────────────────────────┘ │
│          │                                                        │
└──────────┴───────────────────────────────────────────────────────┘
```

### Key Interactive Elements
- Cancel button on running job (prompts confirm: "Cancel current transcode? Original is safe.")
- Cancel (×) on pending jobs
- Pause/Resume queue toggle
- Manual enqueue: path input + Enqueue button
- "Add File" modal with file path input

### Tailwind Component Spec
```
Running job card:
  <div class="border border-amber-200 bg-amber-50 rounded-lg p-4">

Pending job row:
  <div class="flex items-center justify-between py-3 border-b border-stone-100">
    <span class="font-mono text-sm text-stone-700 truncate">filename.mkv</span>
    <div class="flex items-center gap-3">
      <Badge variant="outline">h264</Badge>
      <span class="text-stone-500 text-sm">8.8 GB</span>
      <Button variant="ghost" size="sm">×</Button>
    </div>
  </div>

Pause button:
  Active: bg-amber-100 text-amber-700 border-amber-300
  Paused: bg-stone-100 text-stone-600
```

### States
- **Empty queue, idle**: "Queue is empty. SQZARR will add files automatically on the next scan."
- **Paused**: all pending rows dimmed, resume button prominent
- **Running + pending**: running card on top, list below

---

## Screen 4: History

**Purpose**: Completed and failed jobs. See what was done, how much was saved per file, investigate failures.

### ASCII Wireframe

```
┌──────────────────────────────────────────────────────────────────┐
│  History                              [Filter ▾] [Clear Failed]  │
├──────────┬───────────────────────────────────────────────────────┤
│          │                                                        │
│  Nav     │  Filter: [All ▾]  [Any encoder ▾]  [Date range]      │
│          │                                                        │
│ Dashboard│  File                        Before  After   Saved    │
│          │  ─────────────────────────────────────────────────── │
│ Queue    │  Breaking.Bad.S01E01.mkv     9.2 GB  2.8 GB  6.4 GB ✓│
│          │  Breaking.Bad.S01E02.mkv     8.8 GB  2.6 GB  6.2 GB ✓│
│ History ●│  The.Simpsons.S01E01.mkv     5.1 GB  FAILED  —      ✗│
│          │  The.Wire.S01E01.mkv         6.2 GB  1.9 GB  4.3 GB ✓│
│ Dirs     │  Archer.S01E01.mkv           4.8 GB  1.4 GB  3.4 GB ✓│
│          │                                                        │
│ Settings │  [Load more]                                           │
│          │                                                        │
└──────────┴───────────────────────────────────────────────────────┘

Clicking a failed row expands error detail:
  ┌────────────────────────────────────────────────────────────────┐
  │ ✗ The.Simpsons.S01E01.mkv — Failed                            │
  │                                                                │
  │ Error: ffmpeg exit code 1                                      │
  │ Output: Invalid data found when processing input               │
  │                                                                │
  │ Source: /media/tv/The.Simpsons/Season.01/The.Simpsons.S01E01  │
  │ Encoder: Intel VAAPI                                           │
  │ Time: 2026-04-01 14:32:11 · Duration: 2m 14s                  │
  │                                                                │
  │ [Retry]  [Skip This File]                                      │
  └────────────────────────────────────────────────────────────────┘
```

### Key Interactive Elements
- Filter by status (all / done / failed / skipped)
- Filter by encoder used
- Click row to expand detail / error message
- Retry button on failed jobs
- "Skip This File" permanently skips a file from future scans
- Clear Failed button removes all failed entries

### Tailwind Component Spec
```
Table row:
  Done: text-stone-700, savings in text-green-600
  Failed: text-stone-500, "FAILED" in text-red-500
  Row hover: bg-stone-50

Expanded error:
  <div class="bg-red-50 border border-red-200 rounded-lg p-4 mt-1 text-sm font-mono">

Savings badge:
  <span class="text-green-600 font-medium">-6.4 GB</span>
```

### States
- **Empty**: "No jobs completed yet. Start a scan to process your library."
- **All done**: green-tinted, shows total savings prominently
- **With failures**: red count badge on History nav item

---

## Screen 5: Directories

**Purpose**: Add and configure watched directories. Set per-directory rules. Preview what files would be queued.

### ASCII Wireframe

```
┌──────────────────────────────────────────────────────────────────┐
│  Directories                                    [+ Add Directory] │
├──────────┬───────────────────────────────────────────────────────┤
│          │                                                        │
│  Nav     │  ┌─────────────────────────────────────────────────┐  │
│          │  │ /media/tv                              [●] [Edit]│  │
│ Dashboard│  │ Min age: 7 days · Max bitrate: 4 Mbps           │  │
│          │  │ 843 files · 23 queued · Last scan: 2h ago       │  │
│ Queue    │  └─────────────────────────────────────────────────┘  │
│          │                                                        │
│ History  │  ┌─────────────────────────────────────────────────┐  │
│          │  │ /media/movies                          [●] [Edit]│  │
│ Dirs ●   │  │ Min age: 30 days · Max bitrate: 8 Mbps          │  │
│          │  │ 412 files · 8 queued · Last scan: 2h ago        │  │
│ Settings │  └─────────────────────────────────────────────────┘  │
│          │                                                        │
└──────────┴───────────────────────────────────────────────────────┘

Edit panel (slide-in drawer or inline expand):
┌─────────────────────────────────────────────────────────────────┐
│ Edit: /media/tv                                           [✕]   │
│                                                                  │
│ Path          /media/tv                                          │
│ Enabled       [● On]                                             │
│                                                                  │
│ ── Rules ─────────────────────────────────────────────────────  │
│ Min file age  [7] days   (skip files newer than this)            │
│ Max bitrate   [4] Mbps   (skip files already below this)        │
│ Min file size [500] MB   (skip tiny files)                       │
│                                                                  │
│ ── Codec Excludes (global) ────────────────────────────────── │
│ ✓ HEVC / H.265  ✓ AV1  ☐ VP9                                   │
│                                                                  │
│ [Preview Matches]                                                │
│                                                                  │
│ [Save]  [Cancel]  [Delete Directory]                             │
└─────────────────────────────────────────────────────────────────┘

Preview panel:
┌─────────────────────────────────────────────────────────────────┐
│ Preview — files that would be queued (23 matches)               │
│                                                                  │
│ File                            Size    Bitrate    Age          │
│ ────────────────────────────────────────────────────────────── │
│ Breaking.Bad.S01E01.mkv         9.2 GB  8.4 Mbps  14d          │
│ Breaking.Bad.S01E02.mkv         8.8 GB  7.9 Mbps  14d          │
│ ...                                                              │
│                                                                  │
│ [Close]                                                          │
└─────────────────────────────────────────────────────────────────┘
```

### Key Interactive Elements
- Enable/disable toggle per directory
- Edit button → slide-in drawer with rule configuration
- Number inputs for age, bitrate, size thresholds
- "Preview Matches" → shows dry-run list before committing
- Save / Cancel / Delete controls

### Tailwind Component Spec
```
Directory card:
  <div class="border border-stone-200 rounded-lg p-4 bg-white hover:border-stone-300 transition-colors">

Drawer:
  <Sheet> (shadcn/ui) — slides from right, w-96 on desktop, full-screen on mobile

Toggle:
  <Switch> (shadcn/ui) — amber when on

Number input:
  <div class="flex items-center gap-2">
    <Input type="number" class="w-24" />
    <span class="text-stone-500 text-sm">days</span>
  </div>

Preview table:
  Scrollable, max-h-96, sticky header
```

### States
- **No directories**: "No directories configured. Add one to get started." with big CTA
- **Disabled directory**: card is dimmed, toggle is off
- **Preview loading**: spinner in preview panel
- **Preview empty**: "No files match these rules — try adjusting your thresholds"

---

## Screen 6: Settings

**Purpose**: System-wide config — scan schedule, hardware info, quarantine settings, Plex integration, optional password.

### ASCII Wireframe

```
┌──────────────────────────────────────────────────────────────────┐
│  Settings                                                         │
├──────────┬───────────────────────────────────────────────────────┤
│          │                                                        │
│  Nav     │  ── Hardware ───────────────────────────────────────  │
│          │  Detected encoder:  Intel VAAPI  [hevc_vaapi]  ✓      │
│ Dashboard│  [Re-detect]                                           │
│          │                                                        │
│ Queue    │  ── Scan Schedule ──────────────────────────────────  │
│          │  Scan interval    [6] hours                            │
│ History  │  Concurrent jobs  [1] (max 8)                         │
│          │                                                        │
│ Dirs     │  ── File Safety ────────────────────────────────────  │
│          │  Quarantine mode  [● On]                               │
│ Settings●│  Retention period [10] days                            │
│          │  Quarantine path  [/media/.sqzarr-quarantine  ]        │
│          │  Disk space pause [50] GB free                         │
│          │                                                        │
│          │  ── Plex Integration ───────────────────────────────  │
│          │  [☐] Enable Plex rescan after transcode                │
│          │  Server URL   [http://plex.local:32400      ]         │
│          │  Auth token   [********************          ]  [Test] │
│          │                                                        │
│          │  ── Admin Password ─────────────────────────────────  │
│          │  [☐] Require password to access this panel             │
│          │  Password  [__________________________]                │
│          │  (leave blank to disable authentication)               │
│          │                                                        │
│          │  ── About ──────────────────────────────────────────  │
│          │  SQZARR v1.0.0 · Go 1.22 · SQLite 3.45                │
│          │  ffmpeg 6.1.1 · ffprobe 6.1.1                         │
│          │                                                        │
│          │  [Save Settings]                                       │
│          │                                                        │
└──────────┴───────────────────────────────────────────────────────┘
```

### Key Interactive Elements
- Re-detect hardware button (re-runs probe, shows spinner)
- Plex Test button (calls API with current URL+token, shows "Connected" or error)
- Quarantine toggle (disabling shows warning: "Originals will be deleted immediately after replacement")
- Password field (empty = no auth)
- Save button (writes config file)

### Tailwind Component Spec
```
Section headers:
  <h2 class="text-xs font-semibold uppercase tracking-wider text-stone-400 mb-3">Hardware</h2>

Hardware status:
  Detected: <Badge class="bg-green-100 text-green-700">Intel VAAPI</Badge>
  Not detected: <Badge class="bg-stone-100 text-stone-500">Software only</Badge>

Plex test result:
  Success: text-green-600 "Connected to Plex 1.32.4"
  Failure: text-red-600 "Connection failed: timeout"

Quarantine disable warning:
  <Alert variant="destructive">Disabling quarantine means originals are permanently deleted immediately after replacement. This cannot be undone.</Alert>

Save button:
  <Button class="bg-stone-800 hover:bg-stone-700 text-white">Save Settings</Button>
  On success: brief green "Saved" text beside button for 2s
```

### States
- **Hardware not detected**: amber warning, suggests checking VAAPI passthrough
- **Plex disabled**: Plex URL/token fields hidden/disabled
- **Auth disabled**: password field hidden
- **Save success**: brief confirmation, no full-page reload
- **Save error**: red error message below Save button

---

## Mobile Layout Notes

On viewports < 768px:
- Sidebar nav collapses to bottom tab bar (Dashboard | Queue | History | Dirs)
- Settings accessible via hamburger in top-right
- Stat cards stack 1-column
- Job progress takes full width
- Directory drawer becomes full-screen sheet
- Tables get horizontal scroll or collapse to stacked card format
