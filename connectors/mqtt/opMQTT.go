package mqtt

import (
	"crypto/tls"
	"net/http"
	"os"
	"strconv"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type OpMQTT struct {
}

var _ freepsgraph.FreepsOperator = &OpMQTT{}

func NewMQTTOp(cr *utils.ConfigReader) *OpMQTT {
	fmqtt := &OpMQTT{}
	return fmqtt
}

// GetName returns the name of the operator
func (o *OpMQTT) GetName() string {
	return "mqtt"
}

func (o *OpMQTT) Execute(ctx *utils.Context, fn string, args map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	switch fn {
	case "publish":
		topic, ok := args["topic"]
		if !ok {
			return freepsgraph.MakeOutputError(http.StatusBadRequest, "publish: topic not specified")
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
		fm := GetInstance()
		err = fm.Publish(topic, msg, qos, retain)
		if err != nil {
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		return freepsgraph.MakeEmptyOutput()
	}
	return freepsgraph.MakeOutputError(http.StatusBadRequest, "Unknown function "+fn)
}

func (o *OpMQTT) GetFunctions() []string {
	return []string{"publish"}
}

func (o *OpMQTT) GetPossibleArgs(fn string) []string {
	return []string{"topic", "msg", "qos", "retain", "server", "username", "password"}
}

func (o *OpMQTT) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
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

// Shutdown (noOp)
func (o *OpMQTT) Shutdown(ctx *utils.Context) {
}

// publish on a new connection to a defined server
func (o *OpMQTT) publishToExternal(args map[string]string, topic string, msg interface{}, qos int, retain bool) *freepsgraph.OperatorIO {
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
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, token.Error().Error())
	}

	if token := client.Publish(topic, byte(qos), retain, msg); token.Wait() && token.Error() != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, token.Error().Error())
	}
	client.Disconnect(250)
	return freepsgraph.MakeEmptyOutput()
}
