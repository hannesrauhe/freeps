package base

// FreepsExampleOperator is needed to exclude functions pre-defined functions
var _ FreepsOperatorWithDynamicFunctions = &FreepsExampleOperator{}
var _ FreepsOperatorWithConfig = &FreepsExampleOperator{}
var _ FreepsOperatorWithShutdown = &FreepsExampleOperator{}
var _ FreepsOperatorWithHook = &FreepsExampleOperator{}

type FreepsExampleOperator struct{}

func (feo *FreepsExampleOperator) GetDynamicFunctions() []string             { return nil }
func (feo *FreepsExampleOperator) GetDynamicPossibleArgs(fn string) []string { return nil }
func (feo *FreepsExampleOperator) GetDynamicArgSuggestions(fn string, arg string, otherArgs FunctionArguments) map[string]string {
	return nil
}
func (feo *FreepsExampleOperator) ExecuteDynamic(ctx *Context, fn string, mainArgs FunctionArguments, mainInput *OperatorIO) *OperatorIO {
	return nil
}
func (feo *FreepsExampleOperator) GetDefaultConfig(fullName string) interface{} { return nil }
func (feo *FreepsExampleOperator) InitCopyOfOperator(ctx *Context, config interface{}, fullOperatorName string) (FreepsOperatorWithConfig, error) {
	return nil, nil
}
func (feo *FreepsExampleOperator) StartListening(ctx *Context) {}
func (feo *FreepsExampleOperator) Shutdown(ctx *Context)       {}
func (feo *FreepsExampleOperator) GetHook() interface{}        { return nil }
