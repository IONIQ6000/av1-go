# Go-based AV1 Daemon + Bubble Tea TUI (Cursor Project Spec)

You are my **senior Go engineer** and TUI architect, running inside Cursor. I want to build a Go project that replaces my Tdarr-based AV1 workflow with a **self-contained, robust daemon + TUI**, while preserving all of the key behaviors of the Tdarr flow we designed earlier.

Think of it as:

> **“Tdarr’s AV1 brain, rewritten as a Go service with a Bubble Tea dashboard.”**

The application should be **entirely in Go**, using:

- **Go 1.25** (latest stable).
- A **built-in static `ffmpeg` 8.x build** that the app will **download automatically** from:  
  `https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n8.0-latest-linux64-gpl-8.0.tar.xz`
- **Bubble Tea** (`github.com/charmbracelet/bubbletea`) for the TUI.
- Intel **QSV AV1** encoding (targeting Intel Arc A310).
- All the logic & rules we previously implemented in Tdarr:

  - AV1 QSV pipeline
  - Minimum size threshold (e.g. < 2 GiB → skip)
  - “Already AV1” detection via probe
  - WebRip heuristics (VFR, container type, odd dimensions)
  - Web-safe timestamp & padding flags
  - Wait-for-stable file size before transcoding
  - Remove Russian audio and subtitle tracks
  - `.why.txt` explanatory sidecar files
  - `.av1skip` marker files to permanently skip a title
  - Size gate: new file must be ≤ 90% of original size, or it is rejected
  - Atomic replace of the original file on success

Target environment: **Linux**, running on a media server (likely in Docker) with an **Intel GPU** and VA-API/QSV enabled.

---

## Overall Architecture

Use a single Go module with multiple commands:

- Root module: `github.com/yourname/av1qsvd` (name is flexible)
- Commands:
  - `cmd/av1d` → the **daemon** that watches folders and runs ffmpeg jobs.
  - `cmd/av1top` → the **Bubble Tea TUI** that shows system metrics and job status.

Rough layout:

```text
.
├── go.mod
├── go.sum
├── cmd
│   ├── av1d
│   │   └── main.go
│   └── av1top
│       └── main.go
└── internal
    ├── config
    │   └── config.go
    ├── ffmpeg
    │   ├── binary.go       # download/extract/verify ffmpeg
    │   └── transcode.go    # build and run ffmpeg commands
    ├── metadata
    │   └── probe.go        # ffprobe logic, WebRip heuristics
    ├── jobs
    │   └── jobs.go         # job model, state, persistence
    ├── scan
    │   └── scan.go         # directory scanning & candidate selection
    ├── daemon
    │   └── daemon.go       # main loop & scheduling
    └── tui
        ├── model.go        # Bubble Tea model & messages
        ├── view.go         # rendering logic
        └── update.go       # state transitions
```

This is a suggested layout; refine as needed but keep responsibilities separated.

---

## Go Version & Dependencies

Use **Go 1.25**.

`go.mod` should look like:

```go
module github.com/yourname/av1qsvd

go 1.25
```

Dependencies (added via `go get`):

- Config / JSON / I/O:
  - Standard library: `encoding/json`, `os`, `io`, `path/filepath`, `time`, `os/exec`
- TUI:
  - `github.com/charmbracelet/bubbletea`
  - `github.com/charmbracelet/bubbles`
  - `github.com/charmbracelet/lipgloss`
- System info:
  - `github.com/shirou/gopsutil/v4` (CPU, memory, disk)
- Filesystem watching (optional; can start with polling):
  - `github.com/fsnotify/fsnotify`
- Compression:
  - `github.com/ulikunitz/xz` for `.tar.xz` decompression
- Logging:
  - For now, just `log` from stdlib.

Keep the dependency set reasonably small and organized.

---

## Core Functional Requirements (Tdarr Parity)

The Go application must replicate the key behaviors of the Tdarr AV1 flow:

### 1. Use bundled static ffmpeg 8.x

On startup, ensure an ffmpeg binary exists in an app data directory, e.g.:

```text
~/.local/share/av1qsvd/ffmpeg/ffmpeg
```

If not present:

1. Download the archive:  
   `https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n8.0-latest-linux64-gpl-8.0.tar.xz`
2. Decompress `.xz` using `github.com/ulikunitz/xz`.
3. Untar the resulting tarball.
4. Locate the `ffmpeg` binary inside that archive.
5. Copy it into the app’s ffmpeg directory (e.g. `~/.local/share/av1qsvd/ffmpeg/ffmpeg`).
6. `chmod +x` the binary.

Keep this logic in `internal/ffmpeg/binary.go`.

After installation, **verify**:

