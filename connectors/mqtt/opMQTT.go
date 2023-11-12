package mqtt

import (
	"crypto/tls"
	"net/http"
	"os"
	"strconv"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type OpMQTT struct {
	CR   *utils.ConfigReader
	GE   *freepsgraph.GraphEngine
	impl *FreepsMqttImpl
}

var _ base.FreepsOperatorWithDynamicFunctions = &OpMQTT{}
var _ base.FreepsOperatorWithConfig = &OpMQTT{}
var _ base.FreepsOperatorWithHook = &OpMQTT{}
var _ base.FreepsOperatorWithShutdown = &OpMQTT{}

func (o *OpMQTT) GetDefaultConfig() interface{} {
	return FreepsMqttConfig{Server: "", Username: "", Password: "", Topics: []TopicConfig{DefaultTopicConfig}}
}

func (o *OpMQTT) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*FreepsMqttConfig)
	f, err := newFreepsMqttImpl(ctx.GetLogger(), cfg, o.GE)
	op := &OpMQTT{CR: o.CR, GE: o.GE, impl: f}
	return op, err
}

// GetSubscriptions returns a list of all subscriped topics
func (o *OpMQTT) GetSubscriptions(ctx *base.Context) *base.OperatorIO {
	topics := o.impl.getTopicSubscriptions()
	return base.MakeObjectOutput(topics)
}

func (o *OpMQTT) ExecuteDynamic(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	args := fa.GetOriginalCaseMap()
	switch fn {
	case "publish":
		topic, ok := args["topic"]
		if !ok {
			return base.MakeOutputError(http.StatusBadRequest, "publish: topic not specified")
		}
		msg, ok := args["msg"]
		if !ok {
			msg = input.GetString()
		}
		qos, err := strconv.Atoi(args["qos"])
		if err != nil {
			qos = 0
		}
		retain, err := strconv.ParseBool(args["retain"])
		if err != nil {
			retain = false
		}
		_, ok = args["server"]
		if ok {
			return o.publishToExternal(args, topic, msg, qos, retain)
		}
		err = o.impl.publish(topic, msg, qos, retain)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		return base.MakeEmptyOutput()
	}
	return base.MakeOutputError(http.StatusBadRequest, "Unknown function "+fn)
}

func (o *OpMQTT) GetDynamicFunctions() []string {
	return []string{"publish"}
}

func (o *OpMQTT) GetDynamicPossibleArgs(fn string) []string {
	return []string{"topic", "msg", "qos", "retain", "server", "username", "password"}
}

func (o *OpMQTT) GetDynamicArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "retain":
		return map[string]string{"true": "true", "false": "false"}
	case "qos":
		return map[string]string{"0": "0", "1": "1", "2": "2"}
	case "server":
		return map[string]string{"localhost:1883": "localhost:1883"}
	}

	return map[string]string{}
}

// publish on a new connection to a defined server
func (o *OpMQTT) publishToExternal(args map[string]string, topic string, msg interface{}, qos int, retain bool) *base.OperatorIO {
	server := args["server"]
	username := args["username"]
	password := args["password"]

	hostname, _ := os.Hostname()
	clientid := hostname + "publish" + strconv.Itoa(time.Now().Second())

	connOpts := MQTT.NewClientOptions()
	connOpts.AddBroker(server).SetClientID(clientid).SetCleanSession(true)
	if username != "" {
		connOpts.SetUsername(username)
		if password != "" {
			connOpts.SetPassword(password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)
	connOpts.SetCleanSession(true)

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return base.MakeOutputError(http.StatusInternalServerError, token.Error().Error())
	}

	if token := client.Publish(topic, byte(qos), retain, msg); token.Wait() && token.Error() != nil {
		return base.MakeOutputError(http.StatusInternalServerError, token.Error().Error())
	}
	client.Disconnect(250)
	return base.MakeEmptyOutput()
}

func (o *OpMQTT) GetHook() interface{} {
	return &HookMQTT{o.impl}
}

func (o *OpMQTT) StartListening(ctx *base.Context) {
	o.impl.startTagSubscriptions()
}

func (o *OpMQTT) Shutdown(ctx *base.Context) {
	o.impl.Shutdown()
}
