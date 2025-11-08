package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ffmpegDir = "ffmpeg"
)

// getFFmpegPath returns the path to ffmpeg executable
func getFFmpegPath() string {
	exeName := "ffmpeg"
	if runtime.GOOS == "windows" {
		exeName = "ffmpeg.exe"
	}

	// Check local ffmpeg directory first
	localPath := filepath.Join(ffmpegDir, exeName)
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	// Fall back to system PATH
	return exeName
}

// checkFFmpeg checks if ffmpeg is available
func checkFFmpeg() bool {
	ffmpegPath := getFFmpegPath()
	cmd := exec.Command(ffmpegPath, "-version")
	return cmd.Run() == nil
}

// ensureFFmpeg ensures ffmpeg is available, downloading if necessary
func ensureFFmpeg() (string, error) {
	ffmpegPath := getFFmpegPath()

	// Check if ffmpeg is already available
	cmd := exec.Command(ffmpegPath, "-version")
	if cmd.Run() == nil {
		return ffmpegPath, nil
	}

	// ffmpeg not found, ask user to download
	fmt.Println("\nffmpeg is not found in your system.")
	fmt.Print("Would you like to download it automatically? (y/n): ")

	var response string
	fmt.Scanln(&response)

	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		return "", fmt.Errorf("ffmpeg is required for MP4 conversion. Please install it manually or use .ts output")
	}

	// Download and install ffmpeg
	fmt.Println("\nDownloading ffmpeg...")
	if err := downloadFFmpeg(); err != nil {
		return "", fmt.Errorf("failed to download ffmpeg: %w", err)
	}

	return getFFmpegPath(), nil
}

// downloadFFmpeg downloads and extracts ffmpeg for the current platform
func downloadFFmpeg() error {
	var downloadURL string
	var isZip bool

	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "amd64" {
			downloadURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
			isZip = true
		} else {
			return fmt.Errorf("unsupported Windows architecture: %s", runtime.GOARCH)
		}
	case "darwin":
		return fmt.Errorf("automatic download not supported on macOS. Please install via: brew install ffmpeg")
	case "linux":
		return fmt.Errorf("automatic download not supported on Linux. Please install via: sudo apt install ffmpeg")
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Create ffmpeg directory
	if err := os.MkdirAll(ffmpegDir, 0755); err != nil {
		return fmt.Errorf("failed to create ffmpeg directory: %w", err)
	}

	// Download the file
	fmt.Printf("Downloading from: %s\n", downloadURL)
	resp, err := httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Save to temporary file
	tmpFile := filepath.Join(ffmpegDir, "ffmpeg_download.zip")
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Download with progress
	fmt.Println("Downloading ffmpeg (this may take a few minutes)...")
	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		return fmt.Errorf("failed to save download: %w", err)
	}

	// Extract the archive
	if isZip {
		fmt.Println("Extracting ffmpeg...")
		if err := extractFFmpegFromZip(tmpFile); err != nil {
			return fmt.Errorf("failed to extract: %w", err)
		}
	}

	// Remove temporary file
	os.Remove(tmpFile)

	fmt.Println("ffmpeg downloaded and installed successfully!")
	return nil
}

// extractFFmpegFromZip extracts ffmpeg.exe from the downloaded zip
func extractFFmpegFromZip(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	exeName := "ffmpeg.exe"

	// Find and extract ffmpeg.exe
	for _, f := range r.File {
		// Look for ffmpeg.exe in bin directory
		if strings.HasSuffix(f.Name, "bin/"+exeName) || strings.HasSuffix(f.Name, exeName) {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			// Create output file
			outPath := filepath.Join(ffmpegDir, exeName)
			outFile, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, rc)
			if err != nil {
				return err
			}

			fmt.Printf("Extracted: %s\n", outPath)
			return nil
		}
	}

	return fmt.Errorf("ffmpeg.exe not found in archive")
}
