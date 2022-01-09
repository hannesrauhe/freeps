package freepsmqtt

type FieldConfig struct {
	Datatype  string
	Alias     *string // name used in influx
	TrueValue *string // if datatype==bool, this is used as true value, everything else is false
}

type TopicToFluxConfig struct {
	Topic string // Topic to subscribe to
	Qos   int    // The QoS to subscribe to messages at
	// the topic string is split by slash; the values of the resulting array can be used as measurement and field - the index can be specified here
	MeasurementIndex int // index that points to the measurement in the topic-array
	FieldIndex       int // index that points to the filed in the topic-array
	Fields           map[string]FieldConfig
}

type FreepsMqttConfig struct {
	Server   string // The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883
	Username string // A username to authenticate to the MQTT server
	Password string // Password to match username
	Topics   []TopicToFluxConfig
}

var DefaultTopicConfig = TopicToFluxConfig{"shellies/shellydw2-483FDA81E731/sensor/#", 0, -1, -1, map[string]FieldConfig{}}
var DefaultConfig = FreepsMqttConfig{"tcp://localhost:1883", "", "", []TopicToFluxConfig{DefaultTopicConfig}}
