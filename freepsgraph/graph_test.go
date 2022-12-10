package freepsgraph

import (
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

var _ FreepsOperator = &MockOperator{}

const testGraph = `
"mqttaction": {
	"Actions": [
		{
			"Fn": "pushfields",
			"Mod": "flux"
		},
		{
			"Args": {
				"valueName": "FieldsWithType.open.FieldValue",
				"valueType": "bool"
			},
			"Fn": "eval",
			"FwdTemplateName": "dooropen",
			"Mod": "eval"
		},
		{
			"Args": {
				"operand": "20",
				"operation": "lt",
				"valueName": "FieldsWithType.battery.FieldValue",
				"valueType": "int"
			},
			"Fn": "eval",
			"FwdTemplateName": "phonebatterylow",
			"Mod": "eval"
		}
	]
}
`

func TestOperatorErrorChain(t *testing.T) {
	ctx := utils.NewContext(log.StandardLogger())
	ge := NewGraphEngine(nil, func() {}, map[string]FreepsOperator{})
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
	ge := NewGraphEngine(nil, func() {}, map[string]FreepsOperator{})
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

	ge.temporaryGraphs["test_valid"] = &GraphInfo{Desc: GraphDesc{Operations: []GraphOperationDesc{
		{Operator: "eval"},
	}}}
	opIO = ge.CheckGraph("test_valid")
	assert.Assert(t, !opIO.IsError(), "unexpected output: %v", opIO)

	gd, _ := ge.GetGraphDesc("test_valid")
	assert.Equal(t, gd.Operations[0].Name, "", "original graph should not be modified")

	g, err := NewGraph(ctx, "", gd, ge)
	assert.NilError(t, err)
	assert.Equal(t, g.desc.Operations[0].Name, "#0")
}
