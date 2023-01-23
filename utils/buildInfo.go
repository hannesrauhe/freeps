package utils

import (
	"fmt"
	"time"
)

var (
	Version        = "dev"
	CommitHash     = "n/a"
	BuildTime      = "n/a"
	StartTimestamp = time.Now()
)

func BuildVersion() string {
	return fmt.Sprintf("%s-%s", Version, CommitHash)
}

func BuildFullVersion() string {
	return fmt.Sprintf("%s-%s (%s)", Version, CommitHash, BuildTime)
}
