package freepsflux

type InfluxdbConfig struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

type FreepsFluxConfig struct {
	InfluxdbConnections []InfluxdbConfig
	IgnoreNotPresent    bool
	Enabled             bool
}

var DefaultConfig = FreepsFluxConfig{[]InfluxdbConfig{}, false, true}
