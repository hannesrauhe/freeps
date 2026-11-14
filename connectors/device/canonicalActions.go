package device

// Canonical action names shared by all operators, so a client can control any device without
// knowing the operator behind it. The parameters are part of the contract
const (
	ActionTurnOn               = "turnOn"
	ActionTurnOff              = "turnOff"
	ActionToggle               = "toggle"
	ActionSetBrightness        = "setBrightness"
	ActionSetColor             = "setColor"             // expected Parameters: color: hex string, e.g. #ff0000
	ActionSetColorTemperature  = "setColorTemperature"  // expected Parameters: temperature: int, Kelvin
	ActionSetTargetTemperature = "setTargetTemperature" // expected Parameters: temperature: float, °C
	ActionSetTargetHumidity    = "setTargetHumidity"    // expected Parameters: humidity: int, percent
)
