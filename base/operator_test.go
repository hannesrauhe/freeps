package base

import (
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

type MyTestFuncParams struct {
	Param1              string `json:"param_1"`
	Param2              int
	OptParam3           *int `json:"opt_param_3"`
	OptParam4           *string
	OptParam5           *bool
	OptParamWithDefault *int
	neversetvar         string
	neversetvarptr      *string
	Vars                map[string]string
}

var _ FreepsFunctionParameters = &MyTestFuncParams{}

type MyTestOperator struct {
	bla     int
	counter int
}

func (mt *MyTestOperator) MyFavoriteFunction(ctx *Context, mainInput *OperatorIO, mf MyTestFuncParams, args map[string]string) *OperatorIO {
	if mt == nil {
		return MakeOutputError(500, "The parent object was not passed to the function")
	}

	if mf.OptParamWithDefault == nil {
		return MakeOutputError(500, "The optional parameter with default value was not set")
	}

	if mf.neversetvar != "" {
		return MakeOutputError(500, "The parameter neversetvar was set")
	}
	if mf.neversetvarptr != nil {
		return MakeOutputError(500, "The parameter neversetvarptr was set")
	}

	mt.counter++

	if mf.OptParam3 != nil {
		return MakePlainOutput("3")
	}
	if mf.OptParam4 != nil {
		return MakePlainOutput("4")
	}
	if mf.OptParam5 != nil {
		return MakePlainOutput("5")
	}
	if *mf.OptParamWithDefault != 42 {
		return MakePlainOutput("42!")
	}

	if args != nil && len(args) > 0 {
		return MakePlainOutput("other")
	}

	return MakeEmptyOutput()
}

func (mt *MyTestOperator) Simple1() *OperatorIO {
	return MakePlainOutput("simple1")
}

func (mt *MyTestOperator) Simple2(ctx *Context) *OperatorIO {
	return MakePlainOutput("simple2")
}

func (mt *MyTestOperator) Counter(ctx *Context, mainInput *OperatorIO) *OperatorIO {
	return MakePlainOutput("%v", mt.counter)
}

func (mt *MyTestOperator) AnotherUnusedFunctionWrongReturn(ctx *Context, mainInput *OperatorIO) int {
	return 0
}

func (mt *MyTestOperator) AnotherUnusedFunctionWrongArguments(a int, b string) *OperatorIO {
	return MakeOutputError(500, "This function is invalid and should not be called")
}

func (mf *MyTestFuncParams) GetArgSuggestions(op FreepsOperator, fn string, argName string, otherArgs map[string]string) map[string]string {
	switch argName {
	case "param1":
		return map[string]string{
			"function":  fn,
			"param2":    fmt.Sprint(mf.Param2),
			"optparam4": *mf.OptParam4,
		}
	case "param2":
		return map[string]string{
			"function": fn,
			"param1":   mf.Param1,
		}
	}

	return map[string]string{}
}

func (mf *MyTestFuncParams) InitOptionalParameters(op FreepsOperator, fn string) {
	mf.OptParamWithDefault = new(int)
	*mf.OptParamWithDefault = 42
}

func (mf *MyTestFuncParams) VerifyParameters(op FreepsOperator) *OperatorIO {
	return MakeEmptyOutput()
}

func TestOpBuilderSuggestions(t *testing.T) {
	gops := MakeFreepsOperators(&MyTestOperator{}, nil, NewContext(logrus.StandardLogger()))
	gop := gops[0]
	assert.Assert(t, gop != nil, "")
	assert.Equal(t, gop.GetName(), "MyTestOperator")
	fnl := gop.GetFunctions()
	assert.Equal(t, len(fnl), 4)
	assert.Assert(t, cmp.Contains(fnl, "MyFavoriteFunction"))

	fal := gop.GetPossibleArgs("MyFavoriteFunction")
	assert.Equal(t, len(fal), 6)
	assert.Assert(t, cmp.Contains(fal, "Param1"))

	sug := gop.GetArgSuggestions("MyFavoriteFunction", "Param1", map[string]string{"paRam2": "4", "optparam4": "bla"})
	assert.Equal(t, len(sug), 3)
	assert.Equal(t, sug["function"], "myfavoritefunction")
	assert.Equal(t, sug["param2"], "4")
	assert.Equal(t, sug["optparam4"], "bla")

}

func TestOpBuilderExecute(t *testing.T) {
	gops := MakeFreepsOperators(&MyTestOperator{}, nil, NewContext(logrus.StandardLogger()))
	gop := gops[0]
	// happy path without any parameters
	output := gop.Execute(nil, "simple1", map[string]string{}, MakeEmptyOutput())
	assert.Equal(t, output.GetString(), "simple1")
	// parameters are simply ignored
	output = gop.Execute(nil, "simple2", map[string]string{"Param1": "test", "param2": "12"}, MakeEmptyOutput())
	assert.Equal(t, output.GetString(), "simple2")

	// happy path without optional parameters
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12"}, MakeEmptyOutput())
	assert.Assert(t, output.IsEmpty(), output.GetString())

	// happy path with optional parameters
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparam3": "42"}, MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "3")

	// happy path with optional parameters with JSON names
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"param_1": "test", "param2": "12", "opt_param_3": "42"}, MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "3")

	// happy path with optional parameters
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparam4": "bla"}, MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "4")
	output = gop.Execute(nil, "myFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparam5": "bla"}, MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "5")
	output = gop.Execute(nil, "MyFavoriteFuNCtion", map[string]string{"Param1": "test", "param2": "12", "someotheruserparam": "bla"}, MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "other")
	output = gop.Execute(nil, "counter", map[string]string{}, MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "6")

	// happy path with optional parameters that have names of internal fields
	output = gop.Execute(nil, "MyFavoriteFuNCtion", map[string]string{"Param1": "test", "param2": "12", "neversetvar": "bla", "neversetvarptr": "bla"}, MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "other")

	// happy path with overwritten default value
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparamwithdefault": "12"}, MakeEmptyOutput())
	assert.Equal(t, output.GetString(), "42!")

	// wrong function name
	output = gop.Execute(nil, "MyFavoriteFunctionWrong", map[string]string{"Param1": "test"}, MakeEmptyOutput())
	assert.Assert(t, output.IsError(), "")

	// missing parameter
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param2": "12"}, MakeEmptyOutput())
	assert.Assert(t, output.IsError(), "")

	// wrong type of parameter
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "bla"}, MakeEmptyOutput())
	assert.Assert(t, output.IsError(), "")
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparam3": "notint"}, MakeEmptyOutput())
	assert.Assert(t, output.IsError(), "")
	output = gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "test", "param2": "12", "optparamwithdefault": "blub"}, MakeEmptyOutput())
	assert.Assert(t, output.IsError(), "")
}

