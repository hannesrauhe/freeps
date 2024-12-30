package freepsflow

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

func createValidFlow() FlowDesc {
	return FlowDesc{Operations: []FlowOperationDesc{{Operator: "system", Function: "noop"}}, Source: "test"}
}

func TestOperatorErrorChain(t *testing.T) {
	ctx := base.NewBaseContextWithReason(log.StandardLogger(), "")
	ge := NewFlowEngine(ctx, nil, func() {})
	ge.flows["test"] = &FlowDesc{Operations: []FlowOperationDesc{
		{Name: "dooropen", Operator: "eval", Function: "eval", Arguments: map[string]string{"valueName": "FieldsWithType.open.FieldValue",
			"valueType": "bool"}, InputFrom: "_"},
		{Name: "echook", Operator: "eval", Function: "echo", InputFrom: "dooropen"},
	}, OutputFrom: "echook"}
	oError := ge.ExecuteFlow(ctx, "test", base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput())
	assert.Assert(t, oError.IsError(), "unexpected output: %v", oError)

	testInput := base.MakeByteOutput([]byte(`{"FieldsWithType": {"open" : {"FieldValue": "true", "FieldType": "bool"} }}`))
	oTrue := ge.ExecuteFlow(ctx, "test", base.MakeEmptyFunctionArguments(), testInput)
	assert.Assert(t, oTrue.IsEmpty(), "unexpected output: %v", oTrue)

	// test that output of single operation is directly returned and not merged
	oDirect := ge.ExecuteOperatorByName(ctx, "eval", "echo", base.NewSingleFunctionArgument("output", "true"), base.MakeEmptyOutput())
	assert.Assert(t, oDirect.IsPlain(), "unexpected output: %v", oDirect)
}

func TestCheckFlow(t *testing.T) {
	ctx := base.NewBaseContextWithReason(log.StandardLogger(), "")
	ge := NewFlowEngine(ctx, nil, func() {})
	ge.flows["test_noinput"] = &FlowDesc{Operations: []FlowOperationDesc{
		{Operator: "eval", Function: "eval", InputFrom: "NOTEXISTING"},
	}}
	opIO := ge.CheckFlow("test_noinput")
	assert.Assert(t, opIO.IsError(), "unexpected output: %v", opIO)

	ge.flows["test_noargs"] = &FlowDesc{Operations: []FlowOperationDesc{
		{Operator: "eval", Function: "eval", ArgumentsFrom: "NOTEXISTING"},
	}}
	opIO = ge.CheckFlow("test_noargs")

	assert.Assert(t, opIO.IsError(), "unexpected output: %v", opIO)
	ge.flows["test_noop"] = &FlowDesc{Operations: []FlowOperationDesc{
		{Operator: "NOTHERE"},
	}}

	opIO = ge.CheckFlow("test_noargs")
	assert.Assert(t, opIO.IsError(), "unexpected output: %v", opIO)

	gv := createValidFlow()
	ge.flows["test_valid"] = &gv
	opIO = ge.CheckFlow("test_valid")
	assert.Assert(t, !opIO.IsError(), "unexpected output: %v", opIO)

	gd, _ := ge.GetFlowDesc("test_valid")
	assert.Equal(t, gd.Operations[0].Name, "", "original flow should not be modified")

	g, err := NewFlow(ctx, "", gd, ge)
	assert.NilError(t, err)
	assert.Equal(t, g.desc.Operations[0].Name, "#0")
}

func fileIsInList(cr *utils.ConfigReader, flowFile string) bool {
	type T struct {
		FlowsFromFile []string
	}
	ct := T{}
	cr.ReadSectionWithDefaults("flows", &ct)
	for _, f := range ct.FlowsFromFile {
		if f == flowFile {
			return true
		}
	}
	return false
}

