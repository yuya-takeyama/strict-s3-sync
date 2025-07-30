package checksum

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"os"
)

const bufferSize = 64 * 1024 // 64KB buffer

// CalculateFileSHA256 calculates SHA-256 checksum of a file and returns base64 encoded string
func CalculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	return CalculateSHA256(file)
}

// CalculateSHA256 calculates SHA-256 checksum from reader and returns base64 encoded string
func CalculateSHA256(r io.Reader) (string, error) {
	hash := sha256.New()
	buffer := make([]byte, bufferSize)

	for {
		n, err := r.Read(buffer)
		if n > 0 {
			if _, err := hash.Write(buffer[:n]); err != nil {
				return "", fmt.Errorf("write to hash: %w", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read: %w", err)
		}
	}

	// Return base64 encoded checksum (same format as S3)
	checksum := hash.Sum(nil)
	return base64.StdEncoding.EncodeToString(checksum), nil
}

// TeeReaderWithChecksum creates a reader that calculates checksum while reading
type TeeReaderWithChecksum struct {
	reader   io.Reader
	hash     hash.Hash
	checksum string
	done     bool
}

// NewTeeReaderWithChecksum creates a new TeeReaderWithChecksum
func NewTeeReaderWithChecksum(r io.Reader) *TeeReaderWithChecksum {
	return &TeeReaderWithChecksum{
		reader: r,
		hash:   sha256.New(),
	}
}

// Read implements io.Reader
func (t *TeeReaderWithChecksum) Read(p []byte) (n int, err error) {
	n, err = t.reader.Read(p)
	if n > 0 {
		if _, werr := t.hash.Write(p[:n]); werr != nil {
			return n, werr
		}
	}
	if err == io.EOF {
		t.done = true
		checksum := t.hash.Sum(nil)
		t.checksum = base64.StdEncoding.EncodeToString(checksum)
	}
	return n, err
}

// Checksum returns the calculated checksum (only valid after EOF)
func (t *TeeReaderWithChecksum) Checksum() (string, error) {
	if !t.done {
		return "", fmt.Errorf("checksum not yet calculated (read not complete)")
	}
	return t.checksum, nil
}

// CompareChecksums compares two base64 encoded checksums
func CompareChecksums(checksum1, checksum2 string) bool {
	return checksum1 == checksum2
}