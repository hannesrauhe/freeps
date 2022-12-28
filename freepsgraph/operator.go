package freepsgraph

import (
	"github.com/hannesrauhe/freeps/utils"
)

type FreepsOperator interface {
	// GetOutputType() OutputT
	Execute(ctx *utils.Context, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO

	GetFunctions() []string // returns a list of functions that this operator can execute
	GetPossibleArgs(fn string) []string
	GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string
	GetName() string
	Shutdown(*utils.Context)
}
