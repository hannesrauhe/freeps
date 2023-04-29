package operatorbuilder

import (
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

type MyTestFunc struct {
	papa           *MyTestOperator
	Param1         string
	Param2         int
	OptParam3      *int
	OptParam4      *string
	OptParam5      *bool
	neversetvar    string
	neversetvarptr *string
	Vars           map[string]string
}

func (mf *MyTestFunc) Run(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO {
	if mf.papa == nil {
		return base.MakeOutputError(500, "The parent object was not passed to the function")
	}

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

var _ FreepsGenericFunction = &MyTestFunc{}

type MyTestOperator struct {
	bla int
}

func (mt *MyTestOperator) MyFavoriteFunction() *MyTestFunc {
	return &MyTestFunc{papa: mt}
}

func (mt *MyTestOperator) MyFavoriteFunctionReturningAStruct() MyTestFunc {
	return MyTestFunc{papa: mt}
}

func (mt *MyTestOperator) AnotherUnusedFunctionWrongReturn() int {
	return 0
}

func (mt *MyTestOperator) AnotherUnusedFunctionWrongArguments(a int, b string) MyTestFunc {
	return MyTestFunc{papa: mt}
}

func TestOpBuilderSuggestions(t *testing.T) {
	gop := MakeGenericOperator(&MyTestOperator{}, nil)
	assert.Assert(t, gop != nil, "")
	assert.Equal(t, gop.GetName(), "mytestoperator")
	fnl := gop.GetFunctions()
	assert.Equal(t, len(fnl), 1)
	assert.Equal(t, fnl[0], "myfavoritefunction")

	fal := gop.GetPossibleArgs("MyFavoriteFunction")
	assert.Equal(t, len(fal), 5)
	assert.Equal(t, fal[0], "Param1")

	// sug := gop.GetArgSuggestions("MyFavoriteFunction", "Param1", map[string]string{})
}

func TestOpBuilderExecute(t *testing.T) {
	gop := MakeGenericOperator(&MyTestOperator{}, nil)

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

	// happy path with optional parameters that have names of internal fields
	output = gop.Execute(nil, "MyFavoriteFuNCtion", map[string]string{"Param1": "test", "param2": "12", "neversetvar": "bla", "neversetvarptr": "bla"}, base.MakeEmptyOutput())
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

type MyTestOperatorConfig struct {
	Enabled bool
}

type MyTestOperatorWithConfig struct {
	bla int
}

var _ FreepsGenericOperatorWithShutdown = &MyTestOperatorWithConfig{}

// implement the FreepsGenericOperatorWithShutdown interface
func (mt *MyTestOperatorWithConfig) Init() error {
	mt.bla = 42
	return nil
}

func (mt *MyTestOperatorWithConfig) Shutdown(ctx *base.Context) {
}

func (mt *MyTestOperatorWithConfig) GetConfig() interface{} {
	return &MyTestOperatorConfig{Enabled: true}
}

type MyOtherTestFunc struct {
	papa   *MyTestOperatorWithConfig
	Param1 float64
}

// implement the FreepsGenericFunction interface
func (mf *MyOtherTestFunc) Run(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO {
	if mf.papa == nil {
		return base.MakeOutputError(500, "The parent object was not passed to the function")
	}
	if mf.papa.bla != 42 {
		return base.MakeOutputError(500, "The parent object was not initialized")
	}
	return base.MakeEmptyOutput()
}

func (mt *MyTestOperatorWithConfig) MyFavoriteFunction() *MyOtherTestFunc {
	return &MyOtherTestFunc{papa: mt}
}

func TestOpBuilderExecuteWithConfig(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)
	gop := MakeGenericOperator(&MyTestOperatorWithConfig{}, cr)

	// happy path without optional parameters
	output := gop.Execute(nil, "MyFavoriteFunction", map[string]string{"Param1": "3.14", "param2": "12"}, base.MakeEmptyOutput())
	assert.Assert(t, output.IsEmpty(), output.GetString())
}
