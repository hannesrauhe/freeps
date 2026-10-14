package sensor

import (
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"gotest.tools/v3/assert"
)

// buildArgs is tested directly because ExecuteAction swallows the operator output,
// so the raw arguments it dispatches are no longer observable from the outside.
func TestBuildArgsScaleAndValues(t *testing.T) {
	action := &Action{
		Name: "sethkrtsoll", Operator: "fritz", Function: "sethkrtsoll",
		Fixed: map[string]string{"device": "real"},
		Params: []ActionParam{{Name: "temperature", Raw: "param", Type: "float", Scale: 2,
			Values: map[string]string{"off": "253"}}},
	}

	built, err := action.buildArgs(base.NewFunctionArguments(map[string]string{"temperature": "21.5"}))
	assert.NilError(t, err)
	assert.Equal(t, built.Get("param"), "43")
	assert.Equal(t, built.Get("device"), "real")

	built, err = action.buildArgs(base.NewFunctionArguments(map[string]string{"temperature": "off"}))
	assert.NilError(t, err)
	assert.Equal(t, built.Get("param"), "253")

	// raw values that are not in the Values map pass through the scale
	built, err = action.buildArgs(base.NewFunctionArguments(map[string]string{"temperature": "254"}))
	assert.NilError(t, err)
	assert.Equal(t, built.Get("param"), "508")
}

func TestBuildArgsFixedWins(t *testing.T) {
	action := &Action{Name: "turnOn", Operator: "fritz", Function: "setswitchon",
		Fixed: map[string]string{"device": "real"}}
	built, err := action.buildArgs(base.NewFunctionArguments(map[string]string{"device": "evil"}))
	assert.NilError(t, err)
	assert.Equal(t, built.Get("device"), "real")
}

func TestBuildArgsRejectsBadArgs(t *testing.T) {
	action := &Action{Name: "setLevel", Operator: "fritz", Function: "setlevelpercentage",
		Params: []ActionParam{{Name: "level", Type: "int"}}}

	_, err := action.buildArgs(base.NewFunctionArguments(map[string]string{"level": "50", "extra": "1"}))
	assert.ErrorContains(t, err, "unknown parameter \"extra\"")

	_, err = action.buildArgs(base.NewFunctionArguments(map[string]string{}))
	assert.ErrorContains(t, err, "parameter \"level\" is missing")

	// an optional parameter may be left out
	action.Params = []ActionParam{{Name: "onoff", Type: "string", Optional: true}}
	built, err := action.buildArgs(base.NewFunctionArguments(map[string]string{}))
	assert.NilError(t, err)
	assert.Assert(t, !built.Has("onoff"))
}
