package influx

type InfluxConfig struct {
	URL     string
	Token   string
	Org     string
	Bucket  string
	Enabled bool
}
