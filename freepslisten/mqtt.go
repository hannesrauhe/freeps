package freepslisten

import (
	log "github.com/sirupsen/logrus"

	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type FieldConfig struct {
	Datatype  string
	Alias     *string // name used in influx
	TrueValue *string // if datatype==bool, this is used as true value, everything else is false
}

type TopicConfig struct {
	Topic string // Topic to subscribe to
	Qos   int    // The QoS to subscribe to messages at
	// the topic string is split by slash; the values of the resulting array can be used as measurement and field - the index can be specified here
	MeasurementIndex int // index that points to the measurement in the topic-array
	FieldIndex       int // index that points to the field in the topic-array
	Fields           map[string]FieldConfig
	TemplateToCall   string
}

type FreepsMqttConfig struct {
	Server   string // The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883
	Username string // A username to authenticate to the MQTT server
	Password string // Password to match username
	Topics   []TopicConfig
}

var DefaultTopicConfig = TopicConfig{Topic: "#", Qos: 0, MeasurementIndex: -1, FieldIndex: -1, Fields: map[string]FieldConfig{}, TemplateToCall: "mqttaction"}
var DefaultConfig = FreepsMqttConfig{Server: "", Username: "", Password: "", Topics: []TopicConfig{DefaultTopicConfig}}

type FieldWithType struct {
	FieldType  string
	FieldValue string
}
type JsonArgs struct {
	Measurement    string
	FieldsWithType map[string]FieldWithType
}

type FreepsMqtt struct {
	client     MQTT.Client
	Config     *FreepsMqttConfig
	ge         *freepsgraph.GraphEngine
	Callback   func(string, map[string]string, map[string]interface{}) error
	mqttlogger log.FieldLogger
}

func (fm *FreepsMqtt) processMessage(tc TopicConfig, message []byte, topic string) {
	t := strings.Split(topic, "/")
	field := t[tc.FieldIndex]
	fconf, fieldExists := tc.Fields[field]
	value := string(message)
	if fieldExists {
		fieldAlias := field
		if fconf.Alias != nil {
			fieldAlias = *fconf.Alias
		}
		if fconf.TrueValue != nil {
			if value == *fconf.TrueValue {
				value = "true"
			} else {
				value = "false"
			}
		}

		fwt := FieldWithType{fconf.Datatype, value}
		args := JsonArgs{Measurement: t[tc.MeasurementIndex], FieldsWithType: map[string]FieldWithType{fieldAlias: fwt}}

		input := freepsgraph.MakeObjectOutput(args)
		output := fm.ge.ExecuteGraph(tc.TemplateToCall, map[string]string{"topic": topic}, input)
		output.Log(fm.mqttlogger.WithFields(log.Fields{"topic": topic, "measurement": t[tc.MeasurementIndex], "field": field, "value": value}))
	} else {
		fm.mqttlogger.WithFields(log.Fields{"topic": topic, "measurement": t[tc.MeasurementIndex], "field": field, "value": value}).Info("No field config found")
	}
}

func (fm *FreepsMqtt) configuredMessageReceived(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()

	// lazily trying to match topics -- assuming topic always end with "#"
	// TODO(HR): figure out better matching or how to pass multiple handlers
	for _, k := range fm.Config.Topics {
		prefixLen := len(k.Topic) - 1
		if len(topic) >= prefixLen && k.Topic[:prefixLen] == topic[:prefixLen] {
			fm.processMessage(k, message.Payload(), topic)
		}
	}
}

func (fm *FreepsMqtt) systemMessageReceived(client MQTT.Client, message MQTT.Message) {
	t := strings.Split(message.Topic(), "/")
	if len(t) <= 2 {
		log.Printf("Message to topic \"%v\" ignored, expect \"freeps/<module>/<function>\"", message.Topic())
		return
	}
	input := freepsgraph.MakeObjectOutput(message.Payload())
	output := fm.ge.ExecuteOperatorByName(fm.mqttlogger, t[1], t[2], map[string]string{"topic": message.Topic()}, input)
	output.WriteTo(os.Stdout)
}

func (fm *FreepsMqtt) Shutdown() {
	fm.client.Disconnect(100)
}

func NewMqttSubscriber(logger log.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) *FreepsMqtt {
	mqttlogger := logger.WithField("component", "mqtt")
	fmc := DefaultConfig
	err := cr.ReadSectionWithDefaults("freepsmqtt", &fmc)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}

	if fmc.Server == "" {
		return nil
	}

	hostname, _ := os.Hostname()
	clientid := hostname + strconv.Itoa(time.Now().Second())
	fmqtt := &FreepsMqtt{Config: &fmc, ge: ge, mqttlogger: mqttlogger}

	connOpts := MQTT.NewClientOptions().AddBroker(fmc.Server).SetClientID(clientid).SetCleanSession(true)
	if fmc.Username != "" {
		connOpts.SetUsername(fmc.Username)
		if fmc.Password != "" {
			connOpts.SetPassword(fmc.Password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)
	connOpts.SetCleanSession(true)
	connOpts.OnConnect = func(c MQTT.Client) {
		for _, k := range fmc.Topics {
			// giving a separate callback for each topic in this loop will cause the library to always call the last
			// I'm doing something wrong here
			if token := c.Subscribe(k.Topic, byte(k.Qos), fmqtt.configuredMessageReceived); token.Wait() && token.Error() != nil {
				panic(token.Error())
			}
		}
		if token := c.Subscribe("freeps/#", 0, fmqtt.systemMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
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
	fmqtt.client = client
	return fmqtt
}
