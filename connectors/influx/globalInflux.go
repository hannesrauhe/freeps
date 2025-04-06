package influx

var globalInflux *OperatorInflux

// GetGlobalInfluxInstance returns the global sensor instance, that can be used by other operators to manage their sensors
func GetGlobalInfluxInstance() *OperatorInflux {
	return globalInflux
}
