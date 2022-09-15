package freepsgraph

import (
	"testing"

	"gotest.tools/v3/assert"
)

type MockOperator struct {
	DoCount      int
	LastFunction string
	LastJSON     []byte
}

func (*MockOperator) Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	return mainInput
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

func TestCallFreepsOperator(t *testing.T) {
	ge := NewGraphEngine(nil, func() {})
	ge.configGraphs["test"] = GraphDesc{Operations: []GraphOperationDesc{
		{Name: "dooropen", Operator: "eval", Function: "eval", Arguments: map[string]string{"valueName": "FieldsWithType.open.FieldValue",
			"valueType": "bool"}},
		{Name: "echook", Operator: "eval", Function: "echo", InputFrom: "dooropen"},
	}}
	oError := ge.ExecuteGraph("test", make(map[string]string), MakeEmptyOutput())
	assert.Assert(t, oError.IsError())

	testInput := MakeByteOutput([]byte(`{"FieldsWithType": {"open" : {"FieldValue": "true", "FieldType": "bool"} }}`))
	oTrue := ge.ExecuteGraph("test", make(map[string]string), testInput)
	assert.Assert(t, oTrue.IsEmpty(), "unexpected output: %v", oTrue)
}