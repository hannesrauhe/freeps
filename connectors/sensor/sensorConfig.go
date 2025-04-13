package sensor

type SensorConfig struct {
	Enabled                     bool
	AliasKeys                   []string
	InfluxInstancePerCategory   map[string]string
	InfluxPropertiesPerCategory map[string][]string
}
