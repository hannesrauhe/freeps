package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/freepslib"
	"github.com/hannesrauhe/freeps/freepsmqtt"
	"github.com/hannesrauhe/freeps/utils"
)

var verbose bool

func mqttReceivedMessage(ff *freepsflux.FreepsFlux, tc freepsmqtt.TopicToFluxConfig, client MQTT.Client, message MQTT.Message) {
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
		ff.PushFields(t[tc.MeasurementIndex], map[string]interface{}{fieldAlias: value})

	} else {
		fmt.Printf("#Measuremnt: %s, Field: %s, Value: %s\n", t[tc.MeasurementIndex], field, message.Payload())
	}
}

func mqtt(cr *utils.ConfigReader) {
	hostname, _ := os.Hostname()
	clientid := hostname + strconv.Itoa(time.Now().Second())

	ffc := freepsflux.DefaultConfig
	fmc := freepsmqtt.DefaultConfig
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
	ff.Verbose = verbose
	if err2 != nil {
		log.Fatalf("Error while executing function: %v\n", err2)
	}

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
				mqttReceivedMessage(ff, k, client, message)
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
		fmt.Printf("Connected to %s\n", fmc.Server)
	}
}

func main() {
	var configpath, fn, dev string
	flag.StringVar(&configpath, "c", utils.GetDefaultPath("freeps"), "Specify config file to use")
	flag.StringVar(&fn, "f", "freepsflux", "Specify function")
	flag.StringVar(&dev, "d", "", "Specify device")
	flag.BoolVar(&verbose, "v", false, "Verbose output")

	flag.Parse()

	cr, err := utils.NewConfigReader(configpath)
	if err != nil {
		log.Fatal(err)
	}

	if fn == "mqtt" {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		mqtt(cr)
		<-c
	} else {
		conf := freepslib.DefaultConfig
		err = cr.ReadSectionWithDefaults("freepslib", &conf)
		if err != nil {
			log.Fatal(err)
		}
		cr.WriteBackConfigIfChanged()
		if err != nil {
			log.Print(err)
		}

		f, err := freepslib.NewFreepsLib(&conf)
		f.Verbose = verbose

		var jsonbytes []byte

		switch fn {
		case "freepsflux":
			{
				ffc := freepsflux.DefaultConfig
				err = cr.ReadSectionWithDefaults("freepsflux", &ffc)
				if err != nil {
					log.Fatal(err)
				}
				cr.WriteBackConfigIfChanged()
				if err != nil {
					log.Print(err)
				}

				ff, err2 := freepsflux.NewFreepsFlux(&ffc, f)
				if err2 != nil {
					log.Fatalf("Error while executing function: %v\n", err2)
				}
				ff.Verbose = f.Verbose
				err = ff.Push()
			}
		case "getdevicelistinfos":
			{
				devl, err2 := f.GetDeviceList()
				if err2 != nil {
					log.Fatalf("Error while executing function: %v\n", err2)
				}
				jsonbytes, err = json.MarshalIndent(devl, "", "  ")
			}
		case "gettemplatelistinfos":
			{
				devl, err2 := f.GetTemplateList()
				if err2 != nil {
					log.Fatalf("Error while executing function: %v\n", err2)
				}
				jsonbytes, err = json.MarshalIndent(devl, "", "  ")
			}
		case "getdata":
			{
				devl, err2 := f.GetData()
				if err2 != nil {
					log.Fatalf("Error while executing function: %v\n", err2)
				}
				jsonbytes, err = json.MarshalIndent(devl, "", "  ")
			}
		case "metrics":
			{
				metrics, err := f.GetMetrics()
				if err != nil {
					log.Fatalf("could not load UPnP service: %v", err)
				}
				fmt.Printf("%v\n", metrics)
			}
		default:
			{
				arg := make(map[string]string)
				result, err2 := f.HomeAutomation(fn, dev, arg)
				if err2 != nil {
					log.Fatalf("Error while executing function: %v\n", err2)
				}
				jsonbytes, err = json.MarshalIndent(result, "", "  ")
			}
		}
		if err != nil {
			log.Fatalf("Error while parsing response: %v\n", err)
		}
		var b bytes.Buffer
		b.Write(jsonbytes)
		fmt.Println(b.String())
	}

}
