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
