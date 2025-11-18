# AV1 Transcoding Daemon

A self-contained Go-based AV1 transcoding daemon with a Bubble Tea TUI, designed to replace Tdarr-based workflows with Intel QSV AV1 encoding.

## Features

- **Automatic FFmpeg Management**: Downloads and verifies FFmpeg 8.x automatically
- **Intel QSV AV1 Encoding**: Hardware-accelerated AV1 encoding using Intel Arc GPUs
- **Smart File Detection**: WebRip heuristics, AV1 detection, size thresholds
- **Job Management**: Persistent job state with JSON storage
- **Size Gate**: Rejects transcodes that don't meet size reduction thresholds
- **Atomic File Operations**: Safe file replacement with verification
- **Bubble Tea TUI**: Real-time monitoring with system metrics and job status

## Requirements

- Debian/Ubuntu Linux
- Intel GPU with QSV support (Intel Arc A310 or similar)
- Root/sudo access for installation
- Go 1.25+ (installed automatically by install script)

## Installation

### Quick Install

```bash
sudo ./install.sh
```

The install script will:
1. Install Go 1.25 if needed
2. Build the daemon and TUI binaries
3. Create system user and directories
4. Set up systemd service
5. Create default configuration

### Manual Installation

1. Install Go 1.25+
2. Build the project:
   ```bash
   go mod download
   go build -o av1d ./cmd/av1d
   go build -o av1top ./cmd/av1top
   ```

3. Copy binaries to your PATH
4. Create directories:
   ```bash
   mkdir -p ~/.local/share/av1qsvd/{ffmpeg,jobs}
   ```

## Configuration

Edit `/etc/av1qsvd/config.json` (or use default config):

```json
{
  "ffmpeg_url": "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n8.0-latest-linux64-gpl-8.0.tar.xz",
  "ffmpeg_install_dir": "/var/lib/av1qsvd/ffmpeg",
  "library_roots": ["/path/to/media/library"],
  "min_bytes": 2147483648,
  "max_size_ratio": 0.90,
  "job_state_dir": "/var/lib/av1qsvd/jobs",
  "scan_interval_sec": 60
}
```

### Configuration Options

- `library_roots`: Array of directories to scan for media files
- `min_bytes`: Minimum file size to process (default: 2 GiB)
- `max_size_ratio`: Maximum size ratio for acceptance (default: 0.90 = 90%)
- `scan_interval_sec`: How often to scan for new files (default: 60 seconds)

## Usage

### Daemon (av1d)

Start the daemon:
```bash
sudo systemctl start av1d
```

Enable auto-start:
```bash
sudo systemctl enable av1d
```

Check status:
```bash
sudo systemctl status av1d
```

View logs:
```bash
sudo journalctl -u av1d -f
```

### TUI (av1top)

Run the TUI to monitor jobs:
```bash
av1top
```

Controls:
- `q` or `Ctrl+C`: Quit
- `r`: Manual refresh

## How It Works

1. **Scanning**: The daemon periodically scans library roots for media files (`.mkv`, `.mp4`, `.m4v`)

2. **Filtering**: Files are filtered based on:
   - `.av1skip` marker files (permanent skip)
   - File size threshold (< 2GB)
   - Already AV1 encoded
   - Not a video file

3. **Metadata Analysis**: FFprobe extracts metadata and detects WebRip characteristics

4. **Job Creation**: Valid files become pending jobs

5. **Transcoding**: Jobs are processed one at a time:
   - File stability check (prevents transcoding during copy)
   - AV1 QSV encoding with quality based on resolution
   - Russian audio/subtitle removal
   - Size gate validation
   - Atomic file replacement

6. **Sidecar Files**: 
   - `.why.txt`: Explains why files were skipped/rejected
   - `.av1skip`: Marks files to permanently skip

## File Structure

```
/usr/local/bin/
  ├── av1d          # Daemon binary
  └── av1top        # TUI binary

/var/lib/av1qsvd/
  ├── ffmpeg/       # FFmpeg binary storage
  └── jobs/         # Job JSON files

/etc/av1qsvd/
  └── config.json   # Configuration file
```

## Troubleshooting

### FFmpeg Verification Fails

Ensure Intel GPU drivers and QSV are properly configured:
```bash
# Check QSV availability
vainfo
```

### Jobs Not Processing

Check:
1. Library roots are configured correctly
2. Files meet size threshold (> 2GB)
3. Files aren't already AV1 encoded
4. No `.av1skip` markers exist
5. Check daemon logs: `sudo journalctl -u av1d -f`

### Permission Issues

Ensure the `av1d` user has read access to library directories:
```bash
# Add read access to media directories
sudo setfacl -R -m u:av1d:rx /path/to/media
```

## Rebuilding

### Quick Rebuild

Use the rebuild script:
```bash
./rebuild.sh
```

This builds both binaries in the current directory.

### Rebuild and Install

To rebuild and install system-wide:
```bash
sudo ./rebuild.sh --install
```

This will:
1. Rebuild both binaries
2. Copy them to `/usr/local/bin`
3. Restart the service (you'll need to do this manually)

### Manual Rebuild

Build binaries manually:
```bash
# Update dependencies
go mod download
go mod tidy

# Build daemon
go build -o av1d ./cmd/av1d

# Build TUI
go build -o av1top ./cmd/av1top
```

After rebuilding, if installed system-wide:
```bash
sudo cp av1d av1top /usr/local/bin/
sudo systemctl restart av1d
```

## Development

Run daemon manually (for testing):
```bash
./av1d
```

Run TUI manually (for testing):
```bash
./av1top
```

## License

[Add your license here]

## Contributing

[Add contribution guidelines here]

