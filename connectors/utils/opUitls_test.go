package freepsgraph

import (
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
)

func TestFlattenWithRegexp(t *testing.T) {
	o := &OpUtils{}
	ctx := base.NewContext(logrus.StandardLogger())
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
