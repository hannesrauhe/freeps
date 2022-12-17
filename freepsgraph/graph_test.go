package freepsgraph

import (
	"os"
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

type MockOperator struct {
	DoCount      int
	LastFunction string
	LastJSON     []byte
}

// GetName returns the name of the operator
func (*MockOperator) GetName() string {
	return "mock"
}

func (*MockOperator) Execute(ctx *utils.Context, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	return mainInput
}

func (*MockOperator) GetFunctions() []string {
	return []string{"convert", "convertAll"}
}

func (*MockOperator) GetPossibleArgs(fn string) []string {
	return []string{}
}

func (*MockOperator) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (*MockOperator) Shutdown(ctx *utils.Context) {
}

var _ FreepsOperator = &MockOperator{}

var validGraph = GraphDesc{Operations: []GraphOperationDesc{{Operator: "eval"}}}

func TestOperatorErrorChain(t *testing.T) {
	ctx := utils.NewContext(log.StandardLogger())
	ge := NewGraphEngine(nil, func() {})
	ge.temporaryGraphs["test"] = &GraphInfo{Desc: GraphDesc{Operations: []GraphOperationDesc{
		{Name: "dooropen", Operator: "eval", Function: "eval", Arguments: map[string]string{"valueName": "FieldsWithType.open.FieldValue",
			"valueType": "bool"}},
		{Name: "echook", Operator: "eval", Function: "echo", InputFrom: "dooropen"},
	}, OutputFrom: "echook"}}
	oError := ge.ExecuteGraph(ctx, "test", make(map[string]string), MakeEmptyOutput())
	assert.Assert(t, oError.IsError(), "unexpected output: %v", oError)

	testInput := MakeByteOutput([]byte(`{"FieldsWithType": {"open" : {"FieldValue": "true", "FieldType": "bool"} }}`))
	oTrue := ge.ExecuteGraph(ctx, "test", make(map[string]string), testInput)
	assert.Assert(t, oTrue.IsEmpty(), "unexpected output: %v", oTrue)

	// test that output of single operation is directly returned and not merged
	oDirect := ge.ExecuteOperatorByName(ctx, "eval", "echo", map[string]string{"output": "true"}, MakeEmptyOutput())
	assert.Assert(t, oDirect.IsPlain(), "unexpected output: %v", oDirect)
}

func TestCheckGraph(t *testing.T) {
	ctx := utils.NewContext(log.StandardLogger())
	ge := NewGraphEngine(nil, func() {})
	ge.temporaryGraphs["test_noinput"] = &GraphInfo{Desc: GraphDesc{Operations: []GraphOperationDesc{
		{Operator: "eval", Function: "eval", InputFrom: "NOTEXISTING"},
	}}}
	opIO := ge.CheckGraph("test_noinput")
	assert.Assert(t, opIO.IsError(), "unexpected output: %v", opIO)

	ge.temporaryGraphs["test_noargs"] = &GraphInfo{Desc: GraphDesc{Operations: []GraphOperationDesc{
		{Operator: "eval", Function: "eval", ArgumentsFrom: "NOTEXISTING"},
	}}}
	opIO = ge.CheckGraph("test_noargs")

	assert.Assert(t, opIO.IsError(), "unexpected output: %v", opIO)
	ge.temporaryGraphs["test_noop"] = &GraphInfo{Desc: GraphDesc{Operations: []GraphOperationDesc{
		{Operator: "NOTHERE"},
	}}}

	opIO = ge.CheckGraph("test_noargs")
	assert.Assert(t, opIO.IsError(), "unexpected output: %v", opIO)

	ge.temporaryGraphs["test_valid"] = &GraphInfo{Desc: validGraph}
	opIO = ge.CheckGraph("test_valid")
	assert.Assert(t, !opIO.IsError(), "unexpected output: %v", opIO)

	gd, _ := ge.GetGraphDesc("test_valid")
	assert.Equal(t, gd.Operations[0].Name, "", "original graph should not be modified")

	g, err := NewGraph(ctx, "", gd, ge)
	assert.NilError(t, err)
	assert.Equal(t, g.desc.Operations[0].Name, "#0")
}

func TestGraphStorage(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(log.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)
	ge := NewGraphEngine(cr, func() {})
	ge.AddExternalGraph("test1", &validGraph, "")
	_, err = os.Stat(path.Join(tdir, "externalGraph_test1.json"))
	assert.NilError(t, err)
	ge.AddExternalGraph("test2", &validGraph, "")
	_, err = os.Stat(path.Join(tdir, "externalGraph_test2.json"))
	assert.NilError(t, err)
	ge.AddExternalGraph("test3", &validGraph, "foo.json")
	_, err = os.Stat(path.Join(tdir, "foo.json"))
	assert.NilError(t, err)
	ge.AddExternalGraph("test4", &validGraph, "foo.json")
	_, err = os.Stat(path.Join(tdir, "foo.json"))
	assert.NilError(t, err)
	assert.Equal(t, len(ge.GetAllGraphDesc()), 4)

	ge.DeleteGraph("test4")
	assert.Equal(t, len(ge.GetAllGraphDesc()), 3)
	_, err = os.Stat(path.Join(tdir, "foo.json"))
	assert.NilError(t, err)

	ge.DeleteGraph("test2")
	assert.Equal(t, len(ge.GetAllGraphDesc()), 2)
	_, err = os.Stat(path.Join(tdir, "externalGraph_test2.json"))
	assert.Assert(t, err != nil)

	ge.DeleteGraph("test3")
	assert.Equal(t, len(ge.GetAllGraphDesc()), 1)
	_, err = os.Stat(path.Join(tdir, "foo.json"))
	assert.Assert(t, err != nil)
}
