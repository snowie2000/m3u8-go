# M3U8 Video Downloader

A simple and efficient Go application to download M3U8 streaming videos from a given URL.

## Features

- ✅ Parse M3U8 playlists (both master and media playlists)
- ✅ Support for local M3U8 files with base URL resolution
- ✅ AES-128 encryption support (automatic decryption)
- ✅ Custom encryption key support (for protected keys)
- ✅ Custom HTTP headers (User-Agent, Referer, etc.)
- ✅ **Smart memory management** (auto-switches to disk for large downloads)
- ✅ Concurrent segment downloads for faster performance
- ✅ Automatic retry with exponential backoff for failed downloads
- ✅ Configurable timeout for slow connections
- ✅ Progress tracking during download
- ✅ Automatic URL resolution for relative paths
- ✅ Merge segments into a single video file
- ✅ MP4 conversion with automatic ffmpeg download (Windows)
- ✅ Simple command-line interface

## Installation

### Prerequisites

- Go 1.21 or higher

### Build from source

```bash
# Clone or navigate to the repository
cd f:\Git\m3u8-go

# Build the application
go build -o m3u8-downloader.exe

# Or run directly
go run .
```

## Usage

### Basic Usage

```bash
# Download a video from M3U8 URL
m3u8-downloader.exe -url "https://example.com/playlist.m3u8"

# Or using go run
go run . -url "https://example.com/playlist.m3u8"
```

### Advanced Options

```bash
# Specify output filename
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" -output "my_video.ts"

# Adjust concurrent downloads (default: 10)
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" -concurrent 20

# Download from local M3U8 file with base URL
m3u8-downloader.exe -url "playlist.m3u8" -baseurl "https://example.com/videos/"

# Adjust timeout and retries for slow/unstable connections
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" -timeout 60 -retries 5

# Complete example with all options
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" -output "video.ts" -concurrent 15 -timeout 45 -retries 3
```

### Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-url` | M3U8 playlist URL or local file path (required) | - |
| `-baseurl` | Base URL for resolving relative URLs (optional, only needed for local files with relative URLs) | - |
| `-output` | Output file name | `video.ts` |
| `-concurrent` | Maximum concurrent downloads | `10` |
| `-retries` | Maximum retry attempts for failed downloads | `3` |
| `-timeout` | Timeout in seconds for HTTP requests | `30` |
| `-key` | Path to custom encryption key file (overrides key URL in M3U8) | - |
| `-header` | Custom HTTP header in format `Key:Value` (can be specified multiple times) | - |

## How It Works

1. **Parse Playlist**: Downloads and parses the M3U8 playlist file (or reads from local file)
   - Handles both master playlists (with multiple quality streams) and media playlists
   - Automatically selects the first variant from master playlists
   - Resolves relative URLs to absolute URLs using base URL
   - Detects and extracts encryption keys from #EXT-X-KEY tags

2. **Download Segments**: Downloads all video segments concurrently
   - Uses goroutines for parallel downloads
   - Limits concurrent connections to avoid overwhelming the server
   - **Smart memory management**: 
     - Small downloads (< 50MB): Stores segments in memory for speed
     - Large downloads (≥ 50MB): Automatically switches to temporary disk storage to prevent memory issues
   - Automatically retries failed downloads with exponential backoff
   - Configurable timeout to handle slow connections
   - Automatically decrypts AES-128 encrypted segments
   - Shows real-time progress with download size

3. **Merge Segments**: Combines all segments into a single file
   - Handles both memory-stored and disk-stored segments transparently
   - Maintains the correct segment order
   - Preserves TS packet structure after decryption
   - Outputs a transport stream (.ts) file
   - Automatically cleans up temporary files

## Output Format

The application supports both `.ts` (MPEG Transport Stream) and `.mp4` output formats.

### TS Output (default)
```bash
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" -output "video.ts"
```

### MP4 Output (automatic conversion)
```bash
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" -output "video.mp4"
```

**Automatic ffmpeg Download (Windows only):**
- If ffmpeg is not found, the application will offer to download it automatically
- Downloaded ffmpeg is placed in a local `ffmpeg/` directory
- No system-wide installation required
- For macOS/Linux, you'll need to install ffmpeg manually

The application will:
1. Check if ffmpeg is available in PATH or local `ffmpeg/` directory
2. If not found, prompt you to download it automatically (Windows only)
3. Download and merge segments into a temporary TS file
4. Convert the TS file to MP4 using `ffmpeg -c copy` (fast, no re-encoding)
5. Remove the temporary TS file

