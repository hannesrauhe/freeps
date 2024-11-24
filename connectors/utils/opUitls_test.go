package freepsutils

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestFlattenWithRegexp(t *testing.T) {
	o := &OpUtils{}
	ctx := base.NewContext(logrus.StandardLogger(), "")
	input := base.MakeObjectOutput(map[string]interface{}{
		"a": "1",
		"b": "2",
		"c": "3",
		"params": map[string]interface{}{
			"foo":     "4",
			"iconBar": "5",
		},
	})

	// include regexp: all keys that start with "params", exclude regexp: all keys that contain "icon"
	out := o.Flatten(ctx, input, FlattenArgs{IncludeRegexp: utils.StringPtr("^params"), ExcludeRegexp: utils.StringPtr("icon")})
	if out.IsError() {
		t.Errorf("Flatten returned error: %v", out)
	}
	if !out.IsObject() {
		t.Errorf("Flatten did not return an object: %v", out)
	}
	objI := out.GetObject()
	obj := objI.(map[string]interface{})

	if len(obj) != 1 {
		t.Errorf("Flatten returned wrong number of elements: %v", out)
	}
	if obj["params.foo"] != "4" {
		t.Errorf("Flatten returned wrong value for params.foo: %v", out)
	}
}

func TestStringReplace(t *testing.T) {
	ctx := base.NewContext(logrus.StandardLogger(), "")

	o := base.MakeFreepsOperators(&OpUtils{}, nil, ctx)[0]
	args := base.NewFunctionArguments(map[string]string{
		"InputString": "%a%% + %b%% = %c%%",
		"a":           "1",
		"b":           "2",
		"c":           "3",
	})

	input := base.MakeEmptyOutput()

	// include regexp: all keys that start with "params", exclude regexp: all keys that contain "icon"
	out := o.Execute(ctx, "StringReplaceMulti", args, input)
	assert.Equal(t, out.GetString(), "1% + 2% = 3%")
}

func TestExtractWithBytes(t *testing.T) {
	ctx := base.NewContext(logrus.StandardLogger(), "")

	o := base.MakeFreepsOperators(&OpUtils{}, nil, ctx)[0]
	args := base.NewFunctionArguments(map[string]string{
		"Key":     "SomeBytes",
		"Type":    "bytesFromBase64",
		"Example": "eyJuYW1lIjoiS25vYi5qcyJ9", // ignore: this is a base64 encoded JSON object: {"name":"Knob.js"}
	})

	type testStruct struct {
		SomeString string
		SomeBytes  []byte
	}

	b, _ := json.Marshal(map[string]string{"name": "Knob.js"})
	test := testStruct{
		SomeString: "Contains another json object in SomeBytes",
		SomeBytes:  b,
	}
	input := base.MakeObjectOutput(test)

	// check if the bytes are correctly extracted
	out := o.Execute(ctx, "Extract", args, input)
	assert.Equal(t, out.GetString(), "{\"name\":\"Knob.js\"}")

	// check if the bytes are interpreted as another JSON object
	args = base.NewFunctionArguments(map[string]string{
		"Key":  "name",
		"Type": "string",
	})
	out = o.Execute(ctx, "Extract", args, out)
	assert.Equal(t, out.GetString(), "Knob.js")
}

func TestLogging(t *testing.T) {
	tdir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetOutput(os.Stdout)
	cr, err := utils.NewConfigReader(logger, path.Join(tdir, "test_config.json"))
	ctx := base.NewContext(logger, "")
	assert.NilError(t, err)
	ge := freepsgraph.NewGraphEngine(ctx, cr, func() {})
	availableOperators := []base.FreepsOperator{
		&freepsstore.OpStore{CR: cr, GE: ge},
		&OpUtils{},
	}

	for _, op := range availableOperators {
		ge.AddOperators(base.MakeFreepsOperators(op, cr, ctx))
	}
}
