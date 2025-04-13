package fileutils

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
