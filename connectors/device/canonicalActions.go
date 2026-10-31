package device

// Canonical action names shared by all operators, so a client can control any device without
// knowing the operator behind it. The parameters are part of the contract:
//
//	turnOn, turnOff, toggle  no parameters
//	setBrightness            brightness: int, percent 0-100
//	setColor                 color: hex string, e.g. #ff0000
//	setColorTemperature      temperature: int, Kelvin
//	setTargetTemperature     temperature: float, °C, with off/comfort/eco labels where the device has them
//	setTargetHumidity        humidity: int, percent
const (
	ActionTurnOn               = "turnOn"
	ActionTurnOff              = "turnOff"
	ActionToggle               = "toggle"
	ActionSetBrightness        = "setBrightness"
	ActionSetColor             = "setColor"
	ActionSetColorTemperature  = "setColorTemperature"
	ActionSetTargetTemperature = "setTargetTemperature"
	ActionSetTargetHumidity    = "setTargetHumidity"
)