- `ffmpeg -version` starts with `ffmpeg version 8.` or `ffmpeg version n8.0`.
- `ffmpeg -hide_banner -encoders` output includes `av1_qsv`.
- A quick QSV test works:

  ```bash
  ffmpeg -hide_banner -v error     -init_hw_device qsv=hw -filter_hw_device hw     -f lavfi -i testsrc2=s=1280x720:d=1     -vf format=nv12,hwupload=extra_hw_frames=64     -frames:v 1 -c:v av1_qsv -global_quality 30 -f null -
  ```

If any of these fail, return an error and let the daemon exit with a clear message. Do not run in a partially broken state.

---

### 2. Folder Watching / Scanning

- Support one or more **library root directories** in config.
- The daemon:
  - Periodically scans for candidate media files (e.g. `.mkv`, `.mp4`, `.m4v`).
  - Optionally, use `fsnotify` to trigger scans on change, but v1 can just do periodic scans (e.g. every 60 seconds).
- For each file, apply the filters below.

---

### 3. Skip Rules

For each candidate file:

- Skip if a sidecar file exists:
  - `<basename>.av1skip`  
    (same path as the media file, different extension)
- Determine file size via `os.Stat`:
  - If file size is **≤ 2 GiB**, skip with reason: `"file < 2GB"`.
- Run `ffprobe` or equivalent:

  - Use `ffmpeg -hide_banner -v quiet -print_format json -show_streams -show_format` or a dedicated `ffprobe`.
  - Parse JSON to identify:
    - video streams
    - codecs
    - frame rates
    - resolution
    - container / format data.

- If no video streams → skip with `"not a video"`.
- If **any video stream codec** is already `av1` → skip with `"already av1"`.

---

### 4. Wait-for-stable-file Rule

Before scheduling or starting a transcode:

- Check file size at time `t0`.
- Sleep for N seconds (e.g. 10).
- Check size at `t1`.
- Optionally repeat a couple of times.
- If size changed → treat as `"file still copying"`:
  - Skip for now (no `.av1skip`).
  - Record in `.why.txt`.

This avoids transcoding partially-copied files.

---

### 5. WebRip Heuristics

Using `ffprobe` metadata:

- Parse:
  - `format_name` (containers)
  - video stream `avg_frame_rate`
  - video stream `r_frame_rate`
  - video stream `width` / `height`

A file is considered **WebRip-like** if **any** of the following:

- `format_name` contains `"mp4"`, `"mov"`, or `"webm"`.
- Any video stream has `avg_frame_rate != r_frame_rate` (VFR).
- Any video stream has **odd** dimensions (width or height not divisible by 2).

For WebRip-like inputs:

- Add input flags:

  ```bash
  -fflags +genpts -copyts -start_at_zero
  ```

- Add output flags:

  ```bash
  -vsync 0 -avoid_negative_ts make_zero
  ```

- Enforce even dimensions and a sane SAR:

  ```bash
  -vf:v:0 "pad=ceil(iw/2)*2:ceil(ih/2)*2,setsar=1,format=<surface>,hwupload=extra_hw_frames=64"
  ```

For non-WebRip content, you can still use the pad + format + hwupload chain, but without the web-specific timestamp flags.

---

### 6. AV1 QSV Encoding Parameters

For the chosen video stream:

1. **Select the main video:**

   - Prefer video stream with `disposition.default == 1`.
   - Else, pick the first video stream.

2. **Quality by height:**

   - `height >= 1440` → `global_quality = 23`
   - `height >= 1080 && < 1440` → `global_quality = 24`
   - `< 1080` → `global_quality = 25`

3. **Surface format by bit depth:**

   - If bit depth ≥ 10 → `p010`.
   - Else → `nv12`.

4. **FFmpeg mapping** (video + audio + subs):

   Start from all streams, then prune:

   ```bash
   -map 0
   -map -0:v        # remove all video
   -map -0:t        # remove attachments
   -map 0:v:<vord>  # add only main video
   -map 0:a?        # all audio
   -map -0:a:m:language:rus
   -map -0:a:m:language:ru
   -map 0:s?        # all subtitles
   -map -0:s:m:language:rus
   -map -0:s:m:language:ru
   -map_chapters 0
   ```

This removes **Russian audio and subtitles** by language tag but keeps everything else.

5. **Video filter chain** (for WebRip-like):

   ```bash
   -vf:v:0 "pad=ceil(iw/2)*2:ceil(ih/2)*2,setsar=1,format=<surface>,hwupload=extra_hw_frames=64"
   ```

6. **Codec & tuning:**

   ```bash
   -c:v:0 av1_qsv
   -global_quality:v:0 <qual>
   -preset:v:0 medium
   -look_ahead 1
   ```

