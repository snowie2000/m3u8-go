package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	// HTTP client with timeout
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Custom headers to include in all HTTP requests
	customHeaders map[string]string
)

// SetCustomHeaders sets custom headers to be included in all HTTP requests
func SetCustomHeaders(headers map[string]string) {
	customHeaders = headers
}

// M3U8Playlist represents the parsed M3U8 playlist
type M3U8Playlist struct {
	BaseURL   string
	Segments  []string
	IsStream  bool
	Encrypted bool
	KeyURL    string
	KeyIV     string
	Key       []byte
	CustomKey []byte // Custom key provided by user (skips download)
}

// ParseM3U8 downloads and parses the M3U8 playlist from the given URL
func ParseM3U8(playlistURL string) (*M3U8Playlist, error) {
	return ParseM3U8WithKey(playlistURL, nil)
}

// ParseM3U8WithKey downloads and parses the M3U8 playlist with optional custom key
func ParseM3U8WithKey(playlistURL string, customKey []byte) (*M3U8Playlist, error) {
	// Download the playlist
	req, err := http.NewRequest("GET", playlistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add custom headers
	for key, value := range customHeaders {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download playlist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download playlist: status code %d", resp.StatusCode)
	}

	// Parse the base URL for resolving relative URLs
	baseURL, err := url.Parse(playlistURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playlist URL: %w", err)
	}

	return parseM3U8Content(resp.Body, baseURL, customKey)
}

// ParseM3U8FromFile parses a local M3U8 file with a provided base URL
func ParseM3U8FromFile(filePath string, baseURLStr string) (*M3U8Playlist, error) {
	return ParseM3U8FromFileWithKey(filePath, baseURLStr, nil)
}

// ParseM3U8FromFileWithKey parses a local M3U8 file with optional custom key
func ParseM3U8FromFileWithKey(filePath string, baseURLStr string, customKey []byte) (*M3U8Playlist, error) {
	// Open the local file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	// Parse the base URL for resolving relative URLs (if provided)
	var baseURL *url.URL
	if baseURLStr != "" {
		baseURL, err = url.Parse(baseURLStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse base URL: %w", err)
		}
	} else {
		// Use a dummy base URL that won't resolve anything
		// This allows absolute URLs to work, but relative URLs will stay as-is
		baseURL, _ = url.Parse("file://local")
	}

	return parseM3U8Content(file, baseURL, customKey)
}

// parseM3U8Content parses M3U8 content from an io.Reader
func parseM3U8Content(reader io.Reader, baseURL *url.URL, customKey []byte) (*M3U8Playlist, error) {
	playlist := &M3U8Playlist{
		BaseURL:   baseURL.String(),
		Segments:  make([]string, 0),
		IsStream:  false,
		Encrypted: false,
		CustomKey: customKey,
	}

	// Parse the playlist content
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check for encryption key
		if strings.HasPrefix(line, "#EXT-X-KEY:") {
			err := parseKeyTag(line, baseURL, playlist)
			if err != nil {
				fmt.Printf("Warning: failed to parse encryption key: %v\n", err)
			}
			continue
		}

		// Check for stream info
		if strings.Contains(line, "#EXT-X-STREAM-INF") {
			playlist.IsStream = true
			continue
		}

		// Skip other comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// This is a segment URL
		segmentURL := resolveURL(baseURL, line)

		// Check if this is a relative URL and we don't have a proper base URL
		if baseURL.Scheme == "file" && !isAbsoluteURL(line) {
			return nil, fmt.Errorf("found relative URL '%s' but no base URL provided. Use -baseurl flag to specify the base URL", line)
		}

		playlist.Segments = append(playlist.Segments, segmentURL)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading playlist: %w", err)
	}

	// If it's a master playlist, we need to download the first variant
	if playlist.IsStream && len(playlist.Segments) > 0 {
		fmt.Printf("Master playlist detected, using first variant: %s\n", playlist.Segments[0])
		return ParseM3U8WithKey(playlist.Segments[0], customKey)
	}

	if len(playlist.Segments) == 0 {
		return nil, fmt.Errorf("no segments found in playlist")
	}

	// If custom key was provided, use it instead of the downloaded key
	if customKey != nil && playlist.Encrypted {
		playlist.Key = customKey
		fmt.Println("✓ Using custom encryption key (skipped key download)")
	}

	return playlist, nil
}

// resolveURL resolves a potentially relative URL against a base URL
func resolveURL(base *url.URL, reference string) string {
	ref, err := url.Parse(reference)
	if err != nil {
		return reference
	}

	resolved := base.ResolveReference(ref)
	return resolved.String()
}

// isAbsoluteURL checks if a URL is absolute (has a scheme)
func isAbsoluteURL(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return parsed.Scheme != ""
}

// parseKeyTag parses the #EXT-X-KEY tag to extract encryption information
func parseKeyTag(line string, baseURL *url.URL, playlist *M3U8Playlist) error {
	// Example: #EXT-X-KEY:METHOD=AES-128,URI="https://example.com/key.key",IV=0x12345678901234567890123456789012

	// Check if method is AES-128
	if !strings.Contains(line, "METHOD=AES-128") {
		// Only support AES-128 for now
		return fmt.Errorf("unsupported encryption method (only AES-128 is supported)")
	}

	playlist.Encrypted = true

	// Extract URI
	uriStart := strings.Index(line, "URI=\"")
	if uriStart == -1 {
		return fmt.Errorf("no URI found in KEY tag")
	}
	uriStart += 5 // Move past URI="
	uriEnd := strings.Index(line[uriStart:], "\"")
	if uriEnd == -1 {
		return fmt.Errorf("malformed URI in KEY tag")
	}
	keyURI := line[uriStart : uriStart+uriEnd]

	// Resolve relative key URL
	playlist.KeyURL = resolveURL(baseURL, keyURI)

	// Extract IV if present
	ivStart := strings.Index(line, "IV=0x")
	if ivStart != -1 {
		ivStart += 5 // Move past IV=0x
		// IV is typically followed by comma or end of line
		ivEnd := strings.Index(line[ivStart:], ",")
		if ivEnd == -1 {
			playlist.KeyIV = line[ivStart:]
		} else {
			playlist.KeyIV = line[ivStart : ivStart+ivEnd]
		}
	}

	// Only download the encryption key if custom key is not provided
	if playlist.CustomKey == nil {
		// Download the encryption key
		fmt.Printf("Downloading encryption key from: %s\n", playlist.KeyURL)
		key, err := DownloadContent(playlist.KeyURL)
		if err != nil {
			return fmt.Errorf("failed to download encryption key: %w", err)
		}

		if len(key) != 16 {
			return fmt.Errorf("invalid key length: expected 16 bytes, got %d", len(key))
		}

		playlist.Key = key
		fmt.Println("Encryption key downloaded successfully")
	} else {
		fmt.Println("Encryption detected, will use custom key (skipping download)")
	}

	return nil
}

// DownloadContent downloads content from a URL and returns it as bytes
func DownloadContent(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add custom headers
	for key, value := range customHeaders {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// DownloadContentWithRetry downloads content with retry logic
func DownloadContentWithRetry(url string, maxRetries int) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retrying with exponential backoff
			waitTime := time.Duration(attempt) * time.Second
			time.Sleep(waitTime)
		}

		data, err := DownloadContent(url)
		if err == nil {
			return data, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
