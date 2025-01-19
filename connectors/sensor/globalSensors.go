package sensor

var globalSensor *OpSensor

// GetGlobalSensors returns the global sensor instance, that can be used by other operators to manage their sensors
func GetGlobalSensors() *OpSensor {
	return globalSensor
}
