package influx

var globalInflux map[string]*OperatorInflux

// GetGlobalInfluxInstance returns the global Influx instance with the given name, enabling other operators to use influx directly
func GetGlobalInfluxInstance(instanceName string) *OperatorInflux {
	if globalInflux == nil {
		return nil
	}
	if op, ok := globalInflux[instanceName]; ok {
		return op
	}
	return nil
}
