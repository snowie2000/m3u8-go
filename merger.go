package main

import (
	"fmt"
	"os"
)

// MergeSegments merges all downloaded segments into a single file
func MergeSegments(segments [][]byte, outputPath string) error {
	fmt.Printf("Merging %d segments into %s...\n", len(segments), outputPath)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Write all segments sequentially
	// After decryption, segments are already properly formatted TS data
	totalBytes := 0
	for i, segment := range segments {
		n, err := outFile.Write(segment)
		if err != nil {
			return fmt.Errorf("failed to write segment %d: %w", i, err)
		}
		totalBytes += n
	}

	fmt.Printf("Successfully merged %d segments (%d bytes) into %s\n",
		len(segments), totalBytes, outputPath)

	return nil
}
