package main

import (
	"fmt"
	"os"
)

// MergeSegments merges all downloaded segments into a single file
func MergeSegments(segments []SegmentData, outputPath string) error {
	fmt.Printf("Merging %d segments into %s...\n", len(segments), outputPath)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Write all segments sequentially
	totalBytes := 0
	for i, segment := range segments {
		var data []byte

		if segment.FilePath != "" {
			// Read from temp file
			data, err = os.ReadFile(segment.FilePath)
			if err != nil {
				return fmt.Errorf("failed to read segment %d from disk: %w", i, err)
			}
		} else if segment.Data != nil {
			// Use in-memory data
			data = segment.Data
		} else {
			return fmt.Errorf("segment %d has no data", i)
		}

		n, err := outFile.Write(data)
		if err != nil {
			return fmt.Errorf("failed to write segment %d: %w", i, err)
		}
		totalBytes += n
	}

	fmt.Printf("Successfully merged %d segments (%d bytes) into %s\n",
		len(segments), totalBytes, outputPath)

	return nil
}

// MergeSegmentsWithInit merges fMP4 segments with initialization segment
func MergeSegmentsWithInit(segments []SegmentData, downloader *Downloader, outputPath string) error {
	if downloader.playlist.InitSegment == "" {
		return fmt.Errorf("no initialization segment found for fMP4 format")
	}

	fmt.Printf("Merging fMP4: initialization segment + %d media segments into %s...\n", len(segments), outputPath)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Step 1: Write initialization segment first
	fmt.Println("Writing initialization segment...")
	initData, err := DownloadContentWithRetry(downloader.playlist.InitSegment, downloader.maxRetries)
	if err != nil {
		return fmt.Errorf("failed to download initialization segment: %w", err)
	}

	totalBytes, err := outFile.Write(initData)
	if err != nil {
		return fmt.Errorf("failed to write initialization segment: %w", err)
	}

	// Step 2: Write all media segments sequentially
	fmt.Println("Writing media segments...")
	for i, segment := range segments {
		var data []byte

		if segment.FilePath != "" {
			// Read from temp file
			data, err = os.ReadFile(segment.FilePath)
			if err != nil {
				return fmt.Errorf("failed to read segment %d from disk: %w", i, err)
			}
		} else if segment.Data != nil {
			// Use in-memory data
			data = segment.Data
		} else {
			return fmt.Errorf("segment %d has no data", i)
		}

		n, err := outFile.Write(data)
		if err != nil {
			return fmt.Errorf("failed to write segment %d: %w", i, err)
		}
		totalBytes += n
	}

	fmt.Printf("Successfully merged fMP4: init + %d segments (%d bytes) into %s\n",
		len(segments), totalBytes, outputPath)

	return nil
}
