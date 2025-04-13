package influx

var globalInflux map[string]*OperatorInflux

// GetGlobalInfluxInstance returns the global sensor instance, that can be used by other operators to manage their sensors
func GetGlobalInfluxInstance(instanceName string) *OperatorInflux {
	if globalInflux == nil {
		return nil
	}
	if op, ok := globalInflux[instanceName]; ok {
		return op
	}
	return nil
}