type MyTestOperatorConfig struct {
	Enabled bool
}

type MyTestOperatorWithConfig struct {
	bla int
}

var _ FreepsOperatorWithConfig = &MyTestOperatorWithConfig{}

func (mt *MyTestOperatorWithConfig) InitCopyOfOperator(config interface{}, ctx *Context) (FreepsOperatorWithConfig, error) {
	newMt := MyTestOperatorWithConfig{bla: 42}
	return &newMt, nil
}

func (mt *MyTestOperatorWithConfig) GetDefaultConfig() interface{} {
	newC := MyTestOperatorConfig{Enabled: true}
	return &newC
}

type MyOtherTestFuncParameters struct {
	papa      *MyTestOperatorWithConfig
	Param1    float64
	TimeParam time.Duration
}

// implement the FreepsGenericFunction interface
func (mt *MyTestOperatorWithConfig) MyFavoriteFunction(ctx *Context, mainInput *OperatorIO, mf MyOtherTestFuncParameters) *OperatorIO {
	if mt.bla != 42 {
		return MakeOutputError(500, "The parent object was not initialized")
	}
	if mf.TimeParam != 12*time.Minute {
		return MakeOutputError(500, "The time parameter was not set correctly")
	}
	return MakeEmptyOutput()
}

func TestOpBuilderExecuteWithConfig(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)
	gops := MakeFreepsOperators(&MyTestOperatorWithConfig{}, cr, NewContext(logrus.StandardLogger()))
	gop := gops[0]
	// happy path without optional parameters
	output := gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "3.14", "TimeParam": "12m"}, MakeEmptyOutput())
	assert.Assert(t, output.IsEmpty(), output.GetString())
}
