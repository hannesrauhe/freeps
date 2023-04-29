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

func (*MockOperator) Execute(ctx *base.Context, fn string, mainArgs map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
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
func (*MockOperator) Shutdown(ctx *base.Context) {
}

var _ base.FreepsBaseOperator = &MockOperator{}

var validGraph = GraphDesc{Operations: []GraphOperationDesc{{Operator: "eval", Function: "echo"}}}

func TestOperatorErrorChain(t *testing.T) {
	ctx := base.NewContext(log.StandardLogger())
	ge := NewGraphEngine(nil, func() {})
	ge.temporaryGraphs["test"] = &GraphInfo{Desc: GraphDesc{Operations: []GraphOperationDesc{
		{Name: "dooropen", Operator: "eval", Function: "eval", Arguments: map[string]string{"valueName": "FieldsWithType.open.FieldValue",
			"valueType": "bool"}},
		{Name: "echook", Operator: "eval", Function: "echo", InputFrom: "dooropen"},
	}, OutputFrom: "echook"}}
	oError := ge.ExecuteGraph(ctx, "test", make(map[string]string), base.MakeEmptyOutput())
	assert.Assert(t, oError.IsError(), "unexpected output: %v", oError)

	testInput := base.MakeByteOutput([]byte(`{"FieldsWithType": {"open" : {"FieldValue": "true", "FieldType": "bool"} }}`))
	oTrue := ge.ExecuteGraph(ctx, "test", make(map[string]string), testInput)
	assert.Assert(t, oTrue.IsEmpty(), "unexpected output: %v", oTrue)

	// test that output of single operation is directly returned and not merged
	oDirect := ge.ExecuteOperatorByName(ctx, "eval", "echo", map[string]string{"output": "true"}, base.MakeEmptyOutput())
	assert.Assert(t, oDirect.IsPlain(), "unexpected output: %v", oDirect)
}

func TestCheckGraph(t *testing.T) {
	ctx := base.NewContext(log.StandardLogger())
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
	ge := NewGraphEngine(cr, func() {})
	ge.AddExternalGraph("test1", &validGraph, "")
	_, err = os.Stat(path.Join(tdir, "externalGraph_test1.json"))
	assert.NilError(t, err)
	assert.Assert(t, fileIsInList(cr, "externalGraph_test1.json"))

	ge.AddExternalGraph("test2", &validGraph, "")
	_, err = os.Stat(path.Join(tdir, "externalGraph_test2.json"))
	assert.NilError(t, err)
	assert.Assert(t, fileIsInList(cr, "externalGraph_test2.json"))

	ge.AddExternalGraph("test3", &validGraph, "foo.json")
	_, err = os.Stat(path.Join(tdir, "foo.json"))
	assert.NilError(t, err)
	assert.Assert(t, fileIsInList(cr, "foo.json"))

	ge.AddExternalGraph("test4", &validGraph, "foo.json")
	_, err = os.Stat(path.Join(tdir, "foo.json"))
	assert.NilError(t, err)
	assert.Equal(t, len(ge.GetAllGraphDesc()), 4)
	assert.Assert(t, fileIsInList(cr, "foo.json"))

	ge.DeleteGraph("test4")
	assert.Equal(t, len(ge.GetAllGraphDesc()), 3)
	_, err = os.Stat(path.Join(tdir, "foo.json"))
	assert.NilError(t, err)
	assert.Assert(t, fileIsInList(cr, "foo.json"))

	ge.DeleteGraph("test2")
	assert.Equal(t, len(ge.GetAllGraphDesc()), 2)
	_, err = os.Stat(path.Join(tdir, "externalGraph_test2.json"))
	assert.Assert(t, err != nil)
	assert.Assert(t, false == fileIsInList(cr, "externalGraph_test2.json"))

	ge.DeleteGraph("test3")
	assert.Equal(t, len(ge.GetAllGraphDesc()), 1)
	_, err = os.Stat(path.Join(tdir, "foo.json"))
	assert.Assert(t, err != nil)
	assert.Assert(t, false == fileIsInList(cr, "foo.json"))
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
	assert.NilError(t, err)
	ge := NewGraphEngine(cr, func() {})

	expectByTagExtendedExecution := func(tagGroups [][]string, expectedOutputKeys []string) {
		expectedCode := 200
		if expectedOutputKeys == nil {
			expectedCode = 404
		}
		expectOutput(t,
			ge.ExecuteGraphByTagsExtended(base.NewContext(log.StandardLogger()), tagGroups, make(map[string]string), base.MakeEmptyOutput()),
			expectedCode, expectedOutputKeys)
	}

	expectByTagExecution := func(tags []string, expectedOutputKeys []string) {
		expectedCode := 200
		if expectedOutputKeys == nil {
			expectedCode = 404
		}
		expectOutput(t,
			ge.ExecuteGraphByTags(base.NewContext(log.StandardLogger()), tags, make(map[string]string), base.MakeEmptyOutput()),
			expectedCode, expectedOutputKeys)
	}

	expectByTagExecution([]string{"not"}, nil)

	g0 := validGraph
	ge.AddExternalGraph("test0", &g0, "")
	expectByTagExecution([]string{"t1"}, nil)

	g1 := validGraph
	g1.AddTag("t1")
	ge.AddExternalGraph("test1", &g1, "")
	expectByTagExecution([]string{"t1"}, []string{}) //single graph executed with empty output

	g2 := validGraph
	g2.Tags = []string{"t1", "t4"}
	ge.AddExternalGraph("test2", &g2, "")
	expectByTagExecution([]string{"t1"}, []string{"test1", "test2"})

	g3 := validGraph
	g3.Tags = []string{"t1", "t2", "t4"}
	ge.AddExternalGraph("test3", &g3, "foo.json")

	g4 := validGraph
	g4.Tags = []string{"t4"}
	ge.AddExternalGraph("test4", &g4, "foo.json")

	expectByTagExecution([]string{"t1"}, []string{"test1", "test2", "test3"})
	expectByTagExecution([]string{"t1", "t2"}, []string{}) //single graph executed with empty output

	expectByTagExtendedExecution([][]string{{"t1"}, {"t2", "t4"}}, []string{"test2", "test3"})
	expectByTagExtendedExecution([][]string{{"t2", "t4"}}, []string{"test2", "test3", "test4"})

	// test the operator once
	expectOutput(t,
		ge.ExecuteOperatorByName(base.NewContext(log.StandardLogger()), "graphbytag", "t4", map[string]string{}, base.MakeEmptyOutput()),
		200, []string{"test2", "test3", "test4"})

	/* Keytags */

	g5 := validGraph
	g5.Tags = []string{"keytag1:foo", "footag:", "f:a:shiZ:s", ":yes:man"}
	ge.AddExternalGraph("test5", &g5, "foo.json")
	g6 := validGraph
	g6.Tags = []string{"keytag1:bar", "keytag2:bla"}
	ge.AddExternalGraph("test6", &g6, "foo.json")

	expectByTagExtendedExecution([][]string{{"t2", ":yes:man", "keytag2:bla"}, {"t4", "fadabump", "keytag2:bla"}, {"t2", "keytag2:bla"}}, []string{"test3", "test6"})

	v := ge.GetTagValues("keytag1")
	sort.Strings(v)
	assert.DeepEqual(t, v, []string{"bar", "foo"})
	assert.DeepEqual(t, ge.GetTagValues("keytag2"), []string{"bla"})
	assert.DeepEqual(t, ge.GetTagValues("footag"), []string{})
	assert.DeepEqual(t, ge.GetTagValues(""), []string{})
	assert.DeepEqual(t, ge.GetTagValues(":yes"), []string{"man"})
	assert.DeepEqual(t, ge.GetTagValues("f"), []string{"a:shiZ:s"})
	assert.DeepEqual(t, ge.GetTagValues("f:a"), []string{"shiZ:s"})
}
