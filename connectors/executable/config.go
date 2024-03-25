package freepsexecutable

type ExecutableConfig struct {
	Enabled            bool
	Path               string
	OutputContentType  string
	DefaultArguments   map[string]string
	AvailableArguments map[string]map[string]string
	DefaultEnv         map[string]string
}
