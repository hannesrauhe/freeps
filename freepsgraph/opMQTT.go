package freepsgraph

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/hannesrauhe/freeps/utils"
)

type OpMQTT struct {
	client MQTT.Client
}

// TODO: unify with freepslistener
type FreepsMqttConfig struct {
	Server   string // The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883
	Username string // A username to authenticate to the MQTT server
	Password string // Password to match username
}

var DefaultConfig = FreepsMqttConfig{Server: "", Username: "", Password: ""}

var _ FreepsOperator = &OpMQTT{}

func NewMQTTOp(cr *utils.ConfigReader) *OpMQTT {
	fmc := DefaultConfig
	err := cr.ReadSectionWithDefaults("freepsmqtt", &fmc)
	if err != nil {
		log.Fatal(err)
	}

	if fmc.Server == "" {
		return nil
	}

	hostname, _ := os.Hostname()
	clientid := hostname + strconv.Itoa(time.Now().Second())
	fmqtt := &OpMQTT{}

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

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmqtt.client = client
	return fmqtt
}

func (o *OpMQTT) Execute(fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "publish":
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

		if token := o.client.Publish(topic, byte(qos), retain, msg); token.Wait() && token.Error() != nil {
			return MakeOutputError(http.StatusInternalServerError, token.Error().Error())
		}
		return MakeEmptyOutput()
	}
	return MakeOutputError(http.StatusBadRequest, "Unknown function "+fn)
}

func (o *OpMQTT) GetFunctions() []string {
	return []string{"publish"}
}

func (o *OpMQTT) GetPossibleArgs(fn string) []string {
	return []string{"topic", "msg", "qos", "retain"}
}

func (o *OpMQTT) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "retain":
		return map[string]string{"true": "true", "false": "false"}
	case "qos":
		return map[string]string{"0": "0", "1": "1", "2": "2"}
	}

	return map[string]string{}
}
