# WebRip Classifier Design

## Overview

The AV1 daemon uses a **scored heuristic classifier** to determine if a video source is web-like (requiring web-safe transcoding flags) or disc-like (standard flags). This replaces the previous simple boolean check with a more robust, explainable system.

## Classification Types

- **`SourceWebLike`**: Web-ripped content (Netflix, Amazon, etc.) - requires web-safe flags
- **`SourceDiscLike`**: Disc-based content (Blu-ray remuxes, etc.) - standard flags
- **`SourceUnknown`**: Ambiguous content - treated conservatively as web-like

## Scoring System

The classifier accumulates a score from multiple signals:

- **Positive score** → Web-like
- **Negative score** → Disc-like
- **Near zero** → Unknown

**Thresholds:**
- `score >= +2.0` → `SourceWebLike`
- `score <= -2.0` → `SourceDiscLike`
- Otherwise → `SourceUnknown` (treated as web-like for safety)

## Signals and Weights

### 1. Filename/Folder Tokens (Strong Signals)

**Web-leaning tokens** (+3.0 each):
- `web-dl`, `webrip`, `webhd`, `webdl`
- `nf`, `amzn`, `dsnp`, `hmax`, `hulu`, `atvp`, `disney`, `appletv`

**Disc-leaning tokens** (-4.0 each):
- `bluray`, `bdrip`, `brrip`, `remux`, `uhd`, `bd25`, `bd50`, `blu-ray`, `bd-remux`, `bdr`

**Directory tokens** (weaker: +1.0 web, -2.0 disc)

### 2. Container & Muxing Info

**File extension:**
- `.mp4`, `.mov`, `.webm` → +2.0
- `.mkv` → -1.0

**Format name:**
- MP4/MOV containers → +2.5
- WebM (not Matroska) → +2.5
- Matroska → -1.5

**Muxing app/writing library** (strong signal: ±3.0):
- Web-leaning: `shaka-packager`, `libwebm`, `applehttp`, `dash`, `hls`, `ffmpeg`
- Disc-leaning: `mkvmerge`, `libmatroska`, `makemkv`, `tsmuxer`

### 3. Frame Rate Behavior

**Variable Frame Rate (VFR)** → +2.5
- Only counted if format is NOT Matroska (disc remuxes shouldn't have VFR)
- Detected when `avg_frame_rate != r_frame_rate`

### 4. Dimensions & Aspect Ratio

**Odd dimensions** (only if NOT Matroska):
- Odd width → +1.5
- Odd height → +1.5

**Unusual aspect ratio** (< 1.3 or > 2.5) → +0.5

### 5. Bitrate vs Resolution (Weak Signal)

**Low bitrate** for resolution (< 0.1 bpp at 1080p+) → +1.0
**High bitrate** for resolution (> 0.3 bpp at 1080p+) → -1.0

## Explicit Overrides

You can override the classifier using sidecar files:

- **`.websafe`** → Force `SourceWebLike` (score: +10.0)
- **`.nowebsafe`** → Force `SourceDiscLike` (score: -10.0)

Place these files next to the video file with the same basename.

## Output Files

For each classified file, the daemon creates:

- **`.av1qsvd-classification.txt`**: Contains classification decision, score, and all reasons
- **`.av1qsvd-why.txt`**: Contains skip/reject reasons (if applicable)

## Logging

The daemon logs classification decisions with:
- Source class (`WebLike`, `DiscLike`, `Unknown`)
- Score
- All contributing reasons

Example log output:
```
→ ✓ ACCEPTED: movie.mkv (source: DiscLike, score: -3.5, codec: hevc, resolution: 3840x2160)
  Classification reasons: filename: contains 'remux'; format: matroska (often disc remux); muxer: mkvmerge (disc-leaning)
```

## Integration

The classifier is integrated into the AV1 pipeline:

1. **Probing**: `ProbeFile()` calls `ClassifyWebSource()` and stores the decision
2. **Transcoding**: `TranscodeArgs()` receives `isWebRipLike` boolean (derived from `SourceDecision.IsWebLike()`)
3. **Logging**: Classification details are logged and written to sidecar files

## Backward Compatibility

The old `IsWebRipLike` boolean field is maintained for backward compatibility:
- It's set to `SourceDecision.IsWebLike()` (which treats `Unknown` as web-like)
- Existing code using `IsWebRipLike` continues to work
- New code should use `SourceDecision` for detailed information

## Tuning

To adjust classification behavior:

1. **Modify weights** in `ClassifyWebSource()` function
2. **Adjust thresholds** (currently ±2.0)
3. **Add new signals** (e.g., codec-specific heuristics)
4. **Use sidecar files** for specific files that are misclassified

The classifier is designed to be transparent and tunable - all decisions include explainable reasons.

