package base

// FreepsOperator provides the interface for all operators used by the graph module
type FreepsOperator interface {
	Execute(ctx *Context, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO

	GetFunctions() []string // returns a list of functions that this operator can execute
	GetPossibleArgs(fn string) []string
	GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string
	GetName() string
	Shutdown(*Context)
}
