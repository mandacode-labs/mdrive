package utils

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// SafeIntToInt32 converts int to int32 with overflow check.
func SafeIntToInt32(v int) (int32, error) {
	if v > math.MaxInt32 || v < math.MinInt32 {
		return 0, fmt.Errorf("value %d overflows int32 range", v)
	}
	return int32(v), nil
}

// SafeIntToUint8 converts int to uint8 with overflow check.
func SafeIntToUint8(v int) (uint8, error) {
	if v > math.MaxUint8 || v < 0 {
		return 0, fmt.Errorf("value %d overflows uint8 range", v)
	}
	return uint8(v), nil
}

// SafeReadFile reads a file after validating the path.
// It resolves to absolute, cleans the path, and checks for directory traversal.
func SafeReadFile(path string) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	clean := filepath.Clean(abs)
	if strings.Contains(clean, "..") {
		return nil, fmt.Errorf("invalid file path: %s", path)
	}
	return os.ReadFile(clean)
}