Both formats can be played by most video players including:
- VLC Media Player
- MPV
- FFmpeg
- Windows Media Player (with appropriate codecs)

### Installing FFmpeg

If you don't have ffmpeg installed:
- **Windows**: Download from [ffmpeg.org](https://ffmpeg.org/download.html) and add to PATH
- **macOS**: `brew install ffmpeg`
- **Linux**: `sudo apt install ffmpeg` or `sudo yum install ffmpeg`

## Examples

```bash
# Example 1: Basic download from URL
go run . -url "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8"

# Example 2: Download from local M3U8 file
go run . -url "playlist.m3u8" -baseurl "https://example.com/videos/"

# Example 3: With custom output
go run . -url "https://example.com/stream.m3u8" -output "my_favorite_video.ts"

# Example 4: High concurrency for faster downloads
go run . -url "https://example.com/stream.m3u8" -concurrent 30

# Example 5: Local file with encryption
go run . -url "encrypted-playlist.m3u8" -baseurl "https://cdn.example.com/" -output "decrypted_video.ts"

# Example 6: Slow connection with increased timeout and retries
go run . -url "https://slow-server.com/stream.m3u8" -timeout 60 -retries 5

# Example 7: Download and convert to MP4 automatically
go run . -url "https://example.com/playlist.m3u8" -output "video.mp4"

# Example 8: Use custom encryption key (when key URL is protected)
go run . -url "https://example.com/playlist.m3u8" -key "my_key.key" -output "video.ts"

# Example 9: Add custom headers (for protected content)
go run . -url "https://example.com/playlist.m3u8" -header "User-Agent:Mozilla/5.0" -header "Referer:https://example.com"

# Example 10: Combined - custom key and headers
go run . -url "https://example.com/playlist.m3u8" -key "decryption.key" -header "User-Agent:Chrome/120.0" -header "Origin:https://example.com" -output "video.mp4"
```

## Project Structure

```
m3u8-go/
├── main.go         # Entry point and CLI handling
├── parser.go       # M3U8 playlist parsing logic
├── downloader.go   # Concurrent segment downloading
├── decryptor.go    # AES-128 decryption functionality
├── merger.go       # Segment merging functionality
├── go.mod          # Go module definition
└── README.md       # This file
```

## Troubleshooting

### Issue: "Failed to download encryption key"
- The encryption key URL might be protected or require special headers
- Use `-key` flag to provide a custom encryption key file that you've downloaded manually
- Example: `-key "my_key.key"`

### Issue: "403 Forbidden" or authentication errors
- The server might require specific headers (User-Agent, Referer, Origin, etc.)
- Use `-header` flag to specify custom HTTP headers (can be used multiple times)
- Example: `-header "User-Agent:Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" -header "Referer:https://example.com"`
- Common headers to try:
  - `User-Agent`: Pretend to be a web browser
  - `Referer`: The website URL where the video is embedded
  - `Origin`: The domain of the website

### Issue: "No segments found in playlist"
- Make sure the URL points to a valid M3U8 file
- Check if the playlist requires authentication
- For local files, ensure the base URL is correct

### Issue: "Base URL is required when using a local M3U8 file"
- This error appears only when the local M3U8 file contains relative URLs (e.g., `segment1.ts`)
- Provide the base URL using `-baseurl` flag: `-baseurl "https://example.com/videos/"`
- If your local M3U8 file already contains absolute URLs (e.g., `https://...`), you don't need `-baseurl`

### Issue: Download is slow
- Increase the concurrent downloads: `-concurrent 20` or higher
- Check your internet connection speed

### Issue: Segments timing out or failing
- Increase timeout: `-timeout 60` (60 seconds)
- Increase retry attempts: `-retries 5`
- Reduce concurrency to avoid overwhelming the server: `-concurrent 5`

### Issue: Some segments fail to download
- The server might be rate-limiting requests
- Try reducing concurrent downloads: `-concurrent 5`
- Some segments might be temporarily unavailable

## Notes

- **Memory Management**: The application intelligently manages memory:
  - Downloads under 50MB are stored in memory for maximum speed
  - Downloads 50MB and larger automatically switch to disk storage to prevent memory issues
  - Temporary files are automatically cleaned up after merging
- Some M3U8 streams may be protected by DRM or require authentication, which this tool does not currently support
- Always respect copyright and terms of service when downloading videos

## License

This is a demonstration project. Use responsibly and in accordance with applicable laws and terms of service.

## Contributing

Feel free to submit issues or pull requests for improvements!
