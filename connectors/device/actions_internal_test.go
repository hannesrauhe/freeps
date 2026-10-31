package device

import (
	"testing"

	"gotest.tools/v3/assert"
)

// buildArgs is tested directly because ExecuteAction swallows the operator output,
// so the raw arguments it dispatches are no longer observable from the outside.
func TestBuildArgsScaleAndValues(t *testing.T) {
	action := &Action{
		Name: "sethkrtsoll", Operator: "fritz", Function: "sethkrtsoll",
		Fixed: map[string]string{"device": "real"},
		Param: &ActionParam{Name: "temperature", Raw: "param", Type: "float", Scale: 2,
			Values: map[string]string{"off": "253"}},
	}

	built, err := action.buildArgs("21.5")
	assert.NilError(t, err)
	assert.Equal(t, built.Get("param"), "43")
	assert.Equal(t, built.Get("device"), "real")

	built, err = action.buildArgs("off")
	assert.NilError(t, err)
	assert.Equal(t, built.Get("param"), "253")

	// raw values that are not in the Values map pass through the scale
	built, err = action.buildArgs("254")
	assert.NilError(t, err)
	assert.Equal(t, built.Get("param"), "508")
}

func TestBuildArgsFixedWins(t *testing.T) {
	action := &Action{Name: "turnOn", Operator: "fritz", Function: "setswitchon",
		Fixed: map[string]string{"device": "real"}}
	built, err := action.buildArgs("")
	assert.NilError(t, err)
	assert.Equal(t, built.Get("device"), "real")
}

// an int parameter must never receive decimals from a scale factor
func TestBuildArgsIntScaleRounds(t *testing.T) {
	action := &Action{Name: "setBrightness", Operator: "pixeldisplay", Function: "SetBrightness",
		Param: &ActionParam{Name: "brightness", Type: "int", Scale: 2.55}}

	built, err := action.buildArgs("40")
	assert.NilError(t, err)
	assert.Equal(t, built.Get("brightness"), "102")

	built, err = action.buildArgs("100")
	assert.NilError(t, err)
	assert.Equal(t, built.Get("brightness"), "255")
}

func TestBuildArgsRejectsBadArgs(t *testing.T) {
	action := &Action{Name: "setBrightness", Operator: "fritz", Function: "setlevelpercentage",
		Param: &ActionParam{Name: "brightness", Type: "int", Scale: 2.55}}

	_, err := action.buildArgs("not a number")
	assert.ErrorContains(t, err, "value for action \"setBrightness\" is invalid")

	_, err = action.buildArgs("")
	assert.ErrorContains(t, err, "action \"setBrightness\" needs a brightness")

	// an action without a parameter ignores the value
	noParam := &Action{Name: "turnOn", Operator: "fritz", Function: "setswitchon"}
	built, err := noParam.buildArgs("ignored")
	assert.NilError(t, err)
	assert.Equal(t, len(built.GetOriginalKeys()), 0)
}
