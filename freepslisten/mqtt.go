package freepslisten

import (
	"log"

	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/utils"

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

func mqttReceivedMessage(tc TopicToFluxConfig, client MQTT.Client, message MQTT.Message, callback func(string, map[string]string, map[string]interface{}) error) {
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
		callback(t[tc.MeasurementIndex], map[string]string{}, map[string]interface{}{fieldAlias: value})

	} else {
		fmt.Printf("#Measuremnt: %s, Field: %s, Value: %s\n", t[tc.MeasurementIndex], field, message.Payload())
	}
}

type FreepsMqtt struct {
	client   MQTT.Client
	Config   *FreepsMqttConfig
	Callback func(string, map[string]string, map[string]interface{}) error
}

func (fm *FreepsMqtt) Shutdown() {
	fm.client.Disconnect(100)
}

func NewMqttSubscriber(cr *utils.ConfigReader) *FreepsMqtt {
	ffc := freepsflux.DefaultConfig
	fmc := DefaultConfig
	err := cr.ReadSectionWithDefaults("freepsflux", &ffc)
	if err != nil {
		log.Fatal(err)
	}
	err = cr.ReadSectionWithDefaults("freepsmqtt", &fmc)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}

	ff, err2 := freepsflux.NewFreepsFlux(&ffc, nil)
	if err2 != nil {
		log.Fatalf("Error while executing function: %v\n", err2)
	}

	hostname, _ := os.Hostname()
	clientid := hostname + strconv.Itoa(time.Now().Second())

	connOpts := MQTT.NewClientOptions().AddBroker(fmc.Server).SetClientID(clientid).SetCleanSession(true)
	if fmc.Username != "" {
		connOpts.SetUsername(fmc.Username)
		if fmc.Password != "" {
			connOpts.SetPassword(fmc.Password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)

	connOpts.OnConnect = func(c MQTT.Client) {
		for _, k := range fmc.Topics {
			onMessageReceived := func(client MQTT.Client, message MQTT.Message) {
				mqttReceivedMessage(k, client, message, ff.PushFields)
			}
			if token := c.Subscribe(k.Topic, byte(k.Qos), onMessageReceived); token.Wait() && token.Error() != nil {
				panic(token.Error())
			}
		}
	}

	client := MQTT.NewClient(connOpts)
	go func() {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			panic(token.Error())
		} else {
			fmt.Printf("Connected to %s\n", fmc.Server)
		}
	}()

	return &FreepsMqtt{client: client, Config: &fmc, Callback: ff.PushFields}
}
