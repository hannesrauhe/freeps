package sensor

var globalSensor *OpSensor

// GetGlobalSensor returns the global sensor instance, that can be used by other operators to manage their sensors
func GetGlobalSensor() *OpSensor {
	return globalSensor
}
