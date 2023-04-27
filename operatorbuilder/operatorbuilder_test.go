package operatorbuilder

import (
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"gotest.tools/v3/assert"
)

type MyTestFunc struct {
	papa            *MyTestStruct
	Param1          string
	Param2          int
	OptParam3       *int
	OptParam4       *string
	OptParam5       *bool
	someinternalvar string
	Vars            map[string]string
}

func (mf *MyTestFunc) Run(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO {
	if mf.OptParam3 != nil {
		return base.MakePlainOutput("3")
	}
	if mf.OptParam4 != nil {
		return base.MakePlainOutput("4")
	}
	if mf.OptParam5 != nil {
		return base.MakePlainOutput("5")
	}
	if mf.Vars != nil && len(mf.Vars) > 0 {
		return base.MakePlainOutput("other")
	}

	return base.MakeEmptyOutput()
}

var _ FreepsFunction = &MyTestFunc{}

type MyTestStruct struct {
	bla int
}

func (mt *MyTestStruct) MyFavoriteFunction() MyTestFunc {
	return MyTestFunc{papa: mt}
}

func (mt *MyTestStruct) AnotherUnusedFunctionWrongReturn() int {
	return 0
}

func (mt *MyTestStruct) AnotherUnusedFunctionWrongArguments(a int, b string) MyTestFunc {
	return MyTestFunc{papa: mt}
}

func TestOpBuilderSuggestions(t *testing.T) {
	gop := MakeGenericOperator(&MyTestStruct{})
	assert.Assert(t, gop != nil, "")
	assert.Equal(t, gop.GetName(), "MyTestStruct")
	fnl := gop.GetFunctions()
	assert.Equal(t, len(fnl), 1)
	assert.Equal(t, fnl[0], "myfavoritefunction")

	fal := gop.GetPossibleArgs("MyFavoriteFunction")
	assert.Equal(t, len(fal), 6)
	assert.Equal(t, fal[0], "Param1")

	// sug := gop.GetArgSuggestions("MyFavoriteFunction", "Param1", map[string]string{})
}

func TestOpBuilderExecute(t *testing.T) {
	gop := MakeGenericOperator(&MyTestStruct{})
	assert.Assert(t, gop != nil, "")
	assert.Equal(t, gop.GetName(), "MyTestStruct")

	// happy path without optional parameters
	output := gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12"}, base.MakeEmptyOutput())
	assert.Assert(t, output.IsEmpty(), output.GetString())

	// happy path with optional parameters
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparam3": "42"}, base.MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "3")

	// happy path with optional parameters
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparam4": "bla"}, base.MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "4")
	output = gop.Execute(nil, "myFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparam5": "bla"}, base.MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "5")
	output = gop.Execute(nil, "MyFavoriteFuNCtion", map[string]string{"Param1": "test", "param2": "12", "someotheruserparam": "bla"}, base.MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "other")

	// wrong function name
	output = gop.Execute(nil, "MyFavoriteFunctionWrong", map[string]string{"Param1": "test"}, base.MakeEmptyOutput())
	assert.Assert(t, output.IsError(), "")

	// missing parameter
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param2": "12"}, base.MakeEmptyOutput())
	assert.Assert(t, output.IsError(), "")

	// wrong type of parameter
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "bla"}, base.MakeEmptyOutput())
	assert.Assert(t, output.IsError(), "")
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparam3": "notint"}, base.MakeEmptyOutput())
	assert.Assert(t, output.IsError(), "")
}
