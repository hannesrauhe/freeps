package base

import (
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

type MyDynamicTestOperator struct {
	bla     int
	counter int
}

var _ FreepsOperatorWithDynamicFunctions = &MyDynamicTestOperator{}

func (mt *MyDynamicTestOperator) Simple1() *OperatorIO {
	return MakePlainOutput("simple1")
}

func (mt *MyDynamicTestOperator) Simple2(ctx *Context) *OperatorIO {
	return MakePlainOutput("simple2")
}

type StaticFuncArgs struct {
	Arg1       string
	CommonArg  *string
	CommonArg2 *string
}

func (a *StaticFuncArgs) Arg1Suggestions() []string {
	return []string{"test"}
}

func (mt *MyDynamicTestOperator) StaticFunc(ctx *Context, input *OperatorIO, args StaticFuncArgs) *OperatorIO {
	return MakePlainOutput("staticfunc")
}

func (mt *MyDynamicTestOperator) GetDynamicFunctions() []string {
	return []string{"DynFunc"}
}

func (mt *MyDynamicTestOperator) GetDynamicPossibleArgs(fn string) []string {
	switch fn {
	case "dynfunc":
		return []string{"DynTestArg"}
	}
	return []string{}
}

func (mt *MyDynamicTestOperator) GetDynamicArgSuggestions(fn string, arg string, otherArgs FunctionArguments) map[string]string {
	if arg == "dyntestarg" && fn == "dynfunc" {
		return map[string]string{"DynTestArgValue": "DynTestArgValue"}
	}
	return map[string]string{}
}

func (mt *MyDynamicTestOperator) ExecuteDynamic(ctx *Context, fn string, mainArgs FunctionArguments, mainInput *OperatorIO) *OperatorIO {
	switch fn {
	case "dynfunc":
		return MakePlainOutput("DynFunc Output")
	}
	return MakeOutputError(http.StatusNotFound, "Unknown function %v", fn)
}

func (mt *MyDynamicTestOperator) CommonargSuggestions() []string {
	return []string{"common", "common2", "common3"}
}

func (mt *MyDynamicTestOperator) CoMMonaRg2Suggestions() []string {
	return []string{"common2only"}
}

func TestDynmaicOperator(t *testing.T) {
	ctx := NewContext(logrus.StandardLogger(), "")
	gops := MakeFreepsOperators(&MyDynamicTestOperator{}, nil, ctx)[0]

	out := gops.Execute(ctx, "DynFunc", MakeEmptyFunctionArguments(), MakeEmptyOutput())
	assert.Assert(t, !out.IsError(), "")
	out = gops.Execute(ctx, "DynFunc2", MakeEmptyFunctionArguments(), MakeEmptyOutput())
	assert.Assert(t, out.IsError(), "")
	out = gops.Execute(ctx, "Simple1", MakeEmptyFunctionArguments(), MakeEmptyOutput())
	assert.Assert(t, !out.IsError(), "")
	out = gops.Execute(ctx, "Simple2", MakeEmptyFunctionArguments(), MakeEmptyOutput())
	assert.Assert(t, !out.IsError(), "")
	out = gops.Execute(ctx, "StaticFunc", NewSingleFunctionArgument("Arg1", "test"), MakeEmptyOutput())
	assert.Assert(t, !out.IsError(), "Unexpected error %v", out.GetError())

	// missing arg
	out = gops.Execute(ctx, "StaticFunc", MakeEmptyFunctionArguments(), MakeEmptyOutput())
	assert.Assert(t, out.IsError(), "")

	fn := gops.GetFunctions()
	assert.Assert(t, len(fn) == 4, "Expected 3 functions")

	args := gops.GetPossibleArgs("DynFunc")
	assert.Assert(t, len(args) == 1, "Expected 1 argument, got %v", args)
	args = gops.GetPossibleArgs("DynFunc2")
	assert.Assert(t, len(args) == 0, "Expected 0 arguments, got %v", args)
	args = gops.GetPossibleArgs("Simple1")
	assert.Assert(t, len(args) == 0, "Expected 0 arguments, got %v", args)
	args = gops.GetPossibleArgs("Simple2")
	assert.Assert(t, len(args) == 0, "Expected 0 arguments, got %v", args)
	args = gops.GetPossibleArgs("StaticFunc")
	assert.Assert(t, len(args) == 3, "Expected 3 argument, got %v", args)

	argmap := gops.GetArgSuggestions("DynFunc", "DynTestArg", make(map[string]string))
	assert.Assert(t, len(argmap) == 1, "Expected 1 argument, got %v", argmap)
	argmap = gops.GetArgSuggestions("DynFunc", "DynTestArg2", make(map[string]string))
	assert.Assert(t, len(argmap) == 0, "Expected 0 arguments, got %v", argmap)
	argmap = gops.GetArgSuggestions("DynFunc2", "DynTestArg", make(map[string]string))
	assert.Assert(t, len(argmap) == 0, "Expected 0 arguments, got %v", argmap)
	argmap = gops.GetArgSuggestions("Simple1", "DynTestArg", make(map[string]string))
	assert.Assert(t, len(argmap) == 0, "Expected 0 arguments, got %v", argmap)
	argmap = gops.GetArgSuggestions("Simple2", "DynTestArg", make(map[string]string))
	assert.Assert(t, len(argmap) == 0, "Expected 0 arguments, got %v", argmap)
	argmap = gops.GetArgSuggestions("StaticFunc", "DynTestArg", make(map[string]string))
	assert.Assert(t, len(argmap) == 0, "Expected 0 arguments, got %v", argmap)
	argmap = gops.GetArgSuggestions("StaticFunc", "Arg1", make(map[string]string))
	assert.Assert(t, len(argmap) == 1, "Expected 1 argument, got %v", argmap)
	argmap = gops.GetArgSuggestions("StaticFunc", "CommonArg", make(map[string]string))
	assert.Assert(t, len(argmap) == 3, "Expected 3 argument, got %v", argmap)
	argmap = gops.GetArgSuggestions("StaticFunc", "CommonArG2", make(map[string]string))
	assert.Assert(t, len(argmap) == 1, "Expected 1 argument, got %v", argmap)
}
