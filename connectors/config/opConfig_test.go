package opconfig

import (
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestGetKey(t *testing.T) {
	ctx := base.NewBaseContextWithReason(logrus.StandardLogger(), "")
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	oc := &OpConfig{CR: cr}
	ops := base.MakeFreepsOperators(oc, cr, ctx)
	op := ops[0]

	assert.NilError(t, cr.WriteSection("mistral", map[string]interface{}{
		"APIKey":  "secret123",
		"Model":   "voxtral",
		"Retries": 3,
	}, true))

	args := func(section, key string) base.FunctionArguments {
		return base.NewFunctionArgumentsFromURLValues(map[string][]string{
			"sectionname": {section}, "key": {key},
		})
	}

	res := op.Execute(ctx, "GetKey", args("mistral", "APIKey"), base.MakeEmptyOutput())
	assert.Assert(t, !res.IsError())
	b, err := res.GetBytes()
	assert.NilError(t, err)
	assert.Equal(t, string(b), "secret123")

	// keys are matched case-insensitively
	res = op.Execute(ctx, "GetKey", args("mistral", "apikey"), base.MakeEmptyOutput())
	assert.Assert(t, !res.IsError())

	// non-string values are returned too
	res = op.Execute(ctx, "GetKey", args("mistral", "Retries"), base.MakeEmptyOutput())
	assert.Assert(t, !res.IsError())

	// unknown key and unknown section give 404
	res = op.Execute(ctx, "GetKey", args("mistral", "nope"), base.MakeEmptyOutput())
	assert.Assert(t, res.IsError())
	assert.Equal(t, res.HTTPCode, 404)

	res = op.Execute(ctx, "GetKey", args("nosuchsection", "APIKey"), base.MakeEmptyOutput())
	assert.Assert(t, res.IsError())
	assert.Equal(t, res.HTTPCode, 404)

	// suggestions list the section's keys
	sug := op.GetArgSuggestions("GetKey", "Key", args("mistral", ""))
	assert.Equal(t, len(sug), 3)
	_, ok := sug["APIKey"]
	assert.Assert(t, ok)

	// the operator-level section name suggestion applies to all functions with a SectionName argument
	for _, fn := range []string{"GetKey", "GetSection", "RemoveSection", "SetKey"} {
		sug = op.GetArgSuggestions(fn, "SectionName", base.MakeEmptyFunctionArguments())
		_, ok = sug["mistral"]
		assert.Assert(t, ok, "no section suggestion for %v", fn)
	}

	// SetKey replaces an existing key and keeps the others
	res = op.Execute(ctx, "SetKey", base.NewFunctionArgumentsFromURLValues(map[string][]string{
		"sectionname": {"mistral"}, "key": {"Model"}, "value": {"newmodel"},
	}), base.MakeEmptyOutput())
	assert.Assert(t, !res.IsError())
	res = op.Execute(ctx, "GetKey", args("mistral", "Model"), base.MakeEmptyOutput())
	got, err := res.GetBytes()
	assert.NilError(t, err)
	assert.Equal(t, string(got), "newmodel")
	res = op.Execute(ctx, "GetKey", args("mistral", "APIKey"), base.MakeEmptyOutput())
	assert.Assert(t, !res.IsError())

	// SetKey creates a missing section
	res = op.Execute(ctx, "SetKey", base.NewFunctionArgumentsFromURLValues(map[string][]string{
		"sectionname": {"newsection"}, "key": {"Foo"}, "value": {"bar"},
	}), base.MakeEmptyOutput())
	assert.Assert(t, !res.IsError())
	res = op.Execute(ctx, "GetKey", args("newsection", "Foo"), base.MakeEmptyOutput())
	got, err = res.GetBytes()
	assert.NilError(t, err)
	assert.Equal(t, string(got), "bar")
}
