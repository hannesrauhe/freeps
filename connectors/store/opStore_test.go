package freepsstore

import (
	"net/http"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func testOutput(t *testing.T, fn string, output string) {
	vars := map[string]string{"namespace": "testing", "key": "test_key", "value": "test_value"}
	input := freepsgraph.MakePlainOutput("test_value")
	ctx := utils.NewContext(logrus.StandardLogger())

	s := NewOpStore()
	vars["output"] = "empty"
	out := s.Execute(ctx, "setSimpleValue", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	vars["output"] = output
	out = s.Execute(ctx, fn, vars, input)
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
		rmap := map[string]map[string]*freepsgraph.OperatorIO{}
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
	for _, fn := range []string{"get", "equals", "setSimpleValue", "set", "del"} {
		for _, output := range []string{"direct", "hierarchy", "arguments", "bool", "empty", "INVALID"} {
			t.Run(fn+"-"+output, func(t *testing.T) {
				testOutput(t, fn, output)
			})
		}
	}
}

func TestStoreExpiration(t *testing.T) {
	vars := map[string]string{"namespace": "testing", "key": "test_key", "value": "test_value", "output": "direct"}
	input := freepsgraph.MakePlainOutput("test_value")
	ctx := utils.NewContext(logrus.StandardLogger())

	s := NewOpStore()
	out := s.Execute(ctx, "setSimpleValue", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	vars["maxAge"] = "älter als Papa"
	out = s.Execute(ctx, "get", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusBadRequest)

	time.Sleep(time.Millisecond * 5)
	vars["maxAge"] = "2ms"
	out = s.Execute(ctx, "get", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusGone)

	vars["maxAge"] = "2h"
	out = s.Execute(ctx, "get", vars, input)
	assert.Assert(t, !out.IsError())
	assert.Equal(t, out.GetString(), "test_value")

	out = s.Execute(ctx, "setSimpleValue", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusConflict)

	delete(vars, "maxAge")
	out = s.Execute(ctx, "get", vars, input)
	assert.Assert(t, !out.IsError())

	vars["maxAge"] = "2ms"
	out = s.Execute(ctx, "setSimpleValue", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when overwriting value: %v", out)

	out = s.Execute(ctx, "del", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when deleting value: %v", out)

	vars["maxAge"] = "2h"
	out = s.Execute(ctx, "get", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusNotFound)

	// make sure timestamp is also gone
	out = s.Execute(ctx, "setSimpleValue", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError())
}

func TestStoreCompareAndSwap(t *testing.T) {
	vars := map[string]string{"namespace": "testing", "key": "test_key", "value": "test_value", "output": "direct"}
	input := freepsgraph.MakePlainOutput("a_new_value")
	ctx := utils.NewContext(logrus.StandardLogger())

	s := NewOpStore()
	out := s.Execute(ctx, "compareAndSwap", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusNotFound)

	out = s.Execute(ctx, "setSimpleValue", vars, input)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	out = s.Execute(ctx, "compareAndSwap", vars, input)
	assert.Assert(t, !out.IsError())
	assert.Equal(t, out.GetString(), "a_new_value")

	out = s.Execute(ctx, "get", vars, input)
	assert.Equal(t, out.GetString(), "a_new_value")

	out = s.Execute(ctx, "compareAndSwap", vars, input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusConflict)
}

func TestStoreSetGetAll(t *testing.T) {
	vars := map[string]string{"namespace": "testing"}
	input := freepsgraph.MakeByteOutput([]byte(`{ "v1" : "a_new_value" , "v2" : "second" }`))
	ctx := utils.NewContext(logrus.StandardLogger())

	s := NewOpStore()
	outSet := s.Execute(ctx, "setAll", vars, input)
	assert.Assert(t, !outSet.IsError(), outSet.Output)

	expected := map[string]map[string]*freepsgraph.OperatorIO{"testing": {}}
	expected["testing"]["v1"] = freepsgraph.MakeObjectOutput("a_new_value")
	expected["testing"]["v2"] = freepsgraph.MakeObjectOutput("second")
	outGet := s.Execute(ctx, "getAll", vars, input)
	assert.DeepEqual(t, outGet, freepsgraph.MakeObjectOutput(expected))

	searchVars := map[string]string{"namespace": "testing", "key": "2", "value": "s", "maxAge": "1h"}
	outSearch := s.Execute(ctx, "search", searchVars, input)
	assert.Assert(t, !outSearch.IsError())
}
