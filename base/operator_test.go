package base

import (
	"path"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

type MyTestFuncParams struct {
	Param1         string
	Param2         int
	OptParam3      *int
	OptParam4      *string
	OptParam5      *bool
	neversetvar    string
	neversetvarptr *string
	Vars           map[string]string
}

type MyTestOperator struct {
	bla     int
	counter int
}

func (mt *MyTestOperator) MyFavoriteFunction(ctx *Context, mainInput *OperatorIO, mf MyTestFuncParams, args map[string]string) *OperatorIO {
	if mt == nil {
		return MakeOutputError(500, "The parent object was not passed to the function")
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

func (mf *MyTestFuncParams) GetArgSuggestions(argName string) map[string]string {
	return map[string]string{}
}

func TestOpBuilderSuggestions(t *testing.T) {
	gop := MakeFreepsOperator(&MyTestOperator{}, nil, NewContext(logrus.StandardLogger()))
	assert.Assert(t, gop != nil, "")
	assert.Equal(t, gop.GetName(), "mytestoperator")
	fnl := gop.GetFunctions()
	assert.Equal(t, len(fnl), 4)
	assert.Assert(t, cmp.Contains(fnl, "myfavoritefunction"))

	fal := gop.GetPossibleArgs("MyFavoriteFunction")
	assert.Equal(t, len(fal), 5)
	assert.Assert(t, cmp.Contains(fal, "Param1"))

	// sug := gop.GetArgSuggestions("MyFavoriteFunction", "Param1", map[string]string{})
}

func TestOpBuilderExecute(t *testing.T) {
	gop := MakeFreepsOperator(&MyTestOperator{}, nil, NewContext(logrus.StandardLogger()))

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
	assert.Equal(t, output.GetString(), "5")

	// happy path with optional parameters that have names of internal fields
	output = gop.Execute(nil, "MyFavoriteFuNCtion", map[string]string{"Param1": "test", "param2": "12", "neversetvar": "bla", "neversetvarptr": "bla"}, MakeEmptyOutput())
	assert.Assert(t, !output.IsError(), output.GetString())
	assert.Equal(t, output.GetString(), "other")

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
}

type MyTestOperatorConfig struct {
	Enabled bool
}

type MyTestOperatorWithConfig struct {
	bla int
}

var _ FreepsOperatorWithShutdown = &MyTestOperatorWithConfig{}

// implement the FreepsGenericOperatorWithShutdown interface
func (mt *MyTestOperatorWithConfig) Init(ctx *Context) error {
	mt.bla = 42
	return nil
}

func (mt *MyTestOperatorWithConfig) Shutdown(ctx *Context) {
}

func (mt *MyTestOperatorWithConfig) GetConfig() interface{} {
	return &MyTestOperatorConfig{Enabled: true}
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
	gop := MakeFreepsOperator(&MyTestOperatorWithConfig{}, cr, NewContext(logrus.StandardLogger()))

	// happy path without optional parameters
	output := gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "3.14", "TimeParam": "12m"}, MakeEmptyOutput())
	assert.Assert(t, output.IsEmpty(), output.GetString())
}
