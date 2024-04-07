package freepsgraph

import (
	"os"
	"path"
	"sort"
	"testing"

	"github.com/hannesrauhe/freeps/base"
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

func (*MockOperator) Execute(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	return input
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

// StartListening (noOp)
func (*MockOperator) StartListening(ctx *base.Context) {
}

// Shutdown (noOp)
func (*MockOperator) Shutdown(ctx *base.Context) {
}

// GetHook returns the hook
func (*MockOperator) GetHook() interface{} {
	return nil
}

var _ base.FreepsBaseOperator = &MockOperator{}

func createValidGraph() GraphDesc {
	return GraphDesc{Operations: []GraphOperationDesc{{Operator: "system", Function: "noop"}}, Source: "test"}
}

func TestOperatorErrorChain(t *testing.T) {
	ctx := base.NewContext(log.StandardLogger(), "")
	ge := NewGraphEngine(ctx, nil, func() {})
	ge.graphs["test"] = &GraphDesc{Operations: []GraphOperationDesc{
		{Name: "dooropen", Operator: "eval", Function: "eval", Arguments: map[string]string{"valueName": "FieldsWithType.open.FieldValue",
			"valueType": "bool"}},
		{Name: "echook", Operator: "eval", Function: "echo", InputFrom: "dooropen"},
	}, OutputFrom: "echook"}
	oError := ge.ExecuteGraph(ctx, "test", base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput())
	assert.Assert(t, oError.IsError(), "unexpected output: %v", oError)

	testInput := base.MakeByteOutput([]byte(`{"FieldsWithType": {"open" : {"FieldValue": "true", "FieldType": "bool"} }}`))
	oTrue := ge.ExecuteGraph(ctx, "test", base.MakeEmptyFunctionArguments(), testInput)
	assert.Assert(t, oTrue.IsEmpty(), "unexpected output: %v", oTrue)

	// test that output of single operation is directly returned and not merged
	oDirect := ge.ExecuteOperatorByName(ctx, "eval", "echo", base.NewSingleFunctionArgument("output", "true"), base.MakeEmptyOutput())
	assert.Assert(t, oDirect.IsPlain(), "unexpected output: %v", oDirect)
}

func TestCheckGraph(t *testing.T) {
	ctx := base.NewContext(log.StandardLogger(), "")
	ge := NewGraphEngine(ctx, nil, func() {})
	ge.graphs["test_noinput"] = &GraphDesc{Operations: []GraphOperationDesc{
		{Operator: "eval", Function: "eval", InputFrom: "NOTEXISTING"},
	}}
	opIO := ge.CheckGraph("test_noinput")
	assert.Assert(t, opIO.IsError(), "unexpected output: %v", opIO)

	ge.graphs["test_noargs"] = &GraphDesc{Operations: []GraphOperationDesc{
		{Operator: "eval", Function: "eval", ArgumentsFrom: "NOTEXISTING"},
	}}
	opIO = ge.CheckGraph("test_noargs")

	assert.Assert(t, opIO.IsError(), "unexpected output: %v", opIO)
	ge.graphs["test_noop"] = &GraphDesc{Operations: []GraphOperationDesc{
		{Operator: "NOTHERE"},
	}}

	opIO = ge.CheckGraph("test_noargs")
	assert.Assert(t, opIO.IsError(), "unexpected output: %v", opIO)

	gv := createValidGraph()
	ge.graphs["test_valid"] = &gv
	opIO = ge.CheckGraph("test_valid")
	assert.Assert(t, !opIO.IsError(), "unexpected output: %v", opIO)

	gd, _ := ge.GetGraphDesc("test_valid")
	assert.Equal(t, gd.Operations[0].Name, "", "original graph should not be modified")

	g, err := NewGraph(ctx, "", gd, ge)
	assert.NilError(t, err)
	assert.Equal(t, g.desc.Operations[0].Name, "#0")
}

func fileIsInList(cr *utils.ConfigReader, graphFile string) bool {
	type T struct {
		GraphsFromFile []string
	}
	ct := T{}
	cr.ReadSectionWithDefaults("graphs", &ct)
	for _, f := range ct.GraphsFromFile {
		if f == graphFile {
			return true
		}
	}
	return false
}

func TestGraphStorage(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(log.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	ctx := base.NewContext(log.StandardLogger(), "")
	ge := NewGraphEngine(ctx, cr, func() {})

	// expect embedded graphs to be loaded
	assert.Equal(t, len(ge.GetAllGraphDesc()), 3)

	gdir := ge.GetGraphDir()
	err = ge.AddGraph(ctx, "test1", createValidGraph(), false)
	assert.NilError(t, err)
	_, err = os.Stat(path.Join(gdir, "test1.json"))
	assert.NilError(t, err)

	eg, exists := ge.GetGraphDesc("test1")
	assert.Assert(t, exists)
	assert.Equal(t, eg.Source, "test")

	assert.Equal(t, len(ge.GetAllGraphDesc()), 4)

	err = ge.AddGraph(ctx, "test2", createValidGraph(), false)
	assert.NilError(t, err)
	_, err = os.Stat(path.Join(gdir, "test2.json"))
	assert.NilError(t, err)
	assert.Equal(t, len(ge.GetAllGraphDesc()), 5)

	g := createValidGraph()
	err = ge.AddGraph(ctx, "test2", g, false)
	assert.ErrorContains(t, err, "already exists")
	assert.Equal(t, len(ge.GetAllGraphDesc()), 5)

	g = createValidGraph()
	err = ge.AddGraph(ctx, "test2", g, true)
	assert.NilError(t, err)

	// check proper caps handling and names
	err = ge.AddGraph(ctx, "Test2", createValidGraph(), false)
	assert.NilError(t, err)
	_, err = os.Stat(path.Join(gdir, "Test2.json"))
	assert.NilError(t, err)
	assert.Equal(t, len(ge.GetAllGraphDesc()), 6)

	gdNocap, err := ge.GetCompleteGraphDesc("test2")
	assert.NilError(t, err)
	assert.Equal(t, gdNocap.GraphID, "test2")
	gdCap, err := ge.GetCompleteGraphDesc("Test2")
	assert.NilError(t, err)
	assert.Equal(t, gdCap.GraphID, "Test2")

	assert.Equal(t, gdNocap.DisplayName, gdCap.DisplayName)

	// check deletion
	_, err = ge.DeleteGraph(ctx, "test2")
	_, exists = ge.GetGraphDesc("test2")
	assert.Assert(t, !exists)
	assert.Equal(t, len(ge.GetAllGraphDesc()), 5)

	_, err = ge.DeleteGraph(ctx, "test1")
	assert.NilError(t, err)
	assert.Equal(t, len(ge.GetAllGraphDesc()), 4)
	_, err = os.Stat(path.Join(gdir, "test2.json"))
	assert.Assert(t, err != nil)
}

func expectOutput(t *testing.T, op *base.OperatorIO, expectedCode int, expectedOutputMapKeys []string) {
	assert.Equal(t, op.GetStatusCode(), expectedCode)
	if expectedOutputMapKeys != nil {
		if len(expectedOutputMapKeys) == 0 {
			assert.Equal(t, op.OutputType, base.Empty)
		} else {
			m, err := op.GetArgsMap()
			assert.NilError(t, err)
			assert.Equal(t, len(expectedOutputMapKeys)+1, len(m)) // add the "_" output
			for _, k := range expectedOutputMapKeys {
				_, ok := m[k]
				assert.Assert(t, ok)
			}
		}
	}
}

func TestGraphExecution(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(log.StandardLogger(), path.Join(tdir, "test_config.json"))
	ctx := base.NewContext(log.StandardLogger(), "")
	assert.NilError(t, err)
	ge := NewGraphEngine(ctx, cr, func() {})

	expectByTagExtendedExecution := func(tagGroups [][]string, expectedOutputKeys []string) {
		expectedCode := 200
		if expectedOutputKeys == nil {
			expectedCode = 404
		}
		expectOutput(t,
			ge.ExecuteGraphByTagsExtended(base.NewContext(log.StandardLogger(), ""), tagGroups, base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput()),
			expectedCode, expectedOutputKeys)
	}

	expectByTagExecution := func(tags []string, expectedOutputKeys []string) {
		expectedCode := 200
		if expectedOutputKeys == nil {
			expectedCode = 404
		}
		expectOutput(t,
			ge.ExecuteGraphByTags(base.NewContext(log.StandardLogger(), ""), tags, base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput()),
			expectedCode, expectedOutputKeys)
	}

	expectByTagExecution([]string{"not"}, nil)

	g0 := createValidGraph()
	err = ge.AddGraph(ctx, "test0", g0, false)
	assert.NilError(t, err)
	expectByTagExecution([]string{"t1"}, nil)

	g1 := createValidGraph()
	g1.AddTags("t1")
	err = ge.AddGraph(ctx, "test1", g1, false)
	assert.NilError(t, err)
	expectByTagExecution([]string{"t1"}, []string{}) //single graph executed with empty output

	g2 := createValidGraph()
	g2.AddTags("t1", "t4")
	err = ge.AddGraph(ctx, "test2", g2, false)
	assert.NilError(t, err)
	expectByTagExecution([]string{"t1"}, []string{"test1", "test2"})

	g3 := createValidGraph()
	g3.AddTags("t1", "t2", "t4")
	err = ge.AddGraph(ctx, "test3", g3, false)
	assert.NilError(t, err)

	g4 := createValidGraph()
	g4.AddTags("t4")
	err = ge.AddGraph(ctx, "test4", g4, false)
	assert.NilError(t, err)

	expectByTagExecution([]string{"t1"}, []string{"test1", "test2", "test3"})
	expectByTagExecution([]string{"t1", "t2"}, []string{}) //single graph executed with empty output

	expectByTagExtendedExecution([][]string{{"t1"}, {"t2", "t4"}}, []string{"test2", "test3"})
	expectByTagExtendedExecution([][]string{{"t2", "t4"}}, []string{"test2", "test3", "test4"})

	// test the operator once
	expectOutput(t,
		ge.ExecuteOperatorByName(base.NewContext(log.StandardLogger(), ""), "graphbytag", "t4", base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput()),
		200, []string{"test2", "test3", "test4"})

	/* Keytags */

	g5 := createValidGraph()
	g5.AddTags("keytag1:foo", "footag:", "f:a:shiZ:s", ":yes:man")
	ge.AddGraph(ctx, "test5", g5, false)
	g6 := createValidGraph()
	g6.AddTags("keytag1:bar", "keytag2:bla")
	ge.AddGraph(ctx, "test6", g6, false)

	expectByTagExtendedExecution([][]string{{"t2", ":yes:man", "keytag2:bla"}, {"t4", "fadabump", "keytag2:bla"}, {"t2", "keytag2:bla"}}, []string{"test3", "test6"})

	v := ge.GetTagValues("keytag1")
	sort.Strings(v)
	assert.DeepEqual(t, v, []string{"bar", "foo"})
	assert.DeepEqual(t, ge.GetTagValues("keytag2"), []string{"bla"})
	assert.DeepEqual(t, ge.GetTagValues("footag"), []string{})
	assert.DeepEqual(t, ge.GetTagValues(""), []string{})
	assert.DeepEqual(t, ge.GetTagValues(":yes"), []string{})
	assert.DeepEqual(t, ge.GetTagValues("f"), []string{"a:shiZ:s"})
	assert.DeepEqual(t, ge.GetTagValues("f:a"), []string{})
}
