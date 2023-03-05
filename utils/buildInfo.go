package utils

import (
	"fmt"
	"time"
)

var (
	Version        = "dev"
	CommitHash     = "n/a"
	BuildTime      = "n/a"
	Branch         = "n/a"
	StartTimestamp = time.Now()
)

func BuildVersion() string {
	return fmt.Sprintf("%s-%s", Version, CommitHash)
}

func BuildFullVersion() string {
	return fmt.Sprintf("%s-%s (Build at %s from Branch %s)", Version, CommitHash, BuildTime, Branch)
}
