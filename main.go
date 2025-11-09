package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// headerFlags is a custom flag type to support multiple -header flags
type headerFlags []string

func (h *headerFlags) String() string {
	return strings.Join(*h, ", ")
}

func (h *headerFlags) Set(value string) error {
	*h = append(*h, value)
	return nil
}

func main() {
	// Define command-line flags
	url := flag.String("url", "", "M3U8 playlist URL or local file path to download")
	baseURL := flag.String("baseurl", "", "Base URL for resolving relative URLs (optional for local files with absolute URLs)")
	output := flag.String("output", "video.ts", "Output file name")
	concurrent := flag.Int("concurrent", 10, "Maximum concurrent downloads")
	retries := flag.Int("retries", 3, "Maximum retry attempts for failed downloads")
	timeout := flag.Int("timeout", 30, "Timeout in seconds for HTTP requests")
	keyFile := flag.String("key", "", "Path to custom encryption key file (overrides key URL in M3U8)")

	var headers headerFlags
	flag.Var(&headers, "header", "Custom HTTP header in format 'Key:Value' (can be used multiple times)")

	flag.Parse()

	// Set timeout for HTTP client
	httpClient.Timeout = time.Duration(*timeout) * time.Second

	// Parse and set custom headers
	if len(headers) > 0 {
		customHeaders := parseHeaders(headers)
		SetCustomHeaders(customHeaders)
		fmt.Printf("Custom headers set: %d header(s)\n", len(customHeaders))
	}

	// Validate inputs
	if *url == "" {
		fmt.Println("Error: M3U8 URL or file path is required")
		fmt.Println("\nUsage:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  # Download from URL")
		fmt.Println("  m3u8-downloader.exe -url \"https://example.com/playlist.m3u8\"")
		fmt.Println("\n  # Download from local file")
		fmt.Println("  m3u8-downloader.exe -url \"playlist.m3u8\" -baseurl \"https://example.com/\"")
		os.Exit(1)
	}

	// Check if input is a local file or URL
	isLocalFile := !strings.HasPrefix(*url, "http://") && !strings.HasPrefix(*url, "https://")

	// Ensure output has correct extension
	if !strings.HasSuffix(*output, ".ts") && !strings.HasSuffix(*output, ".mp4") {
		*output = *output + ".ts"
	}

	fmt.Printf("M3U8 Downloader\n")
	fmt.Printf("================\n")
	if isLocalFile {
		fmt.Printf("Local File: %s\n", *url)
		fmt.Printf("Base URL: %s\n", *baseURL)
	} else {
		fmt.Printf("URL: %s\n", *url)
	}
	fmt.Printf("Output: %s\n", *output)
	fmt.Printf("Max Concurrent Downloads: %d\n", *concurrent)
	fmt.Printf("Timeout: %d seconds\n", *timeout)
	fmt.Printf("Max Retries: %d\n\n", *retries)

	// Step 1: Parse the M3U8 playlist
	fmt.Println("Parsing M3U8 playlist...")
	var playlist *M3U8Playlist
	var err error

	// Load custom encryption key if provided
	var customKey []byte
	if *keyFile != "" {
		customKey, err = os.ReadFile(*keyFile)
		if err != nil {
			fmt.Printf("Error reading custom key file: %v\n", err)
			os.Exit(1)
		}
		if len(customKey) != 16 {
			fmt.Printf("Error: Invalid key length. Expected 16 bytes, got %d\n", len(customKey))
			os.Exit(1)
		}
		fmt.Printf("✓ Custom encryption key loaded from: %s\n", *keyFile)
	}

	// Parse M3U8 with custom key (if provided)
	if isLocalFile {
		playlist, err = ParseM3U8FromFileWithKey(*url, *baseURL, customKey)
	} else {
		playlist, err = ParseM3U8WithKey(*url, customKey)
	}

	if err != nil {
		fmt.Printf("Error parsing playlist: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d segments to download\n", len(playlist.Segments))
	if playlist.Encrypted {
		fmt.Println("⚠️  Encrypted stream detected - will decrypt segments")
	}
	fmt.Println()

	// Step 2: Download all segments
	downloader := NewDownloader(*concurrent, playlist, *retries)
	segments, err := downloader.DownloadSegments(playlist.Segments)
	if err != nil {
		fmt.Printf("Error downloading segments: %v\n", err)
		downloader.CleanupTempFiles()
		os.Exit(1)
	}

	fmt.Println()

	// Step 3: Merge segments into output file
	// If output is MP4, create a temporary TS file first
	isMP4 := strings.HasSuffix(*output, ".mp4")
	finalOutput := *output
	var tsFile string

	if isMP4 {
		// Create temporary TS file
		tsFile = strings.TrimSuffix(*output, ".mp4") + "_temp.ts"
		fmt.Printf("Creating temporary TS file: %s\n", tsFile)
		err = MergeSegments(segments, tsFile)
	} else {
		err = MergeSegments(segments, *output)
	}

	if err != nil {
		fmt.Printf("Error merging segments: %v\n", err)
		downloader.CleanupTempFiles()
		os.Exit(1)
	}

	// Clean up temporary segment files after successful merge
	downloader.CleanupTempFiles()

	// Step 4: Convert to MP4 if needed
	if isMP4 {
		fmt.Printf("\nConverting TS to MP4 using ffmpeg...\n")
		err = convertToMP4(tsFile, finalOutput)
		if err != nil {
			fmt.Printf("Error converting to MP4: %v\n", err)
			fmt.Printf("Temporary TS file kept at: %s\n", tsFile)
			os.Exit(1)
		}

		// Remove temporary TS file
		os.Remove(tsFile)
		fmt.Printf("Temporary TS file removed\n")
	}

	// Get file size
	fileInfo, err := os.Stat(finalOutput)
	if err == nil {
		fmt.Printf("Output file size: %.2f MB\n", float64(fileInfo.Size())/(1024*1024))
	}

	absPath, _ := filepath.Abs(finalOutput)
	fmt.Printf("\nDownload complete! File saved to:\n%s\n", absPath)
}

// convertToMP4 uses ffmpeg to convert TS to MP4
func convertToMP4(tsFile, mp4File string) error {
	// Ensure ffmpeg is available (download if necessary)
	ffmpegPath, err := ensureFFmpeg()
	if err != nil {
		return err
	}

	// Convert TS to MP4 using ffmpeg
	// -i: input file
	// -c copy: copy streams without re-encoding (fast)
	// -y: overwrite output file
	cmd := exec.Command(ffmpegPath, "-i", tsFile, "-c", "copy", "-y", mp4File)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// parseHeaders parses multiple header strings in format "Key:Value"
func parseHeaders(headerSlice []string) map[string]string {
	headers := make(map[string]string)
	for _, headerStr := range headerSlice {
		parts := strings.SplitN(headerStr, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers[key] = value
		}
	}
	return headers
}
