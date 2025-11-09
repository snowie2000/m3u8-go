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

	// Step 2: Download video segments
	fmt.Println("Downloading video segments...")
	downloader := NewDownloader(*concurrent, playlist, *retries)
	videoSegments, err := downloader.DownloadSegments(playlist.Segments)
	if err != nil {
		fmt.Printf("Error downloading video segments: %v\n", err)
		downloader.CleanupTempFiles()
		os.Exit(1)
	}

	// Step 2.5: Download audio segments if separate audio track exists
	var audioSegments []SegmentData
	var audioDownloader *Downloader
	if playlist.HasAudio && len(playlist.AudioSegments) > 0 {
		fmt.Println()
		fmt.Println("Downloading audio segments...")
		audioDownloader = NewDownloader(*concurrent, playlist, *retries)
		audioSegments, err = audioDownloader.DownloadSegments(playlist.AudioSegments)
		if err != nil {
			fmt.Printf("Error downloading audio segments: %v\n", err)
			downloader.CleanupTempFiles()
			audioDownloader.CleanupTempFiles()
			os.Exit(1)
		}
	}

	fmt.Println()

	// Step 3: Merge segments into output file
	// Determine output format
	isMP4 := strings.HasSuffix(*output, ".mp4")
	finalOutput := *output
	var tempVideoFile string
	var tempAudioFile string

	// For fMP4 format, output directly to MP4 (no conversion needed)
	// For TS format with MP4 output, create temporary TS file first
	if playlist.IsFragmented {
		// fMP4 format - segments are already MP4
		if !isMP4 {
			// User wants .ts but we have fMP4 - convert extension
			fmt.Println("⚠️  Fragmented MP4 format detected - output will be .mp4")
			finalOutput = strings.TrimSuffix(*output, filepath.Ext(*output)) + ".mp4"
		}

		// Merge video
		tempVideoFile = strings.TrimSuffix(finalOutput, ".mp4") + "_video.mp4"
		err = MergeSegmentsWithInit(videoSegments, downloader, tempVideoFile)
		if err != nil {
			fmt.Printf("Error merging video segments: %v\n", err)
			downloader.CleanupTempFiles()
			if audioDownloader != nil {
				audioDownloader.CleanupTempFiles()
			}
			os.Exit(1)
		}

		// Merge audio if exists
		if playlist.HasAudio && len(audioSegments) > 0 {
			tempAudioFile = strings.TrimSuffix(finalOutput, ".mp4") + "_audio.mp4"
			// Create a modified playlist for audio init segment
			audioPlaylist := *playlist
			audioPlaylist.InitSegment = playlist.AudioInit
			audioDownloader.playlist = &audioPlaylist
			err = MergeSegmentsWithInit(audioSegments, audioDownloader, tempAudioFile)
			if err != nil {
				fmt.Printf("Error merging audio segments: %v\n", err)
				downloader.CleanupTempFiles()
				audioDownloader.CleanupTempFiles()
				os.Remove(tempVideoFile)
				os.Exit(1)
			}
		}
	} else {
		// Traditional TS format
		if isMP4 {
			// Create temporary TS file for conversion
			tempVideoFile = strings.TrimSuffix(*output, ".mp4") + "_temp.ts"
			fmt.Printf("Creating temporary TS file: %s\n", tempVideoFile)
			err = MergeSegments(videoSegments, tempVideoFile)
		} else {
			err = MergeSegments(videoSegments, *output)
		}
		if err != nil {
			fmt.Printf("Error merging segments: %v\n", err)
			downloader.CleanupTempFiles()
			if audioDownloader != nil {
				audioDownloader.CleanupTempFiles()
			}
			os.Exit(1)
		}
	}

	// Clean up temporary segment files after successful merge
	downloader.CleanupTempFiles()
	if audioDownloader != nil {
		audioDownloader.CleanupTempFiles()
	}

	// Step 4: Convert/Merge to final output
	if playlist.IsFragmented {
		// fMP4: Merge video and audio using ffmpeg if audio exists
		if playlist.HasAudio && tempAudioFile != "" {
			fmt.Printf("\nMerging video and audio using ffmpeg...\n")
			err = mergeVideoAudio(tempVideoFile, tempAudioFile, finalOutput)
			if err != nil {
				fmt.Printf("Error merging video and audio: %v\n", err)
				fmt.Printf("Temporary files kept:\n  Video: %s\n  Audio: %s\n", tempVideoFile, tempAudioFile)
				os.Exit(1)
			}
			// Remove temporary files
			os.Remove(tempVideoFile)
			os.Remove(tempAudioFile)
			fmt.Printf("Temporary video and audio files removed\n")
		} else {
			// No separate audio, rename video file to final output
			if tempVideoFile != finalOutput {
				os.Rename(tempVideoFile, finalOutput)
			}
		}
	} else if isMP4 {
		// TS to MP4 conversion
		fmt.Printf("\nConverting TS to MP4 using ffmpeg...\n")
		err = convertToMP4(tempVideoFile, finalOutput)
		if err != nil {
			fmt.Printf("Error converting to MP4: %v\n", err)
			fmt.Printf("Temporary TS file kept at: %s\n", tempVideoFile)
			os.Exit(1)
		}

		// Remove temporary TS file
		os.Remove(tempVideoFile)
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

// mergeVideoAudio uses ffmpeg to merge separate video and audio files
func mergeVideoAudio(videoFile, audioFile, outputFile string) error {
	// Ensure ffmpeg is available (download if necessary)
	ffmpegPath, err := ensureFFmpeg()
	if err != nil {
		return err
	}

	// Merge video and audio using ffmpeg
	// -i: input files (video and audio)
	// -c copy: copy streams without re-encoding (fast)
	// -y: overwrite output file
	cmd := exec.Command(ffmpegPath, "-i", videoFile, "-i", audioFile, "-c", "copy", "-y", outputFile)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg merge failed: %w\nOutput: %s", err, string(output))
	}

	fmt.Println("✓ Video and audio merged successfully")
	return nil
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