7. **Audio & subs: passthrough**

   ```bash
   -c:a copy
   -c:s copy
   ```

8. **Container / muxing:**

   ```bash
   -max_muxing_queue_size 2048
   -map_metadata 0
   -f matroska
   -movflags +faststart
   ```

9. **QSV / input configuration:**

   ```bash
   -hwaccel none
   -init_hw_device qsv=hw
   -filter_hw_device hw
   -analyzeduration 50M
   -probesize 50M
   ```

In Go, construct the ffmpeg command as `[]string` (slice of args) rather than concatenating a single string.

---

### 7. Size Gate & Sidecar Markers

After a transcode:

- `origBytes` = original file size.
- `newBytes` = converted file size.

If **`newBytes > origBytes * 0.90`**:

- Treat as **rejected**.
- Write `<basename>.why.txt`:

  ```text
  rejected: new 1234.5 MB vs orig 1500.0 MB (>90%)
  ```

- Write `<basename>.av1skip` (content can be simple, e.g. `"skip"`).
- Delete the converted file.
- Keep the original.
- Mark job status as `Skipped` with that reason.

Else (accepted):

- Atomically replace original:
  - Write to `<basename>.av1-tmp.mkv`.
  - Once done & verified:
    - Either rename tmp to original and move original aside as `.orig.mkv`, or
    - Overwrite original in place.
- Mark job as `Success` and record `newBytes`.

---

### 8. `.why.txt` Explanatory Files

For any file that is **skipped or rejected**, write `<basename>.why.txt` summarizing the reason, e.g.:

- `"file < 2GB"`
- `"already av1"`
- `"not a video"`
- `"file still copying"`
- `"ffmpeg exit code X"`
- `"av1_qsv encoder missing"`
- `"size gate: new > 90% of original"`

This gives filesystem-level visibility into what the daemon decided and why.

---

## Job Model & State Persistence

In `internal/jobs`, define a model like:

```go
type JobStatus string

const (
    JobStatusPending JobStatus = "pending"
    JobStatusRunning JobStatus = "running"
    JobStatusSuccess JobStatus = "success"
    JobStatusFailed  JobStatus = "failed"
    JobStatusSkipped JobStatus = "skipped"
)

type Job struct {
    ID           string     `json:"id"`
    SourcePath   string     `json:"source_path"`
    OutputPath   string     `json:"output_path,omitempty"`
    CreatedAt    time.Time  `json:"created_at"`
    StartedAt    *time.Time `json:"started_at,omitempty"`
    FinishedAt   *time.Time `json:"finished_at,omitempty"`
    Status       JobStatus  `json:"status"`
    Reason       string     `json:"reason,omitempty"`
    OriginalSize int64      `json:"original_bytes,omitempty"`
    NewSize      int64      `json:"new_bytes,omitempty"`
    IsWebRipLike bool       `json:"is_webrip_like"`
}
```

Store job state as **JSON files** in a configured directory, e.g.:

```text
~/.local/share/av1qsvd/jobs/<job_id>.json
```

Helper functions:

```go
func SaveJob(job *Job, jobsDir string) error
func LoadAllJobs(jobsDir string) ([]*Job, error)
```

The daemon should:

- Create a Job when a file is first identified as a candidate.
- Update transitions:
  - `Pending` → `Running` → `Success` / `Failed` / `Skipped`.

The TUI will read from this jobs directory to display job lists and details.

---

## Config

In `internal/config`, create a minimal config system:

```go
type TranscodeConfig struct {
    FFmpegURL        string   `json:"ffmpeg_url"`
    FFmpegInstallDir string   `json:"ffmpeg_install_dir"`
    LibraryRoots     []string `json:"library_roots"`
    MinBytes         int64    `json:"min_bytes"`          // e.g. 2 GiB
    MaxSizeRatio     float64  `json:"max_size_ratio"`     // e.g. 0.90
    JobStateDir      string   `json:"job_state_dir"`
    ScanIntervalSec  int      `json:"scan_interval_sec"`  // e.g. 60
}

func DefaultConfig() TranscodeConfig
func LoadConfig(path string) (TranscodeConfig, error)
```

For v1 it’s fine if `LoadConfig` just returns `DefaultConfig()` and ignores the file path, but structure it so reading from JSON or TOML later is easy.

---

## Daemon Behavior (`cmd/av1d`)

In `cmd/av1d/main.go`:

1. Load config (or use `DefaultConfig()`).
2. Ensure ffmpeg is installed and verified via `internal/ffmpeg/binary`.
3. Enter a main loop:

   - Every `ScanIntervalSec` seconds:
     - Scan each `LibraryRoots` directory recursively for media files.
     - For each file:
       - Check `.av1skip` & existing Job records.
       - Run file size checks.
       - Run ffprobe & heuristics.
       - Apply skip rules.
       - If it passes, create or update a Job and enqueue it.

