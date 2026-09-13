package base

import (
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

type DescribedFuncArgs struct {
	// a required argument with a description
	Name string `doc:"the name of the thing"`
	// an optional argument with a description and a json name
	Count  *int    `json:"amount" doc:"how many of them"`
	Secret *string // no doc tag, description stays empty
	Plain  int     `doc:"a required int"`
	Dur    int64   `doc:"how long to wait"`
	Slice  []string
	SkipMe string
}

type DescribedTestOperator struct{}

func (o *DescribedTestOperator) DescribedFunction(ctx *Context, mainInput *OperatorIO, args DescribedFuncArgs) *OperatorIO {
	return MakeEmptyOutput()
}

func (o *DescribedTestOperator) Simple1() *OperatorIO {
	return MakeEmptyOutput()
}

func TestArgumentDescriptions(t *testing.T) {
	gops := MakeFreepsOperators(&DescribedTestOperator{}, nil, NewBaseContextWithReason(logrus.StandardLogger(), ""))
	gop := gops[0]

	list := gop.GetArgumentDescriptions("DescribedFunction")
	byName := map[string]ArgumentDescription{}
	for _, a := range list {
		byName[a.Name] = a
	}

	assert.Equal(t, len(list), 7)

	assert.Equal(t, byName["Name"].Description, "the name of the thing")
	assert.Equal(t, byName["Name"].Required, true)
	assert.Equal(t, byName["Name"].Type, "string")

	assert.Equal(t, byName["Count"].Description, "how many of them")
	assert.Equal(t, byName["Count"].Required, false)
	assert.Equal(t, byName["Count"].Type, "int")

	// doc tags are optional: no tag means empty description, not an error
	assert.Equal(t, byName["Secret"].Description, "")
	assert.Equal(t, byName["Secret"].Required, false)
	assert.Equal(t, byName["Secret"].Type, "string")

	assert.Equal(t, byName["Plain"].Type, "int")
	assert.Equal(t, byName["Plain"].Required, true)
	assert.Equal(t, byName["Dur"].Type, "int64 or duration")
	assert.Equal(t, byName["Slice"].Type, "string list")
	assert.Equal(t, byName["Slice"].Required, false)

	// functions without a parameter struct have no arguments
	assert.Equal(t, len(gop.GetArgumentDescriptions("Simple1")), 0)

	// an unknown function returns an empty list, not an error
	assert.Equal(t, len(gop.GetArgumentDescriptions("NoSuchFunction")), 0)

	// names are matched case insensitively, like everywhere else
	assert.Equal(t, len(gop.GetArgumentDescriptions("describedfunction")), 7)
}

func TestArgumentDescriptionsDynamic(t *testing.T) {
	ctx := NewBaseContextWithReason(logrus.StandardLogger(), "")
	gop := MakeFreepsOperators(&MyDynamicTestOperator{}, nil, ctx)[0]

	// a dynamic function has no parameter struct, so it is described by name only
	list := gop.GetArgumentDescriptions("DynFunc")
	assert.Equal(t, len(list), 1)
	assert.Equal(t, list[0].Name, "DynTestArg")
	assert.Equal(t, list[0].Description, "")
	assert.Equal(t, list[0].Type, "")

	// a static function of the same operator still gets the full description
	list = gop.GetArgumentDescriptions("StaticFunc")
	assert.Equal(t, len(list), 3)
}

// an operator that implements FreepsBaseOperator directly uses the NameOnlyArgumentDescriptions
// default, so it has no types or descriptions
type noDescriptionsOperator struct{}

func (o *noDescriptionsOperator) Execute(ctx *Context, fn string, mainArgs FunctionArguments, mainInput *OperatorIO) *OperatorIO {
	return MakeEmptyOutput()
}
func (o *noDescriptionsOperator) GetFunctions() []string             { return []string{"fn"} }
func (o *noDescriptionsOperator) GetPossibleArgs(fn string) []string { return []string{"Arg1"} }
func (o *noDescriptionsOperator) GetArgSuggestions(fn string, arg string, otherArgs FunctionArguments) map[string]string {
	return nil
}
func (o *noDescriptionsOperator) GetName() string         { return "noDescriptions" }
func (o *noDescriptionsOperator) GetHook() interface{}    { return nil }
func (o *noDescriptionsOperator) StartListening(*Context) {}
func (o *noDescriptionsOperator) Shutdown(*Context)       {}
func (o *noDescriptionsOperator) GetArgumentDescriptions(fn string) []ArgumentDescription {
	return NameOnlyArgumentDescriptions(o, fn)
}

var _ FreepsBaseOperator = &noDescriptionsOperator{}

func TestNameOnlyArgumentDescriptions(t *testing.T) {
	op := &noDescriptionsOperator{}
	desc := op.GetArgumentDescriptions("fn")
	assert.Equal(t, len(desc), 1)
	assert.Equal(t, desc[0].Name, "Arg1")
	assert.Equal(t, desc[0].Description, "")
	assert.Equal(t, desc[0].Type, "")
}
