package mqtt

import (
	"sync"

	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type FreepsMqttImpl struct {
	client    MQTT.Client
	Config    *FreepsMqttConfig
	ge        *freepsflow.FlowEngine
	ctx       *base.Context
	topics    map[string]bool
	topicLock sync.Mutex
}

type FreepsMqttConfig struct {
	Enabled     bool
	Server      string // The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883
	Username    string // A username to authenticate to the MQTT server
	Password    string // Password to match username
	ResultTopic string // Topic to publish results to; empty (default) means no publishing of results
}

func (fm *FreepsMqttImpl) publishResult(topic string, ctx *base.Context, out *base.OperatorIO) {
	if fm.Config.ResultTopic == "" {
		return
	}
	rt := fm.Config.ResultTopic + "/" + ctx.GetID() + "/"
	err := fm.publish(rt+"topic", topic, 0, false)
	if err != nil {
		fm.ctx.GetLogger().Errorf("Publishing freepsresult/topic failed: %v", err.Error())
	}
	// err = fm.publish(rt+"flowName", flowName, 0, false)
	// if err != nil {
	// 	fm.ctx.GetLogger().Errorf("Publishing freepsresult/flowName failed: %v", err.Error())
	// }
	err = fm.publish(rt+"type", string(out.OutputType), 0, false)
	if err != nil {
		fm.ctx.GetLogger().Errorf("Publishing freepsresult/type failed: %v", err.Error())
	}
	err = fm.publish(rt+"content", out.GetString(), 0, false)
	if err != nil {
		fm.ctx.GetLogger().Errorf("Publishing freepsresult/content failed: %v", err.Error())
	}
}

func (fm *FreepsMqttImpl) systemMessageReceived(client MQTT.Client, message MQTT.Message) {
	t := strings.Split(message.Topic(), "/")
	if len(t) <= 2 {
		fm.ctx.GetLogger().Infof("Message to topic \"%v\" ignored, expect \"freeps/<module>/<function>\"", message.Topic())
		return
	}
	input := base.MakeObjectOutput(message.Payload())
	ctx := base.CreateContextWithField(fm.ctx, "component", "mqtt", "MQTT topic: "+message.Topic())
	out := fm.ge.ExecuteOperatorByName(ctx, t[1], t[2], base.NewSingleFunctionArgument("topic", message.Topic()), input)
	fm.publishResult(message.Topic(), ctx, out)
}

func (fm *FreepsMqttImpl) startTagSubscriptions() error {
	c := fm.client
	if c == nil || !c.IsConnected() {
		return fmt.Errorf("client is not connected")
	}

	fm.topicLock.Lock()
	defer fm.topicLock.Unlock()

	tokens := []MQTT.Token{}

	newTopics := map[string]bool{}
	existingTopics := map[string]bool{}
	for _, topic := range fm.ge.GetTagValues("topic", "mqtt") {
		if topic == fm.Config.ResultTopic {
			fm.ctx.GetLogger().Errorf("Skipping subscription to result topic to prevent endless loops")
			continue
		}
		if _, ok := fm.topics[topic]; ok {
			fm.topics[topic] = true
			existingTopics[topic] = false
		} else {
			newTopics[topic] = true
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
		topic := topic // see https://go.dev/doc/faq#closures_and_goroutines
		onMessageReceived := func(client MQTT.Client, message MQTT.Message) {
			ctx := base.CreateContextWithField(fm.ctx, "component", "mqtt", "MQTT topic: "+topic)
			fm.executeTrigger(ctx, topic, message)
		}
		tokens = append(tokens, c.Subscribe(topic, byte(0), onMessageReceived))
		existingTopics[topic] = false
	}

	fm.topics = existingTopics

	errStr := ""
	for _, token := range tokens {
		token.Wait()
		if err := token.Error(); err != nil {
			fm.ctx.GetLogger().Errorf("Error when trying to subscribe/unsubscribe: %v", err)
			errStr += "* " + err.Error() + "\n"
		}
	}
	if errStr == "" {
		return nil
	}
	return fmt.Errorf("Errors during subscribe/unsubscribe:\n%v", errStr)
}

func (fm *FreepsMqttImpl) discoverTopics(ctx *base.Context, discoverDuration time.Duration) error {
	c := fm.client
	if c == nil || !c.IsConnected() {
		return fmt.Errorf("client is not connected")
	}

	fm.topicLock.Lock()
	defer fm.topicLock.Unlock()

	if _, ok := fm.topics["#"]; ok {
		return nil
	}

	ns, err := freepsstore.GetGlobalStore().GetNamespace("_mqtt")
	if err != nil {
		return fmt.Errorf("Error getting namespace: %v", err)
	}

	onMessageReceived := func(client MQTT.Client, message MQTT.Message) {
		ns.SetValue(message.Topic(), base.MakeEmptyOutput(), ctx)
	}
	token := c.Subscribe("#", byte(0), onMessageReceived)
	token.Wait()
	if err := token.Error(); err != nil {
		fm.ctx.GetLogger().Errorf("Error when trying to disover new topics: %v", err)
	}

	time.Sleep(discoverDuration)

	token = c.Unsubscribe("#")
	token.Wait()
	if err := token.Error(); err != nil {
		fm.ctx.GetLogger().Errorf("Error when trying to disover new topics: %v", err)
	}

	return nil
}

func (fm *FreepsMqttImpl) startConfigSubscriptions(c MQTT.Client) {
	if token := c.Subscribe("freeps/#", 0, fm.systemMessageReceived); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fm.startTagSubscriptions()
}

func newFreepsMqttImpl(ctx *base.Context, fmc *FreepsMqttConfig, ge *freepsflow.FlowEngine) (*FreepsMqttImpl, error) {
	if fmc.Server == "" {
		return nil, fmt.Errorf("no server given in the config file")
	}

	hostname, _ := os.Hostname()
	clientid := hostname + strconv.Itoa(time.Now().Second())
	fmqtt := &FreepsMqttImpl{Config: fmc, ge: ge, ctx: ctx, topics: map[string]bool{}}

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
	fmqtt.client = client
	return fmqtt, nil
}

func (fm *FreepsMqttImpl) publish(topic string, msg interface{}, qos int64, retain bool) error {
	if fm.client == nil {
		return fmt.Errorf("MQTT client is uninitialized")
	}

	token := fm.client.Publish(topic, byte(qos), retain, msg)
	token.Wait()
	return token.Error()
}

func (fm *FreepsMqttImpl) getTopicSubscriptions() []string {
	fm.topicLock.Lock()
	defer fm.topicLock.Unlock()

	topics := make([]string, 0, len(fm.topics))
	for t := range fm.topics {
		topics = append(topics, t)
	}
	return topics
}

func (fm *FreepsMqttImpl) StartListening() error {
	go func() {
		if token := fm.client.Connect(); token.Wait() && token.Error() != nil {
			fm.ctx.GetLogger().Errorf("Error when connecting to %s: %v \n", fm.Config.Server, token.Error().Error())
		} else {
			fm.ctx.GetLogger().Infof("Connected to %s, starting to subscribe", fm.Config.Server)
		}
	}()
	return nil
}

// Shutdown MQTT and cancel all subscriptions
func (fm *FreepsMqttImpl) Shutdown() {
	if fm.client == nil {
		return
	}
	fm.client.Disconnect(100)
	fm.client = nil
}
