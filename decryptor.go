package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
)

// DecryptSegment decrypts an AES-128 encrypted segment
func DecryptSegment(encryptedData []byte, key []byte, iv string, segmentIndex int) ([]byte, error) {
	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Determine IV
	var ivBytes []byte
	if iv != "" {
		// Use the IV from the playlist
		ivBytes, err = hex.DecodeString(iv)
		if err != nil {
			return nil, fmt.Errorf("failed to decode IV: %w", err)
		}
	} else {
		// If no IV specified, use the segment sequence number as IV (padded to 16 bytes)
		ivBytes = make([]byte, 16)
		// Put the segment index in the last 4 bytes (big-endian)
		ivBytes[12] = byte(segmentIndex >> 24)
		ivBytes[13] = byte(segmentIndex >> 16)
		ivBytes[14] = byte(segmentIndex >> 8)
		ivBytes[15] = byte(segmentIndex)
	}

	if len(ivBytes) != aes.BlockSize {
		return nil, fmt.Errorf("invalid IV length: expected %d bytes, got %d", aes.BlockSize, len(ivBytes))
	}

	// Decrypt using AES-128 CBC
	mode := cipher.NewCBCDecrypter(block, ivBytes)

	// The data must be a multiple of the block size
	if len(encryptedData)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("encrypted data is not a multiple of block size")
	}

	decrypted := make([]byte, len(encryptedData))
	mode.CryptBlocks(decrypted, encryptedData)

	// For TS streams, gomedia will handle the proper structure
	// Just return the decrypted data as-is, let gomedia handle any padding/stuffing
	return decrypted, nil
}
