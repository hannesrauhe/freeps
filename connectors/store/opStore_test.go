package freepsstore

import (
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func prepareStore(t *testing.T) (base.FreepsBaseOperator, *base.Context) {
	ctx := base.NewBaseContextWithReason(logrus.StandardLogger(), "")

	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	s := base.MakeFreepsOperators(&OpStore{}, cr, ctx)[0]
	return s, ctx
}

func testOutput(t *testing.T, fn string, output string) {
	s, ctx := prepareStore(t)

	vars := map[string]string{"namespace": "testing", "key": "test_key", "value": "test_value"}
	input := base.MakePlainOutput("test_value")

	vars["output"] = "empty"
	out := s.Execute(ctx, "setSimpleValue", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	vars["output"] = output
	out = s.Execute(ctx, fn, base.NewFunctionArguments(vars), input)
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
		rmap := map[string]map[string]*base.OperatorIO{}
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
	s, ctx := prepareStore(t)
	vars := map[string]string{"namespace": "testing", "key": "test_key", "value": "test_value", "output": "direct"}
	input := base.MakePlainOutput("test_value")

	out := s.Execute(ctx, "setSimpleValue", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	vars["maxAge"] = "älter als Papa"
	out = s.Execute(ctx, "get", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusBadRequest)

	time.Sleep(time.Millisecond * 5)
	vars["maxAge"] = "2ms"
	out = s.Execute(ctx, "get", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusGone)

	vars["maxAge"] = "2h"
	out = s.Execute(ctx, "get", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !out.IsError())
	assert.Equal(t, out.GetString(), "test_value")

	out = s.Execute(ctx, "setSimpleValue", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusConflict)

	delete(vars, "maxAge")
	out = s.Execute(ctx, "get", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !out.IsError())

	vars["maxAge"] = "2ms"
	out = s.Execute(ctx, "setSimpleValue", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when overwriting value: %v", out)

	out = s.Execute(ctx, "del", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when deleting value: %v", out)

	vars["maxAge"] = "2h"
	out = s.Execute(ctx, "get", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusNotFound)

	// make sure timestamp is also gone
	out = s.Execute(ctx, "setSimpleValue", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError())
}

func TestStoreCompareAndSwap(t *testing.T) {
	s, ctx := prepareStore(t)
	vars := map[string]string{"namespace": "testing", "key": "test_key", "value": "test_value", "output": "direct"}
	input := base.MakePlainOutput("a_new_value")

	out := s.Execute(ctx, "compareAndSwap", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusNotFound)

	out = s.Execute(ctx, "setSimpleValue", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	out = s.Execute(ctx, "compareAndSwap", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !out.IsError())
	assert.Equal(t, out.GetString(), "a_new_value")

	out = s.Execute(ctx, "get", base.NewFunctionArguments(vars), input)
	assert.Equal(t, out.GetString(), "a_new_value")

	out = s.Execute(ctx, "compareAndSwap", base.NewFunctionArguments(vars), input)
	assert.Assert(t, out.IsError())
	assert.Equal(t, out.GetStatusCode(), http.StatusConflict)
}

func TestStoreDynamicArgName(t *testing.T) {
	s, ctx := prepareStore(t)
	vars := map[string]string{"namespace": "testing", "keyargname": "schluessel", "valueargname": "wert", "schluessel": "test_key", "wert": "test_value", "output": "direct"}
	input := base.MakePlainOutput("a_new_value")

	out := s.Execute(ctx, "setSimpleValue", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	out = s.Execute(ctx, "get", base.NewFunctionArguments(vars), input)
	assert.Equal(t, out.GetString(), "test_value")

	out = s.Execute(ctx, "del", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !out.IsError(), "Unexpected error when deleting value: %v", out)

	vars["wert"] = ""
	out = s.Execute(ctx, "set", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !out.IsError())

	delete(vars, "wert")
	out = s.Execute(ctx, "set", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !out.IsError())
}

func TestStoreGetDefault(t *testing.T) {
	s, ctx := prepareStore(t)
	vars := map[string]string{"namespace": "testing", "key": "test_key", "defaultvalue": "mydefault", "output": "direct"}
	input := base.MakePlainOutput("a_new_value")

	out := s.Execute(ctx, "set", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !out.IsError(), "Unexpected error when setting value for tests: %v", out)

	vars["key"] = "test_key2"
	out = s.Execute(ctx, "get", base.NewFunctionArguments(vars), input)
	assert.Equal(t, out.GetString(), "mydefault")
}

func TestStoreSetGetAll(t *testing.T) {
	s, ctx := prepareStore(t)
	vars := map[string]string{"namespace": "testing"}
	input := base.MakeByteOutput([]byte(`{ "v1" : "a_new_value" , "v2" : "second" }`))

	outSet := s.Execute(ctx, "setAll", base.NewFunctionArguments(vars), input)
	assert.Assert(t, !outSet.IsError(), outSet.Output)

	expected := map[string]map[string]*base.OperatorIO{"testing": {}}
	expected["testing"]["v1"] = base.MakeObjectOutput("a_new_value")
	expected["testing"]["v2"] = base.MakeObjectOutput("second")
	outGet := s.Execute(ctx, "getAll", base.NewFunctionArguments(vars), input)
	assert.DeepEqual(t, outGet, base.MakeObjectOutput(expected))

	searchVars := base.NewFunctionArguments(map[string]string{"namespace": "testing", "key": "2", "value": "s", "maxAge": "1h"})
	outSearch := s.Execute(ctx, "search", searchVars, input)
	assert.Assert(t, !outSearch.IsError())
}

func TestStoreUpdateTransaction(t *testing.T) {
	ctx := base.NewBaseContextWithReason(logrus.StandardLogger(), "")

	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	base.MakeFreepsOperators(&OpStore{}, cr, ctx)

	ns := store.GetNamespaceNoError("testing")
	ns.SetValue("v1", base.MakePlainOutput("old_value"), ctx)
	se := ns.UpdateTransaction("v1", func(oldEntry StoreEntry) *base.OperatorIO {
		oldV := oldEntry.GetData()
		if oldV.GetString() != "old_value" {
			t.Errorf("old value is not old_value but %v", oldV.GetString())
			return base.MakeOutputError(500, "old value is not old_value")
		}
		return base.MakePlainOutput("new_value")
	}, ctx)
	if se.IsError() {
		t.Errorf("Error while updating value: %v", se)
	}
	o := se.GetData()
	assert.Equal(t, o.GetString(), "new_value")
	o = ns.GetValue("v1").GetData()
	assert.Equal(t, o.GetString(), "new_value")
	se = ns.UpdateTransaction("v2", func(oldEntry StoreEntry) *base.OperatorIO {
		if oldEntry != NotFoundEntry{
			t.Errorf("old value is not empty but %v", oldEntry)
			return base.MakeOutputError(500, "old value is not empty")
		}
		return base.MakePlainOutput("new_value_2")
	}, ctx)
	o = se.GetData()
	assert.Equal(t, o.GetString(), "new_value_2")
}
