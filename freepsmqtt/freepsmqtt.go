package freepsmqtt

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

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
var DefaultConfig = FreepsMqttConfig{"", "", "", []TopicToFluxConfig{DefaultTopicConfig}}

type FreepsMqtt struct {
	Config   *FreepsMqttConfig
	Callback func(string, map[string]interface{}) error
}

func mqttReceivedMessage(tc TopicToFluxConfig, client MQTT.Client, message MQTT.Message, callback func(string, map[string]interface{}) error) {
	var err error
	t := strings.Split(message.Topic(), "/")
	field := t[tc.FieldIndex]
	fconf, fieldExists := tc.Fields[field]
	if fieldExists {
		var value interface{}
		fieldAlias := field
		if fconf.Alias != nil {
			fieldAlias = *fconf.Alias
		}
		switch fconf.Datatype {
		case "float":
			value, err = strconv.ParseFloat(string(message.Payload()), 64)
		case "int":
			value, err = strconv.Atoi(string(message.Payload()))
		case "bool":
			if fconf.TrueValue == nil {
				value, err = strconv.ParseBool(string(message.Payload()))
			} else if string(message.Payload()) == *fconf.TrueValue {
				value = true
			} else {
				value = false
			}
		default:
			value = string(message.Payload())
		}
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s %s=%v\n", t[tc.MeasurementIndex], fieldAlias, value)
		callback(t[tc.MeasurementIndex], map[string]interface{}{fieldAlias: value})

	} else {
		fmt.Printf("#Measuremnt: %s, Field: %s, Value: %s\n", t[tc.MeasurementIndex], field, message.Payload())
	}
}

func (fm *FreepsMqtt) Start() {
	if fm.Config.Server == "" {
		return
	}

	hostname, _ := os.Hostname()
	clientid := hostname + strconv.Itoa(time.Now().Second())

	connOpts := MQTT.NewClientOptions().AddBroker(fm.Config.Server).SetClientID(clientid).SetCleanSession(true)
	if fm.Config.Username != "" {
		connOpts.SetUsername(fm.Config.Username)
		if fm.Config.Password != "" {
			connOpts.SetPassword(fm.Config.Password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)

	connOpts.OnConnect = func(c MQTT.Client) {
		for _, k := range fm.Config.Topics {
			onMessageReceived := func(client MQTT.Client, message MQTT.Message) {
				mqttReceivedMessage(k, client, message, fm.Callback)
			}
			if token := c.Subscribe(k.Topic, byte(k.Qos), onMessageReceived); token.Wait() && token.Error() != nil {
				panic(token.Error())
			}
		}
	}

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to %s\n", fm.Config.Server)
	}
}
