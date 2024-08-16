package opalert

type AlertConfig struct {
	Enabled           bool
	SeverityOverrides map[string]int
}
