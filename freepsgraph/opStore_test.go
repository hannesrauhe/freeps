package freepsgraph

import (
	"testing"

	"gotest.tools/v3/assert"
)

func testOutput(t *testing.T, fn string, output string) {
	vars := map[string]string{"namespace": "testing", "key": "test_key", "value": "test_value"}
	input := MakePlainOutput("test_value")

	s := NewOpStore()
	vars["output"] = "empty"
	out := s.Execute("setSimpleValue", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	vars["output"] = output
	out = s.Execute(fn, vars, input)
	assert.Assert(t, out != nil)

	if fn == "del" {
		assert.Assert(t, out.IsEmpty())
		return
	}

	switch output {
	case "direct":
		assert.Assert(t, out.IsPlain())
		rvalue := out.Output.(string)
		assert.Equal(t, rvalue, vars["value"])
	case "arguments":
		rmap, err := out.GetArgsMap()
		assert.NilError(t, err)
		assert.Equal(t, rmap[vars["key"]], vars["value"])
	case "hierarchy":
		rmap := map[string]map[string]*OperatorIO{}
		assert.NilError(t, out.ParseJSON(&rmap))
		assert.Equal(t, rmap[vars["namespace"]][vars["key"]].GetString(), vars["value"])
	case "empty":
		assert.Assert(t, out.IsEmpty())
	case "bool":
		assert.Assert(t, out.IsPlain())
		assert.Equal(t, out.GetString(), "true")
	default:
		assert.Assert(t, out.IsError())
	}
}

func TestStoreOpOutput(t *testing.T) {
	for _, fn := range []string{"get", "getAll", "equals", "setSimpleValue", "set", "del"} {
		for _, output := range []string{"direct", "hierarchy", "arguments", "bool", "empty", "INVALID"} {
			t.Run(fn+"-"+output, func(t *testing.T) {
				testOutput(t, fn, output)
			})
		}
	}
}