func TestFlowStorage(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(log.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	ctx := base.NewBaseContextWithReason(log.StandardLogger(), "")
	ge := NewFlowEngine(ctx, cr, func() {})

	// expect embedded flows to be loaded
	assert.Equal(t, len(ge.GetAllFlowDesc()), 2)

	gdir := ge.GetFlowDir()
	err = ge.AddFlow(ctx, "test1", createValidFlow(), false)
	assert.NilError(t, err)
	_, err = os.Stat(path.Join(gdir, "test1.json"))
	assert.NilError(t, err)

	eg, exists := ge.GetFlowDesc("test1")
	assert.Assert(t, exists)
	assert.Equal(t, eg.Source, "test")

	assert.Equal(t, len(ge.GetAllFlowDesc()), 3)

	err = ge.AddFlow(ctx, "test2", createValidFlow(), false)
	assert.NilError(t, err)
	_, err = os.Stat(path.Join(gdir, "test2.json"))
	assert.NilError(t, err)
	assert.Equal(t, len(ge.GetAllFlowDesc()), 4)

	g := createValidFlow()
	err = ge.AddFlow(ctx, "test2", g, false)
	assert.ErrorContains(t, err, "already exists")
	assert.Equal(t, len(ge.GetAllFlowDesc()), 4)

	g = createValidFlow()
	err = ge.AddFlow(ctx, "test2", g, true)
	assert.NilError(t, err)

	// check proper caps handling and names
	err = ge.AddFlow(ctx, "Test2", createValidFlow(), false)
	assert.NilError(t, err)
	_, err = os.Stat(path.Join(gdir, "Test2.json"))
	assert.NilError(t, err)
	assert.Equal(t, len(ge.GetAllFlowDesc()), 5)

	gdNocap, err := ge.GetCompleteFlowDesc("test2")
	assert.NilError(t, err)
	assert.Equal(t, gdNocap.FlowID, "test2")
	gdCap, err := ge.GetCompleteFlowDesc("Test2")
	assert.NilError(t, err)
	assert.Equal(t, gdCap.FlowID, "Test2")

	assert.Equal(t, gdNocap.DisplayName, gdCap.DisplayName)

	// check deletion
	_, err = ge.DeleteFlow(ctx, "test2")
	_, exists = ge.GetFlowDesc("test2")
	assert.Assert(t, !exists)
	assert.Equal(t, len(ge.GetAllFlowDesc()), 4)

	_, err = ge.DeleteFlow(ctx, "test1")
	assert.NilError(t, err)
	assert.Equal(t, len(ge.GetAllFlowDesc()), 3)
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

func TestFlowExecution(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(log.StandardLogger(), path.Join(tdir, "test_config.json"))
	ctx := base.NewBaseContextWithReason(log.StandardLogger(), "")
	assert.NilError(t, err)
	ge := NewFlowEngine(ctx, cr, func() {})

	expectByTagExtendedExecution := func(tagGroups [][]string, expectedOutputKeys []string) {
		expectedCode := 200
		if expectedOutputKeys == nil {
			expectedCode = 404
		}
		expectOutput(t,
			ge.ExecuteFlowByTagsExtended(base.NewBaseContextWithReason(log.StandardLogger(), ""), tagGroups, base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput()),
			expectedCode, expectedOutputKeys)
	}

	expectByTagExecution := func(tags []string, expectedOutputKeys []string) {
		expectedCode := 200
		if expectedOutputKeys == nil {
			expectedCode = 404
		}
		expectOutput(t,
			ge.ExecuteFlowByTags(base.NewBaseContextWithReason(log.StandardLogger(), ""), tags, base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput()),
			expectedCode, expectedOutputKeys)
	}

	expectByTagExecution([]string{"not"}, nil)

	g0 := createValidFlow()
	err = ge.AddFlow(ctx, "test0", g0, false)
	assert.NilError(t, err)
	expectByTagExecution([]string{"t1"}, nil)

	g1 := createValidFlow()
	g1.AddTags("t1")
	err = ge.AddFlow(ctx, "test1", g1, false)
	assert.NilError(t, err)
	expectByTagExecution([]string{"t1"}, []string{}) //single flow executed with empty output

	g2 := createValidFlow()
	g2.AddTags("t1", "t4")
	err = ge.AddFlow(ctx, "test2", g2, false)
	assert.NilError(t, err)
	expectByTagExecution([]string{"t1"}, []string{"test1", "test2"})

	g3 := createValidFlow()
	g3.AddTags("t1", "t2", "t4")
	err = ge.AddFlow(ctx, "test3", g3, false)
	assert.NilError(t, err)

	g4 := createValidFlow()
	g4.AddTags("t4")
	err = ge.AddFlow(ctx, "test4", g4, false)
	assert.NilError(t, err)

	expectByTagExecution([]string{"t1"}, []string{"test1", "test2", "test3"})
	expectByTagExecution([]string{"t1", "t2"}, []string{}) //single flow executed with empty output

	expectByTagExtendedExecution([][]string{{"t1"}, {"t2", "t4"}}, []string{"test2", "test3"})
	expectByTagExtendedExecution([][]string{{"t2", "t4"}}, []string{"test2", "test3", "test4"})

	// test the operator once
	expectOutput(t,
		ge.ExecuteOperatorByName(base.NewBaseContextWithReason(log.StandardLogger(), ""), "flowbytag", "t4", base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput()),
		200, []string{"test2", "test3", "test4"})

	/* Keytags */

	g5 := createValidFlow()
	g5.AddTags("keytag1:foo", "footag:", "f:a:shiZ:s", ":yes:man")
	assert.Equal(t, g5.GetTagValue("keytag1"), "foo")
	assert.Equal(t, g5.GetTagValue("keyTAG1"), "foo")
	assert.Equal(t, g5.GetTagValue("footag"), "")
	assert.Equal(t, g5.GetTagValue("NOPE"), "")
	ge.AddFlow(ctx, "test5", g5, false)
	g6 := createValidFlow()
	g6.AddTags("keytag1:bar", "keytag2:bla")
	ge.AddFlow(ctx, "test6", g6, false)

	expectByTagExtendedExecution([][]string{{"t2", ":yes:man", "keytag2:bla"}, {"t4", "fadabump", "keytag2:bla"}, {"t2", "keytag2:bla"}}, []string{"test3", "test6"})

	v := ge.GetTagValues("keytag1")
	sort.Strings(v)
	assert.DeepEqual(t, v, []string{"bar", "foo"})
	assert.DeepEqual(t, ge.GetTagValues("keytag2"), []string{"bla"})
	assert.DeepEqual(t, ge.GetTagValues("keytag2", "keytag1"), []string{"bla"})
	assert.DeepEqual(t, ge.GetTagValues("keytag1", "keytag2"), []string{"bar"})
	assert.DeepEqual(t, ge.GetTagValues("keytag2", "tag_that_doesn't_exist"), []string{})
	assert.DeepEqual(t, ge.GetTagValues("footag"), []string{})
	assert.DeepEqual(t, ge.GetTagValues(""), []string{})
	assert.DeepEqual(t, ge.GetTagValues(":yes"), []string{})
	assert.DeepEqual(t, ge.GetTagValues("f"), []string{"a:shiZ:s"})
	assert.DeepEqual(t, ge.GetTagValues("f:a"), []string{})
}

func test_replace_args(ctx *base.Context, ge *FlowEngine, input1 string, input2 string) *base.OperatorIO {
	op1 := FlowOperationDesc{Name: "echo_output", Operator: "eval", Function: "echo", Arguments: map[string]string{"output": input1}}
	op2 := FlowOperationDesc{Name: "stat_output", Operator: "system", Function: "stats", Arguments: map[string]string{"statType": input1}}
	op3 := FlowOperationDesc{Name: "output", Operator: "eval", Function: "echo", Arguments: map[string]string{"output": input2}}
	g0 := FlowDesc{Operations: []FlowOperationDesc{op1, op2, op3}, Source: "test", OutputFrom: "output"}

	ge.AddFlow(ctx, "test0", g0, true)
	return ge.ExecuteFlow(ctx, "test0", base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput())
}

func TestArgumentReplacement(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(log.StandardLogger(), path.Join(tdir, "test_config.json"))
	ctx := base.NewBaseContextWithReason(log.StandardLogger(), "")
	assert.NilError(t, err)
	ge := NewFlowEngine(ctx, cr, func() {})

	/* test simple string replacement */
	r := test_replace_args(ctx, ge, "test", "Operator output was: ${echo_output}")
	assert.Equal(t, r.GetString(), "Operator output was: test")
	r = test_replace_args(ctx, ge, "", "${foo")
	assert.Equal(t, r.GetString(), "${foo")
	r = test_replace_args(ctx, ge, "cpu", "${stat_output}")
	s := r.GetString()
	assert.Assert(t, s[0] == '{') /* just check if it looks like a json object */

	/* test if objects in maps can be accessed like this */
	r = test_replace_args(ctx, ge, "cpu", "${stat_output.Nice}")
	assert.Assert(t, r.GetString() != "") /* any string is good, as long as it's not an error */

	r = test_replace_args(ctx, ge, "cpu", "${echo_output}: ${stat_output.Nice}")
	assert.Assert(t, utils.StringStartsWith(r.GetString(), "cpu: ")) /* still don't care about the value of Nice */

	/* output does not exist */
	r = test_replace_args(ctx, ge, "cpu", "${doesntexist}")
	assert.Equal(t, r.GetError().Error(), "Output \"doesntexist\" not found")
	r = test_replace_args(ctx, ge, "cpu", "${doesntexist.foo}")
	assert.Equal(t, r.GetError().Error(), "Output \"doesntexist\" not found")

	/* output is not a map */
	r = test_replace_args(ctx, ge, "cpu", "${echo_output.Nice}")
	assert.Assert(t, r.IsError())
	assert.Equal(t, r.GetError().Error(), "Cannot get \"Nice\" from \"echo_output\": Output is not convertible to type map, type is string")

	/* key in map does not exist */
	r = test_replace_args(ctx, ge, "cpu", "${stat_output.doesntexist}")
	assert.Equal(t, r.GetError().Error(), "Variable \"doesntexist\" not found in output \"stat_output\"")
}

func TestIfElseInputLogic(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(log.StandardLogger(), path.Join(tdir, "test_config.json"))
	ctx := base.NewBaseContextWithReason(log.StandardLogger(), "")
	assert.NilError(t, err)
	ge := NewFlowEngine(ctx, cr, func() {})

	testFlow := FlowDesc{Operations: []FlowOperationDesc{
		{Name: "success", Operator: "system", Function: "echo", Arguments: map[string]string{"output": "success"}},
		{Name: "fail", Operator: "system", Function: "fail"},
		/* should be executed because "success" succeeded */
		{Name: "echo_on_success", Operator: "system", Function: "echo", Arguments: map[string]string{"output": "echo_on_success"}, ExecuteOnSuccessOf: "success"},
		/* should be executed because "fail" failed */
		{Name: "echo_on_fail", Operator: "system", Function: "echo", Arguments: map[string]string{"output": "echo_on_fail"}, ExecuteOnFailOf: "fail"},
		/* should be executed because "success" succeeded and "fail" failed */
		{Name: "echo_on_success_fail", Operator: "system", Function: "echo", Arguments: map[string]string{"output": "echo_on_success_fail"}, ExecuteOnSuccessOf: "success", ExecuteOnFailOf: "fail"},
		/* should echo the main input */
		{Name: "echo_main_input", Operator: "system", Function: "echo", InputFrom: "_"},
		/* should not be executed because "fail" did not succeed */
		{Name: "no_echo_main_input_on_success", Operator: "system", Function: "echo", InputFrom: "_", ExecuteOnSuccessOf: "fail"},
		/* should not be executed because "success" did not fail */
		{Name: "no_echo_main_input_on_fail", Operator: "system", Function: "echo", InputFrom: "_", ExecuteOnFailOf: "success"},
		/* should be executed but return an emptry result because there is no input */
		{Name: "echo_empty_input", Operator: "system", Function: "echo", ExecuteOnSuccessOf: "success"},
		/* should echo the first output of the "success" operation*/
		{Name: "echo_first_output", Operator: "system", Function: "echo", InputFrom: "success"},
		/* should echo the first output of the "success" operation because "fail" failed */
		{Name: "echo_first_output_on_fail", Operator: "system", Function: "echo", InputFrom: "success", ExecuteOnFailOf: "fail"},
		{Name: "no_echo_first_output_on_success", Operator: "system", Function: "echo", InputFrom: "success", ExecuteOnSuccessOf: "fail"},
		/* should not be executed because "fail" did not succeed */
		{Name: "no_echo_first_output", Operator: "system", Function: "echo", InputFrom: "fail"},
	}, Source: "test"}

	ge.AddFlow(ctx, "test", testFlow, true)
	out := ge.ExecuteFlow(ctx, "test", base.MakeEmptyFunctionArguments(), base.MakePlainOutput("MainInput"))
	outInt := out.GetObject()
	outMap := outInt.(map[string]*base.OperatorIO)
	assert.Equal(t, outMap["success"].GetString(), "success")
	assert.Equal(t, outMap["echo_on_success"].GetString(), "echo_on_success")
	assert.Equal(t, outMap["echo_on_fail"].GetString(), "echo_on_fail")
	assert.Equal(t, outMap["echo_on_success_fail"].GetString(), "echo_on_success_fail")
	assert.Equal(t, outMap["echo_main_input"].GetString(), "MainInput")
	assert.Assert(t, outMap["echo_empty_input"].IsEmpty())
	assert.Equal(t, outMap["echo_first_output"].GetString(), "success")
	assert.Equal(t, outMap["echo_first_output_on_fail"].GetString(), "success")

	assert.Assert(t, outMap["fail"].IsError())
	assert.Assert(t, outMap["no_echo_main_input_on_success"].IsError())
	assert.Assert(t, outMap["no_echo_main_input_on_fail"].IsError())
	assert.Assert(t, outMap["no_echo_first_output"].IsError())
}
