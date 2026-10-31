package pixeldisplay

import (
	"image"
	"image/color"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
	"github.com/hannesrauhe/freeps/freepsd/helper"
	"gotest.tools/v3/assert"
)

func TestActionsForDisplay(t *testing.T) {
	actions := actionsForDisplay("pixeldisplay")
	byName := map[string]*device.Action{}
	for _, a := range actions {
		byName[a.Name] = a
	}
	assert.Equal(t, len(byName), 5)

	brightness := byName["setBrightness"]
	assert.Assert(t, brightness != nil)
	assert.Equal(t, brightness.Operator, "pixeldisplay")
	assert.Equal(t, brightness.Function, "SetBrightness")
	assert.Equal(t, len(brightness.Params), 1)
	assert.Equal(t, brightness.Params[0].Name, "brightness")
	// the canonical contract is percent, WLED takes 0-255
	assert.Equal(t, brightness.Params[0].Scale, 2.55)

	assert.Equal(t, byName["turnOn"].Function, "TurnOn")
	assert.Equal(t, len(byName["turnOn"].Params), 0)
}

func TestDeviceName(t *testing.T) {
	// the operator name is derived from the struct, so it is capitalized
	assert.Equal(t, (&OpPixelDisplay{name: "PixelDisplay"}).deviceName(), "Default")
	assert.Equal(t, (&OpPixelDisplay{name: "pixeldisplay"}).deviceName(), "Default")
	assert.Equal(t, (&OpPixelDisplay{name: "PixelDisplay.albert"}).deviceName(), "albert")
	assert.Equal(t, (&OpPixelDisplay{name: "pixeldisplay.matrix"}).deviceName(), "matrix")
	assert.Equal(t, (&OpPixelDisplay{name: "pixeldisplay.a.b"}).deviceName(), "a_b")
}

func TestRegisterDevice(t *testing.T) {
	ctx, _, _ := helper.SetupEngineWithCommonOperators(t, nil)
	opSensor := device.GetGlobalSensors()

	op := &OpPixelDisplay{name: "pixeldisplay"}
	op.StartListening(ctx)

	res := opSensor.GetActions(ctx, base.MakeEmptyOutput(), device.DeviceArgs{DeviceCategory: "pixeldisplay", DeviceName: "Default"})
	assert.Assert(t, !res.IsError(), "%v", res.GetError())
	actions := map[string]device.ActionInfo{}
	assert.NilError(t, res.ParseJSON(&actions))
	assert.Equal(t, len(actions), 5)
	_, hasBrightness := actions["setBrightness"]
	assert.Assert(t, hasBrightness)

	alias := opSensor.GetSensorAlias(ctx, base.MakeEmptyOutput(), device.SensorArgs{SensorCategory: "pixeldisplay", SensorName: "Default"})
	assert.Equal(t, alias.GetString(), "Default")
}

// fakeDisplay records nothing, it only returns what the operator functions mirror into the sensor.
type fakeDisplay struct {
	on         bool
	brightness int
}

func (f *fakeDisplay) TurnOn() *base.OperatorIO  { f.on = true; return base.MakeEmptyOutput() }
func (f *fakeDisplay) TurnOff() *base.OperatorIO { f.on = false; return base.MakeEmptyOutput() }
func (f *fakeDisplay) SetBrightness(b int) *base.OperatorIO {
	f.brightness = b
	return base.MakeEmptyOutput()
}
func (f *fakeDisplay) SetEffect(fx int) *base.OperatorIO { return base.MakeEmptyOutput() }
func (f *fakeDisplay) DrawImage(ctx *base.Context, i image.Image, r bool) *base.OperatorIO {
	return base.MakeEmptyOutput()
}
func (f *fakeDisplay) SetBackgroundLayer(ctx *base.Context, i image.Image, n string) {}
func (f *fakeDisplay) ResetBackground(ctx *base.Context)                             {}
func (f *fakeDisplay) GetBackgroundLayerNames() []string                             { return nil }
func (f *fakeDisplay) SetColor(c color.Color)                                        {}
func (f *fakeDisplay) SetBackgroundColor(c color.Color)                              {}
func (f *fakeDisplay) GetMaxPictureSize() image.Point                                { return image.Point{X: 32, Y: 16} }
func (f *fakeDisplay) GetDimensions() image.Point                                    { return image.Point{X: 16, Y: 16} }
func (f *fakeDisplay) GetColor() color.Color                                         { return color.RGBA{R: 0xff, A: 0xff} }
func (f *fakeDisplay) GetBackgroundColor() color.Color                               { return color.RGBA{A: 0xff} }
func (f *fakeDisplay) GetBrightness() int                                            { return f.brightness }
func (f *fakeDisplay) IsOn() bool                                                    { return f.on }
func (f *fakeDisplay) Shutdown(ctx *base.Context)                                    {}

