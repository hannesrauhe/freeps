package utils

import (
	"fmt"
	"os"
)

var dir = ""

// GetTempDir returns the temp dir that was created when the process was started
func GetTempDir() (string, error) {
	var err error = nil
	if dir == "" {
		dir, err = os.MkdirTemp("", "freeps")
	}
	return dir, err
}

// DeleteTempDir deletes the temp dir that was created when the process was started
func DeleteTempDir() error {
	if dir == "" {
		return fmt.Errorf("Freeps Temp Dir does not exist")
	}
	dir = ""
	return os.RemoveAll(dir)
}
