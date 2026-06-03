package utils

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// SafeIntToInt32 converts int to int32 with overflow check.
// Returns 0 if the value overflows int32 range.
func SafeIntToInt32(v int) int32 {
	if v > math.MaxInt32 || v < math.MinInt32 {
		return 0
	}
	return int32(v)
}

// SafeIntToUint8 converts int to uint8 with overflow check.
// Returns 0 if the value overflows uint8 range.
func SafeIntToUint8(v int) uint8 {
	if v > math.MaxUint8 || v < 0 {
		return 0
	}
	return uint8(v)
}

// SafeReadFile reads a file after validating the path does not contain
// directory traversal sequences. This mitigates G304 (CWE-22).
func SafeReadFile(path string) ([]byte, error) {
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("invalid file path: %s", path)
	}
	// #nosec G304 — path traversal is checked above
	return os.ReadFile(path)
}
