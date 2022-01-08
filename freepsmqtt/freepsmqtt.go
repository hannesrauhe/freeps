package freepsmqtt

type FreepsMqttConfig struct {
	server   string // := flag.String("server", "tcp://raspi:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	topic    string // := flag.String("topic", "shellies/shellydw2-483FDA81E731/sensor/#", "Topic to subscribe to")
	qos      int    // := flag.Int("qos", 0, "The QoS to subscribe to messages at")
	username string //:= flag.String("username", "", "A username to authenticate to the MQTT server")
	password string //:= flag.String("password", "", "Password to match username")
}

var DefaultConfig = FreepsMqttConfig{"tcp://raspi:1883", "shellies/shellydw2-483FDA81E731/sensor/#", 0, "", ""}
