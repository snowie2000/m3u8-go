package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

const (
	// MemoryThresholdMB is the threshold in MB for switching to disk storage
	// If total downloaded size exceeds this, segments will be saved to temp files
	MemoryThresholdMB = 50
	MemoryThreshold   = MemoryThresholdMB * 1024 * 1024
)

// Downloader manages concurrent downloads of video segments
type Downloader struct {
	maxConcurrent  int
	progress       int32
	total          int
	playlist       *M3U8Playlist
	maxRetries     int
	totalSize      int64
	useDiskStorage bool
	tempDir        string
	mu             sync.Mutex
}

// NewDownloader creates a new downloader with specified concurrency
func NewDownloader(maxConcurrent int, playlist *M3U8Playlist, maxRetries int) *Downloader {
	return &Downloader{
		maxConcurrent:  maxConcurrent,
		progress:       0,
		playlist:       playlist,
		maxRetries:     maxRetries,
		useDiskStorage: false,
		totalSize:      0,
	}
}

// SegmentData holds a downloaded segment with its index
type SegmentData struct {
	Index    int
	Data     []byte // Used when storing in memory
	FilePath string // Used when storing on disk
	Error    error
}

// shouldUseDisk checks if we should switch to disk storage
func (d *Downloader) shouldUseDisk() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.useDiskStorage
}

// checkAndSwitchToDisk checks if we need to switch to disk storage
func (d *Downloader) checkAndSwitchToDisk(newDataSize int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Always update total size regardless of storage mode
	d.totalSize += int64(newDataSize)

	// Check if we need to switch to disk storage
	if !d.useDiskStorage && d.totalSize > MemoryThreshold {
		// Create temp directory for segments
		tempDir, err := os.MkdirTemp("", "m3u8-segments-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		d.tempDir = tempDir
		d.useDiskStorage = true
		fmt.Printf("\n⚠️  Download size exceeded %dMB, switching to disk storage (%s)\n", MemoryThresholdMB, tempDir)
	}

	return nil
}

// DownloadSegments downloads all segments concurrently
func (d *Downloader) DownloadSegments(segments []string) ([]SegmentData, error) {
	d.total = len(segments)

	// For fMP4, just note that we'll handle init segment during merge
	if d.playlist.IsFragmented && d.playlist.InitSegment != "" {
		fmt.Printf("ℹ️  Fragmented MP4 format detected\n")
		fmt.Printf("   Initialization segment: %s\n", d.playlist.InitSegment)
		fmt.Printf("   Media segments: %d\n", len(segments))
	}

	results := make([]SegmentData, len(segments))

	// Create a semaphore to limit concurrent downloads
	semaphore := make(chan struct{}, d.maxConcurrent)
	var wg sync.WaitGroup
	resultChan := make(chan SegmentData, len(segments))

	// Start downloading segments
	for i, segmentURL := range segments {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Download the segment with retry
			data, err := DownloadContentWithRetry(url, d.maxRetries)

			if err != nil {
				resultChan <- SegmentData{
					Index: index,
					Error: err,
				}
				atomic.AddInt32(&d.progress, 1)
				return
			}

			// Decrypt if necessary
			if d.playlist.Encrypted {
				data, err = DecryptSegment(data, d.playlist.Key, d.playlist.KeyIV, index)
				if err != nil {
					resultChan <- SegmentData{
						Index: index,
						Error: fmt.Errorf("decryption failed: %w", err),
					}
					atomic.AddInt32(&d.progress, 1)
					return
				}
			}

			// Check if we should switch to disk storage
			err = d.checkAndSwitchToDisk(len(data))
			if err != nil {
				resultChan <- SegmentData{
					Index: index,
					Error: fmt.Errorf("storage check failed: %w", err),
				}
				atomic.AddInt32(&d.progress, 1)
				return
			}

			var segmentData SegmentData
			segmentData.Index = index

			// Store based on storage mode
			if d.shouldUseDisk() {
				// Save to temp file
				tempFile := filepath.Join(d.tempDir, fmt.Sprintf("segment_%06d.ts", index))
				err = os.WriteFile(tempFile, data, 0644)
				if err != nil {
					segmentData.Error = fmt.Errorf("failed to write temp file: %w", err)
				} else {
					segmentData.FilePath = tempFile
				}
			} else {
				// Store in memory
				segmentData.Data = data
			}

			// Update progress
			current := atomic.AddInt32(&d.progress, 1)
			if segmentData.Error == nil {
				fmt.Printf("\rDownloading segments: %d/%d (%.1f%%) [%s]",
					current, d.total, float64(current)/float64(d.total)*100,
					formatBytes(d.totalSize))
			} else {
				fmt.Printf("\rDownloading segments: %d/%d (%.1f%%) - Error on segment %d",
					current, d.total, float64(current)/float64(d.total)*100, index)
			}

			resultChan <- segmentData
		}(i, segmentURL)
	}

	// Wait for all downloads to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var errors []error
	for result := range resultChan {
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("segment %d: %w", result.Index, result.Error))
		} else {
			results[result.Index] = result
		}
	}

	fmt.Println() // New line after progress

	if len(errors) > 0 {
		// Clean up temp directory on error
		if d.tempDir != "" {
			os.RemoveAll(d.tempDir)
		}
		return nil, fmt.Errorf("failed to download %d segments: %v", len(errors), errors[0])
	}

	if d.useDiskStorage {
		fmt.Printf("✓ Segments stored in temporary directory: %s\n", d.tempDir)
	} else {
		fmt.Printf("✓ Segments stored in memory (%s)\n", formatBytes(d.totalSize))
	}

	return results, nil
}

// CleanupTempFiles removes temporary files if they were used
func (d *Downloader) CleanupTempFiles() {
	if d.tempDir != "" {
		os.RemoveAll(d.tempDir)
		fmt.Printf("✓ Temporary files cleaned up\n")
	}
}

// formatBytes formats bytes into human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
