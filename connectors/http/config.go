package freepshttp

// HTTPConfig is the config for the http connector
type HTTPConfig struct {
	// Port is the port to listen on
	Port int `json:"port"`
	// enablePprof enables pprof on the given port
	EnablePprof bool `json:"enablePprof"`
	// Flow processing timeout in seconds
	FlowProcessingTimeout int `json:"flowProcessingTimeout"`
}
