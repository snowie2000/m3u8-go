package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Downloader manages concurrent downloads of video segments
type Downloader struct {
	maxConcurrent int
	progress      int32
	total         int
	playlist      *M3U8Playlist
	maxRetries    int
}

// NewDownloader creates a new downloader with specified concurrency
func NewDownloader(maxConcurrent int, playlist *M3U8Playlist, maxRetries int) *Downloader {
	return &Downloader{
		maxConcurrent: maxConcurrent,
		progress:      0,
		playlist:      playlist,
		maxRetries:    maxRetries,
	}
}

// SegmentData holds a downloaded segment with its index
type SegmentData struct {
	Index int
	Data  []byte
	Error error
}

// DownloadSegments downloads all segments concurrently
func (d *Downloader) DownloadSegments(segments []string) ([][]byte, error) {
	d.total = len(segments)
	results := make([][]byte, len(segments))

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

			// Decrypt if necessary
			if err == nil && d.playlist.Encrypted {
				data, err = DecryptSegment(data, d.playlist.Key, d.playlist.KeyIV, index)
				if err != nil {
					err = fmt.Errorf("decryption failed: %w", err)
				}
			}

			// Update progress
			current := atomic.AddInt32(&d.progress, 1)
			if err == nil {
				fmt.Printf("\rDownloading segments: %d/%d (%.1f%%)",
					current, d.total, float64(current)/float64(d.total)*100)
			} else {
				fmt.Printf("\rDownloading segments: %d/%d (%.1f%%) - Error on segment %d",
					current, d.total, float64(current)/float64(d.total)*100, index)
			}

			resultChan <- SegmentData{
				Index: index,
				Data:  data,
				Error: err,
			}
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
			results[result.Index] = result.Data
		}
	}

	fmt.Println() // New line after progress

	if len(errors) > 0 {
		return nil, fmt.Errorf("failed to download %d segments: %v", len(errors), errors[0])
	}

	return results, nil
}