func TestSensorUpdatedByCommands(t *testing.T) {
	ctx, _, _ := helper.SetupEngineWithCommonOperators(t, nil)
	opSensor := device.GetGlobalSensors()

	fake := &fakeDisplay{}
	op := &OpPixelDisplay{name: "pixeldisplay", display: fake, t2p: NewText2Pixeldisplay(fake)}
	op.StartListening(ctx)

	prop := func(name string) string {
		return opSensor.GetSensorPropertyInternal(ctx, "pixeldisplay", "Default", name).GetString()
	}

	op.TurnOn(ctx, base.MakeEmptyOutput())
	assert.Equal(t, prop("isOn"), "true")
	op.TurnOff(ctx, base.MakeEmptyOutput())
	assert.Equal(t, prop("isOn"), "false")

	op.SetBrightness(ctx, base.MakeEmptyOutput(), BrightnessArgs{Brightness: 42})
	// the device counts 0-255, the property is a percentage like the setBrightness action
	assert.Equal(t, prop("brightness"), "16")

	op.SetColor(ctx, base.MakeEmptyOutput(), ColorArgs{Color: "#00ff00"})
	assert.Equal(t, prop("color"), "#00ff00")

	op.SetBackgroundColor(ctx, base.MakeEmptyOutput(), ColorArgs{Color: "#0000ff"})
	assert.Equal(t, prop("backgroundColor"), "#0000ff")

	op.SetEffect(ctx, base.MakeEmptyOutput(), EffectArgs{Fx: 7})
	assert.Equal(t, prop("effect"), "7")

	op.GetDimensions(ctx, base.MakeEmptyOutput())
	assert.Equal(t, prop("width"), "16")
	assert.Equal(t, prop("height"), "16")

	op.GetMaxPictureSize(ctx, base.MakeEmptyOutput())
	assert.Equal(t, prop("maxWidth"), "32")

	op.GetColor(ctx, base.MakeEmptyOutput())
	assert.Equal(t, prop("color"), "#ff0000")

	op.GetBrightness(ctx, base.MakeEmptyOutput())
	assert.Equal(t, prop("brightness"), "16")

	op.GetBackgroundColor(ctx, base.MakeEmptyOutput())
	assert.Equal(t, prop("backgroundColor"), "#000000")

	op.IsOn(ctx, base.MakeEmptyOutput())
	assert.Equal(t, prop("isOn"), "false")

	text := "Hello"
	op.DrawText(ctx, base.MakeEmptyOutput(), TextArgs{Text: &text})
	assert.Equal(t, prop("text"), "Hello")
}

// A failed command must not write a property, the sensor keeps the last known value.
func TestSensorNotUpdatedOnError(t *testing.T) {
	ctx, _, _ := helper.SetupEngineWithCommonOperators(t, nil)
	opSensor := device.GetGlobalSensors()

	op := &OpPixelDisplay{name: "pixeldisplay", display: &fakeDisplay{}}
	op.StartListening(ctx)

	op.SetBrightness(ctx, base.MakeEmptyOutput(), BrightnessArgs{Brightness: 42})
	op.SetColor(ctx, base.MakeEmptyOutput(), ColorArgs{Color: "not a color"})
	assert.Equal(t, opSensor.GetSensorPropertyInternal(ctx, "pixeldisplay", "Default", "color").IsError(), true)
	assert.Equal(t, opSensor.GetSensorPropertyInternal(ctx, "pixeldisplay", "Default", "brightness").GetString(), "16")
}
