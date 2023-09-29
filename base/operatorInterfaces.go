package base

// FreepsBaseOperator provides the interface for all operators used by the graph module
// Operators can either implement this interface directly or use MakeFreepsOperator to convert a struct into an operator
type FreepsBaseOperator interface {
	Execute(ctx *Context, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO

	GetFunctions() []string // returns a list of functions that this operator can execute
	GetPossibleArgs(fn string) []string
	GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string
	GetName() string

	StartListening(*Context)
	Shutdown(*Context)
}

// FreepsOperator is the interface structs need to implement so FreepsOperatorWrapper can create a FreepsOperator from them
type FreepsOperator interface {
	// every exported function that follows the rules given in FreepsFunctionType is a FreepsFunction
}

// FreepsOperatorWithConfig adds methods to support multiple configurations to FreepsOperator
type FreepsOperatorWithConfig interface {
	FreepsOperator
	// GetDefaultConfig returns a copy of the default config
	GetDefaultConfig() interface{}
	// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
	InitCopyOfOperator(config interface{}, ctx *Context) (FreepsOperatorWithConfig, error)
}

// FreepsOperatorWithShutdown adds the Shutdown() method to FreepsOperatorWithConfig
type FreepsOperatorWithShutdown interface {
	FreepsOperator
	// StartListening is called when the graph engine is starting up
	StartListening(ctx *Context)
	// Shutdown is called when the graph engine is shutting down
	Shutdown(ctx *Context)
}

// FreepsFunctionParameters is the interface for a paramter struct that can return ArgumentSuggestions
type FreepsFunctionParameters interface {
	// InitOptionalParameters initializes the optional (pointer) arguments of the parameters struct with default values
	InitOptionalParameters(operator FreepsOperator, fn string)

	// GetArgSuggestions returns a map of possible arguments for the given function and argument name
	GetArgSuggestions(operator FreepsOperator, fn string, argName string, otherArgs map[string]string) map[string]string

	// VerifyParameters checks if the given parameters are valid
	VerifyParameters(operator FreepsOperator) *OperatorIO
}
