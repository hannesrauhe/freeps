package freepsgraph

type OpMutt struct{}

var _ FreepsOperator = &OpMutt{}

func (o *OpMutt) Execute(fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	return MakeEmptyOutput()
}

// GetFunctions returns a list of graphs stored in the engine
func (o *OpMutt) GetFunctions() []string {
	fn := make([]string, 0)
	return fn
}

func (o *OpMutt) GetPossibleArgs(fn string) []string {
	return []string{}
}

func (o *OpMutt) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}
