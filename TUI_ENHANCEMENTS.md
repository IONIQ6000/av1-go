# TUI Enhancements - Verbose Mode

## Overview

The TUI has been enhanced to provide comprehensive information similar to Tdarr, showing detailed file metadata, encoding progress, and system resource usage.

## New Features

### 1. Active Job Display
Shows detailed real-time information about the currently running transcode:

```
⚡ ACTIVE TRANSCODE
File: movie.mkv
Resolution: 1920x1080
Source Codec: h264 (8-bit)
Frame Rate: 23.976 fps
Container: matroska,webm
Streams: 2 audio, 3 subtitle
Original Size: 4.2 GB
Estimated Size: 2.1 GB (50.0% reduction)
Elapsed Time: 2h15m30s
Type: WebRip (VFR/odd dimensions)
```

### 2. Expanded Job Table
New columns added to show comprehensive information:

| Column | Description |
|--------|-------------|
| STATUS | Job status (PENDING/RUNNING/SUCCESS/FAILED/SKIPPED) |
| FILE | Filename |
| CODEC | Source video codec |
| RESOLUTION | Video resolution (e.g., 1920x1080) |
| ORIG_SIZE | Original file size |
| NEW_SIZE | Final transcoded file size |
| EST_SIZE | Estimated output size (for pending/running jobs) |
| SAVINGS | Actual space savings percentage |
| DURATION | Transcode duration |
| REASON | Skip/failure reason if applicable |

### 3. Unified System Metrics
All system metrics (CPU, MEM, GPU) now use consistent formatting:

```
CPU: ████████████████░░░░  85.2%  MEM: ██████████░░░░░░░░░░  52.3%  GPU: ███████████████░░░░░  78.5%
```

- Same bar width (20 characters)
- Same percentage format (5.1f with proper alignment)
- Consistent spacing

### 4. Additional Metadata in Jobs

Jobs now store and display:
- **Source codec**: Original video codec (h264, h265, vp9, etc.)
- **Resolution**: Video dimensions
- **Bit depth**: 8-bit, 10-bit, etc.
- **Frame rate**: FPS of source video
- **Container**: File container format
- **Stream counts**: Number of audio and subtitle streams
- **Estimated size**: Predicted output file size (50% of original as baseline)

## How It Works

### Metadata Collection
When scanning files, the daemon now:
1. Runs ffprobe to extract comprehensive metadata
2. Parses video codec, resolution, frame rate, bit depth
3. Counts audio and subtitle streams
4. Identifies container format
5. Estimates output size based on typical AV1 compression ratios

### Display Updates
The TUI updates every second showing:
- Real-time system resource usage
- Currently active transcode with full details
- Complete job history with all metadata
- Color-coded status (green=success, red=failed, yellow=skipped, blue=running, gray=pending)

## Estimation Algorithm

**Output size estimation**: 
- Default: 50% of original size
- This is conservative for AV1 at medium quality
- Actual results will vary based on:
  - Source codec efficiency
  - Content complexity
  - Resolution and bitrate
  - Quality settings (23-25 for different resolutions)

## Fields Displayed

### Active Job Section
- File name
- Source resolution
- Source codec with bit depth
- Frame rate
- Container format
- Audio/subtitle stream counts
- Original file size
- Estimated output size with predicted savings
- Elapsed time
- WebRip indicator

### Job Table
- All jobs with comprehensive metadata
- Sortable by creation time (newest first)
- Color-coded by status
- Full filename, codec, resolution
- Size information (original, new, estimated)
- Savings percentage
- Duration/elapsed time
- Failure/skip reasons

## Usage

Simply run the TUI:
```bash
av1top
```

The enhanced display automatically shows all available information for each job based on what data has been collected.

## Technical Details

### New Job Fields
```go
type Job struct {
    // ... existing fields ...
    SourceCodec   string  // e.g., "h264", "hevc"
    Resolution    string  // e.g., "1920x1080"
    BitDepth      int     // e.g., 8, 10
    FrameRate     string  // e.g., "23.976"
    Container     string  // e.g., "matroska,webm"
    VideoCodec    string  // Target codec (av1)
    AudioStreams  int     // Count of audio streams
    SubStreams    int     // Count of subtitle streams
    EstimatedSize int64   // Estimated output size in bytes
}
```

### Daemon Updates
The daemon now populates all metadata fields when:
- Discovering new files (during scan)
- Starting transcode jobs
- Completing transcode jobs

All metadata is persisted in job JSON files for historical tracking.

