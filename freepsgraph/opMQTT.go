package freepsgraph

import (
	"crypto/tls"
	"net/http"
	"os"
	"strconv"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/hannesrauhe/freeps/utils"
)

type OpMQTT struct {
}

var _ FreepsOperator = &OpMQTT{}

func NewMQTTOp(cr *utils.ConfigReader) *OpMQTT {
	fmqtt := &OpMQTT{}
	return fmqtt
}

func (o *OpMQTT) Execute(fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "publish":
		server, ok := args["server"]
		if !ok {
			return MakeOutputError(http.StatusBadRequest, "missing server")
		}
		username := args["username"]
		password := args["password"]
		topic, ok := args["topic"]
		if !ok {
			return MakeOutputError(http.StatusBadRequest, "publish: topic not specified")
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
			return MakeOutputError(http.StatusInternalServerError, token.Error().Error())
		}

		if token := client.Publish(topic, byte(qos), retain, msg); token.Wait() && token.Error() != nil {
			return MakeOutputError(http.StatusInternalServerError, token.Error().Error())
		}
		client.Disconnect(250)
		return MakeEmptyOutput()
	}
	return MakeOutputError(http.StatusBadRequest, "Unknown function "+fn)
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
