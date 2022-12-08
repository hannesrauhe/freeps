package freepsgraph

import (
	"net/http"
	"testing"
	"time"

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
	out = s.Execute(ctx*Context, fn, vars, input)
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

func TestStoreExpiration(t *testing.T) {
	vars := map[string]string{"namespace": "testing", "key": "test_key", "value": "test_value", "output": "direct"}
	input := MakePlainOutput("test_value")

	s := NewOpStore()
	out := s.Execute("setSimpleValue", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	vars["maxAge"] = "älter als Papa"
	out = s.Execute("get", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusBadRequest)

	time.Sleep(time.Millisecond * 5)
	vars["maxAge"] = "2ms"
	out = s.Execute("get", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusGone)

	vars["maxAge"] = "2h"
	out = s.Execute("get", vars, input)
	assert.Assert(t, !out.IsError())
	assert.Equal(t, out.GetString(), "test_value")

	out = s.Execute("setSimpleValue", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusConflict)

	delete(vars, "maxAge")
	out = s.Execute("get", vars, input)
	assert.Assert(t, !out.IsError())

	vars["maxAge"] = "2ms"
	out = s.Execute("setSimpleValue", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when overwriting value: %v", out)

	out = s.Execute("del", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when deleting value: %v", out)

	vars["maxAge"] = "2h"
	out = s.Execute("get", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusNotFound)

	// make sure timestamp is also gone
	out = s.Execute("setSimpleValue", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError())
}

func TestStoreCompareAndSwap(t *testing.T) {
	vars := map[string]string{"namespace": "testing", "key": "test_key", "value": "test_value", "output": "direct"}
	input := MakePlainOutput("a_new_value")

	s := NewOpStore()
	out := s.Execute("compareAndSwap", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusNotFound)

	out = s.Execute("setSimpleValue", vars, input)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	out = s.Execute("compareAndSwap", vars, input)
	assert.Assert(t, !out.IsError())
	assert.Equal(t, out.GetString(), "a_new_value")

	out = s.Execute("get", vars, input)
	assert.Equal(t, out.GetString(), "a_new_value")

	out = s.Execute("compareAndSwap", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusConflict)
}