4. Implement a simple job runner:

   - For v1, allow **one active job at a time**.
   - For each job:
     - Mark as `Running`, set `StartedAt`.
     - Build ffmpeg args with `internal/ffmpeg/transcode` (based on metadata and rules).
     - Run `exec.CommandContext(ffmpegPath, args...)`.
     - Capture exit code.
     - If ffmpeg failed, mark as `Failed` and write `.why.txt`.
     - If ffmpeg succeeded, run size gate.
     - If size gate passes, atomically replace original and mark `Success`.
     - If size gate fails, delete new file, write `.av1skip`, mark `Skipped`.

5. Log key events with `log.Printf`:

   - “Discovered file …”
   - “Skipping … reason: …”
   - “Starting job …”
   - “ffmpeg completed with exit code …”
   - “Job succeeded, savings: x%”
   - “Job rejected by size gate …”

---

## TUI (`cmd/av1top`) with Bubble Tea

Use **Bubble Tea** + `bubbles` + `lipgloss` to build a TUI that:

- Shows **system metrics**:
  - CPU usage
  - Memory usage
  - (Optional) disk usage for library paths
- Shows a **job list**, reading from the job JSONs:
  - Columns:
    - `STATUS`
    - `FILE` (basename of `SourcePath`)
    - `RES` (if available, e.g. 1920x1080, optional)
    - `ORIG_SIZE`
    - `NEW_SIZE`
    - `SAVINGS` (%)
    - `DURATION`
    - `REASON` (for skipped/failed)

### Layout (first pass)

- **Top row**: CPU and memory usage bars (e.g. `bubbles/progress`).
- **Middle**: Full-width table listing jobs, newest first.
- **Bottom**: Status line showing:
  - total jobs / running / failed / skipped
  - job state directory path
  - key hints: `q` = quit, `r` = refresh

### Behavior

- The Bubble Tea model should:
  - Periodically (e.g. every 1 second) tick:
    - Re-read job JSONs from disk.
    - Poll system metrics via `gopsutil`.
  - Handle keys:
    - `q` → quit.
    - `r` → immediate refresh.

Don’t overcomplicate selection/details yet; a single table view is enough for v1.

---

## Coding Style & Expectations

In Cursor, treat this spec as the **authoritative description** of the project.

Guidelines:

- Use **clear, explicit Go code**; readability over clever tricks.
- Prefer standard library where possible.
- Keep `internal` packages small and focused:
  - `ffmpeg` → ffmpeg binaries & command construction.
  - `metadata` → ffprobe & heuristics.
  - `jobs` → job model & persistence.
  - `daemon` → high-level orchestration.
  - `tui` → Bubble Tea state, update, and view.
- Add brief comments to key functions and exported types.

---

## What I Want Cursor To Do First

1. **Initialize the Go module and layout**:
   - Create `go.mod` with `go 1.25`.
   - Create `cmd/av1d`, `cmd/av1top`, and `internal/...` structure as described.
   - Add stubs for key internal packages and types.

2. Implement in `internal/config`:
   - `TranscodeConfig`, `DefaultConfig`, and a stub `LoadConfig`.

3. Implement in `internal/ffmpeg/binary`:
   - Functions to:
     - Check if ffmpeg exists at the configured path.
     - If not, download, extract, locate `ffmpeg`, place it in install dir, and mark executable.
     - Verify version, `av1_qsv` presence, and run the small QSV test.
   - Return `error` on failure so `av1d` can exit cleanly.

4. Implement in `internal/jobs`:
   - `JobStatus`, `Job` struct.
   - `SaveJob(job *Job, jobsDir string) error`
   - `LoadAllJobs(jobsDir string) ([]*Job, error)`

5. Implement in `cmd/av1d/main.go`:
   - Wire config + ffmpeg install/verification.
   - Perform a **single scan pass** of library roots and print which files would be queued or skipped, with reasons (no actual ffmpeg yet).

6. Implement a minimal `cmd/av1top/main.go`:
   - Set up a Bubble Tea program with a placeholder model.
   - Draw a dummy job table and a simple status bar to validate layout & key handling.
   - Later, swap dummy data for real jobs from `internal/jobs`.

After these basics compile and run, iterate:

- Flesh out `internal/metadata` (ffprobe parsing and heuristics).
- Implement the real ffmpeg transcode logic in `internal/ffmpeg/transcode`.
- Connect `av1top` to the actual job JSONs and real system metrics.

Use this document as the **project spec** inside Cursor. Start by generating the module skeleton and foundational code, then implement features incrementally while keeping everything idiomatic, testable, and easy to extend.
