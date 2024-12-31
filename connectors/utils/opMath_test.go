package freepsutils

import (
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestDiv(t *testing.T) {
	ctx, _ := base.NewBaseContext(logrus.StandardLogger())

	o := base.MakeFreepsOperators(&OpMath{}, nil, ctx)[0]
	args := base.NewFunctionArguments(map[string]string{
		"left":  "39",
		"right": "2",
	})

	input := base.MakeEmptyOutput()

	out := o.Execute(ctx, "DivideFloat", args, input)
	assert.Equal(t, out.GetString(), "19.5")
}
