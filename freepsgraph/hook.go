package freepsgraph

type FreepsHook interface {
	OnExecute(graphName string, mainArgs map[string]string, mainInput *OperatorIO) error
	Shutdown() error
}
