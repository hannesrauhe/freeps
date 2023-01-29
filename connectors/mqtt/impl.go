package mqtt

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

type FreepsMqttImpl struct {
	client     MQTT.Client
	Config     *FreepsMqttConfig
	ge         *freepsgraph.GraphEngine
	mqttlogger log.FieldLogger
	topics     map[string]bool
}

type FieldConfig struct {
	Datatype  string
	Alias     *string // name used in influx
	TrueValue *string // if datatype==bool, this is used as true value, everything else is false
}

type TopicConfig struct {
	Topic string // Topic to subscribe to
	Qos   int    // The QoS to subscribe to messages at
	// the topic string is split by slash; the values of the resulting array can be used as measurement and field - the index can be specified here
	MeasurementIndex int                    // index that points to the measurement in the topic-array
	FieldIndex       int                    // index that points to the field in the topic-array
	Fields           map[string]FieldConfig `json:",omitempty"`
	TemplateToCall   string                 `json:",omitempty"`
	GraphToCall      string
}

type FreepsMqttConfig struct {
	Server      string // The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883
	Username    string // A username to authenticate to the MQTT server
	Password    string // Password to match username
	Topics      []TopicConfig
	ResultTopic string // Topic to publish results to; empty (default) means no publishing of results
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

func (fm *FreepsMqttImpl) publishResult(topic string, ctx *utils.Context, out *freepsgraph.OperatorIO) {
	if fm.Config.ResultTopic == "" {
		return
	}
	rt := fm.Config.ResultTopic + "/" + ctx.GetID() + "/"
	err := fm.publish(rt+"topic", topic, 0, false)
	if err != nil {
		fm.mqttlogger.Errorf("Publishing freepsresult/topic failed: %v", err.Error())
	}
	// err = fm.publish(rt+"graphName", graphName, 0, false)
	// if err != nil {
	// 	fm.mqttlogger.Errorf("Publishing freepsresult/graphName failed: %v", err.Error())
	// }
	err = fm.publish(rt+"type", string(out.OutputType), 0, false)
	if err != nil {
		fm.mqttlogger.Errorf("Publishing freepsresult/type failed: %v", err.Error())
	}
	err = fm.publish(rt+"content", out.GetString(), 0, false)
	if err != nil {
		fm.mqttlogger.Errorf("Publishing freepsresult/content failed: %v", err.Error())
	}
}

func (fm *FreepsMqttImpl) systemMessageReceived(client MQTT.Client, message MQTT.Message) {
	t := strings.Split(message.Topic(), "/")
	if len(t) <= 2 {
		log.Printf("Message to topic \"%v\" ignored, expect \"freeps/<module>/<function>\"", message.Topic())
		return
	}
	input := freepsgraph.MakeObjectOutput(message.Payload())
	ctx := utils.NewContext(fm.mqttlogger)
	out := fm.ge.ExecuteOperatorByName(ctx, t[1], t[2], map[string]string{"topic": message.Topic()}, input)
	fm.publishResult(message.Topic(), ctx, out)
}

func (fm *FreepsMqttImpl) starTagSubscriptions() error {
	c := fm.client
	if c == nil || !c.IsConnected() {
		return fmt.Errorf("client is not connected")
	}

	tokens := []MQTT.Token{}

	newTopics := map[string]bool{}
	existingTopics := map[string]bool{}
	for _, info := range fm.ge.GetGraphInfoByTag([]string{"mqtt"}) {
		for _, t := range info.Desc.Tags {
			if len(t) > len("topic:") && t[:6] == "topic:" {
				topic := t[6:]
				if _, ok := fm.topics[topic]; ok {
					fm.topics[topic] = true
					existingTopics[topic] = false
				} else {
					newTopics[topic] = true
				}
			}
		}
	}

	// unsubscribe unused topics first
	unsubTopics := []string{}
	for t, s := range fm.topics {
		if !s {
			unsubTopics = append(unsubTopics, t)
		}
	}
	if len(unsubTopics) > 0 {
		tokens = append(tokens, c.Unsubscribe(unsubTopics...))
	}

	// subscribe to new Topics
	for topic := range newTopics {
		// build the slice here so we don't run into https://go.dev/doc/faq#closures_and_goroutines
		tags := []string{"mqtt", "topic:" + topic}
		onMessageReceived := func(client MQTT.Client, message MQTT.Message) {
			ctx := utils.NewContext(fm.mqttlogger)
			input := freepsgraph.MakeByteOutput(message.Payload())
			out := fm.ge.ExecuteGraphByTags(ctx, tags, map[string]string{"topic": message.Topic(), "subscription": tags[1]}, input)
			fm.publishResult(topic, ctx, out)
		}
		tokens = append(tokens, c.Subscribe(topic, byte(0), onMessageReceived))
		existingTopics[topic] = false
	}

	fm.topics = existingTopics

	errStr := ""
	for _, token := range tokens {
		token.Wait()
		if err := token.Error(); err != nil {
			fmt.Println(err)
			errStr += "* " + err.Error() + "\n"
		}
	}
	if errStr == "" {
		return nil
	}
	return fmt.Errorf("Errors during subscribe/unsubscribe:\n%v", errStr)
}

func (fm *FreepsMqttImpl) startConfigSubscriptions(c MQTT.Client) {
	for _, k := range fm.Config.Topics {
		k := k // see https://go.dev/doc/faq#closures_and_goroutines
		onMessageReceived := func(client MQTT.Client, message MQTT.Message) {
			fm.processMessage(k, message.Payload(), message.Topic())
		}
		if token := c.Subscribe(k.Topic, byte(k.Qos), onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}
	if token := c.Subscribe("freeps/#", 0, fm.systemMessageReceived); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
}

func newFreepsMqttImpl(logger log.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) (*FreepsMqttImpl, error) {
	mqttlogger := logger.WithField("component", "mqtt")
	fmc := DefaultConfig
	err := cr.ReadSectionWithDefaults("freepsmqtt", &fmc)
	if err != nil {
		return nil, fmt.Errorf("Reading the config failed: %v", err.Error())
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}

	if fmc.Server == "" {
		return nil, fmt.Errorf("no server given in the config file")
	}

	hostname, _ := os.Hostname()
	clientid := hostname + strconv.Itoa(time.Now().Second())
	fmqtt := &FreepsMqttImpl{Config: &fmc, ge: ge, mqttlogger: mqttlogger, topics: map[string]bool{}}

	connOpts := MQTT.NewClientOptions().AddBroker(fmc.Server).SetClientID(clientid).SetCleanSession(true).SetOrderMatters(false)
	if fmc.Username != "" {
		connOpts.SetUsername(fmc.Username)
		if fmc.Password != "" {
			connOpts.SetPassword(fmc.Password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)
	connOpts.SetCleanSession(true)
	connOpts.OnConnect = fmqtt.startConfigSubscriptions

	client := MQTT.NewClient(connOpts)
	go func() {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			panic(token.Error())
		} else {
			fmt.Printf("Connected to %s\n", fmc.Server)
		}
	}()
	fmqtt.client = client
	return fmqtt, nil
}

func (fm *FreepsMqttImpl) publish(topic string, msg interface{}, qos int, retain bool) error {
	if fm.client == nil {
		return fmt.Errorf("MQTT client is uninitialized")
	}

	token := fm.client.Publish(topic, byte(qos), retain, msg)
	token.Wait()
	return token.Error()
}

// Shutdown MQTT and cancel all subscriptions
func (fm *FreepsMqttImpl) Shutdown() {
	if fm.client == nil {
		return
	}
	fm.client.Disconnect(100)
	fm.client = nil
}
