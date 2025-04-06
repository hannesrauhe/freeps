//go:build !noinflux

package influx

import "time"

type OldFluxConnectionConfig struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

type OldFreepsFluxConfig struct {
	InfluxdbConnections []OldFluxConnectionConfig
	Enabled             bool
}

type InfluxConfig struct {
	Enabled            bool
	URL                string
	Token              string
	Org                string
	Bucket             string
	WriteAlertSeverity int
	WriteAlertDuration time.Duration
}
